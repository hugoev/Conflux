package raft

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hugovillarreal/conflux/pkg/snapshot"
	"github.com/hugovillarreal/conflux/pkg/wal"
	"go.uber.org/zap"
)

// NodeState represents the state of a Raft node
type NodeState string

const (
	StateFollower  NodeState = "follower"
	StateCandidate NodeState = "candidate"
	StateLeader    NodeState = "leader"
)

// Node represents a Raft node
type Node struct {
	mu sync.RWMutex

	// Persistent state (should be on stable storage)
	currentTerm int
	votedFor    string
	log         []LogEntry

	// Volatile state
	commitIndex int
	lastApplied int

	// Leader state (reinitialized after election)
	nextIndex  map[string]int
	matchIndex map[string]int

	// Node configuration
	nodeID            string
	peers             []string
	state             NodeState
	electionTimeout   time.Duration
	heartbeatInterval time.Duration

	// Channels
	applyCh            chan ApplyMsg
	electionResetEvent chan struct{}
	stopCh             chan struct{}

	// Channel management
	applyChMu     sync.RWMutex // Protects applyCh access
	applyChClosed bool         // Tracks if channel is closed

	// Stopped state - atomic flag for fast checks without lock
	stopped int32 // 0 = running, 1 = stopped (use atomic operations)

	// Persistence
	wal         *wal.WAL
	snapshotter *snapshot.Snapshotter
	dataDir     string

	// Configuration
	logger *zap.Logger
	server *http.Server
}

// LogEntry represents a single log entry
type LogEntry struct {
	Term    int
	Index   int
	Command interface{}
}

// ApplyMsg is sent to the state machine
type ApplyMsg struct {
	Command      interface{}
	CommandIndex int
	CommandValid bool
}

// NewNode creates a new Raft node
func NewNode(nodeID string, peers []string, dataDir string, logger *zap.Logger) *Node {
	return &Node{
		nodeID:             nodeID,
		peers:              peers,
		state:              StateFollower,
		currentTerm:        0,
		votedFor:           "",
		log:                []LogEntry{{Term: 0, Index: 0}}, // Index 0 is dummy
		commitIndex:        0,
		lastApplied:        0,
		nextIndex:          make(map[string]int),
		matchIndex:         make(map[string]int),
		electionTimeout:    2000 * time.Millisecond,
		heartbeatInterval:  50 * time.Millisecond,
		applyCh:            make(chan ApplyMsg, 100),
		electionResetEvent: make(chan struct{}, 1),
		stopCh:             make(chan struct{}),
		applyChClosed:      false,
		stopped:            0, // 0 = running
		dataDir:            dataDir,
		logger:             logger,
	}
}

// Start starts the Raft node
func (n *Node) Start() {
	n.logger.Info("Starting Raft node", zap.String("nodeID", n.nodeID))

	// Reset stopped flag (in case node was restarted)
	atomic.StoreInt32(&n.stopped, 0)

	// Start apply loop
	n.startApplyLoop()

	go n.run()
}

// run is the main loop for the Raft node
func (n *Node) run() {
	for {
		select {
		case <-n.stopCh:
			n.logger.Info("Raft node stopping")
			return
		default:
		}

		n.mu.RLock()
		state := n.state
		n.mu.RUnlock()

		switch state {
		case StateFollower:
			n.runFollower()
		case StateCandidate:
			n.runCandidate()
		case StateLeader:
			n.runLeader()
		}
	}
}

// runFollower runs the follower loop
func (n *Node) runFollower() {
	timeout := n.resetElectionTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-n.electionResetEvent:
			timer.Stop()
			timer.Reset(n.resetElectionTimeout())
		case <-timer.C:
			n.logger.Info("Election timeout, becoming candidate")
			n.mu.Lock()
			n.state = StateCandidate
			n.mu.Unlock()
			return
		}
	}
}

// runCandidate runs the candidate loop
func (n *Node) runCandidate() {
	n.mu.Lock()
	n.currentTerm++
	n.state = StateCandidate
	n.votedFor = n.nodeID
	term := n.currentTerm
	n.mu.Unlock()

	// Calculate votes needed based on total cluster size (peers + self)
	// For a cluster of N nodes, we need (N/2 + 1) votes
	// This is the Raft requirement: majority of TOTAL configured cluster, not just active nodes
	totalNodes := len(n.peers) + 1 // peers doesn't include self
	votesNeeded := totalNodes/2 + 1

	n.logger.Info("Starting election",
		zap.Int("term", term),
		zap.Int("peers", len(n.peers)),
		zap.Int("total_nodes", totalNodes),
		zap.Int("votes_needed", votesNeeded))

	// Vote for self
	votesReceived := 1

	// Channel to collect vote results
	// Buffer size = number of peers (we'll get one response per peer)
	voteCh := make(chan bool, len(n.peers))
	responsesExpected := len(n.peers) // Number of peers we'll send votes to
	responsesReceived := 0

	// Send RequestVote to all peers
	for _, peer := range n.peers {
		if peer == n.nodeID {
			continue
		}
		go func(peer string) {
			args := &RequestVoteArgs{
				Term:         term,
				CandidateID:  n.nodeID,
				LastLogIndex: n.getLastLogIndex(),
				LastLogTerm:  n.getLastLogTerm(),
			}
			var reply RequestVoteReply

			// Retry logic for transient failures (e.g., DNS not ready during startup)
			maxRetries := 3
			retryDelay := 100 * time.Millisecond

			var err error
			for attempt := 0; attempt < maxRetries; attempt++ {
				err = n.sendRequestVote(peer, args, &reply)
				if err == nil {
					break // Success
				}

				// Only retry on DNS/network errors, not on application errors
				if attempt < maxRetries-1 {
					time.Sleep(retryDelay)
					retryDelay *= 2 // Exponential backoff
				}
			}

			if err != nil {
				// RPC failed after retries (peer is unreachable/stopped)
				n.logger.Debug("RequestVote RPC failed",
					zap.String("peer", peer),
					zap.Error(err),
					zap.Int("term", term))
				voteCh <- false
				return
			}

			// Process reply
			n.mu.Lock()

			if n.state != StateCandidate || n.currentTerm != term {
				n.mu.Unlock()
				voteCh <- false
				return
			}
			if reply.Term > term {
				n.currentTerm = reply.Term
				n.state = StateFollower
				n.votedFor = ""
				n.mu.Unlock()
				voteCh <- false
				return
			}
			n.mu.Unlock()
			// Send vote result without holding lock to avoid deadlock
			voteCh <- reply.VoteGranted
		}(peer)
	}

	timeout := n.resetElectionTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-n.electionResetEvent:
			// Another leader was elected or we received a valid AppendEntries
			n.mu.Lock()
			n.state = StateFollower
			n.mu.Unlock()
			return
		case vote := <-voteCh:
			responsesReceived++
			n.mu.Lock()
			if n.state != StateCandidate || n.currentTerm != term {
				n.mu.Unlock()
				return
			}
			if vote {
				votesReceived++
				n.logger.Info("Received vote",
					zap.Int("term", term),
					zap.Int("votes", votesReceived),
					zap.Int("needed", votesNeeded),
					zap.Int("total_nodes", totalNodes),
					zap.Int("peers_count", len(n.peers)),
					zap.Int("responses", responsesReceived),
					zap.Int("expected", responsesExpected))

				// Check if we have enough votes to win
				if votesReceived >= votesNeeded {
					n.logger.Info("Won election", zap.Int("term", term), zap.Int("votes", votesReceived))
					n.state = StateLeader
					// Reinitialize leader state
					n.nextIndex = make(map[string]int)
					n.matchIndex = make(map[string]int)
					lastLogIdx := n.getLastLogIndexLocked()
					// Initialize matchIndex for all peers (including self)
					n.matchIndex[n.nodeID] = lastLogIdx // Leader always has all its entries
					// Initialize nextIndex for all peers to lastLogIndex + 1
					// This ensures we send all entries to followers on first heartbeat
					for _, p := range n.peers {
						if p != n.nodeID {
							n.nextIndex[p] = lastLogIdx + 1
							n.matchIndex[p] = 0 // Will be updated as replication progresses
						}
					}
					n.mu.Unlock()
					// Trigger immediate heartbeat
					n.sendHeartbeats()
					return
				}
			}
			n.mu.Unlock()

			// If we've received all responses and don't have enough votes, we lost
			// Continue waiting for timeout to retry election
			if responsesReceived >= responsesExpected && votesReceived < votesNeeded {
				n.logger.Info("Election lost: not enough votes",
					zap.Int("votes", votesReceived),
					zap.Int("needed", votesNeeded),
					zap.Int("responses", responsesReceived))
				// Don't return - wait for timeout to retry
			}
		case <-timer.C:
			// Election timeout, retry
			return
		}
	}
}

// runLeader runs the leader loop
func (n *Node) runLeader() {
	n.logger.Info("Became leader", zap.Int("term", n.currentTerm))

	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-n.electionResetEvent:
			// Stepped down (e.g. saw higher term)
			n.mu.Lock()
			n.state = StateFollower
			n.mu.Unlock()
			return
		case <-ticker.C:
			n.sendHeartbeats()
		}
	}
}

// Propose proposes a command to the Raft cluster
func (n *Node) Propose(cmd interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != StateLeader {
		return fmt.Errorf("not leader")
	}

	entry := LogEntry{
		Term:    n.currentTerm,
		Index:   len(n.log),
		Command: cmd,
	}
	n.log = append(n.log, entry)
	n.matchIndex[n.nodeID] = entry.Index
	n.nextIndex[n.nodeID] = entry.Index + 1

	// Persist to WAL
	if err := n.persistLogEntry(entry); err != nil {
		n.logger.Error("Failed to persist log entry", zap.Error(err))
		return fmt.Errorf("failed to persist log entry: %w", err)
	}

	n.logger.Info("Proposed command", zap.Int("index", entry.Index), zap.Int("term", entry.Term))

	// Trigger immediate replication (don't wait for next heartbeat)
	// This improves latency and test reliability
	// Note: sendHeartbeats acquires its own locks, so we release our lock first
	n.mu.Unlock()

	// Send heartbeats to replicate the new entry immediately
	n.sendHeartbeats()

	// Re-acquire lock (though we're about to return, this maintains lock discipline)
	n.mu.Lock()

	return nil
}

func (n *Node) sendHeartbeats() {
	n.mu.RLock()
	term := n.currentTerm
	leaderID := n.nodeID
	n.mu.RUnlock()

	for _, peer := range n.peers {
		if peer == n.nodeID {
			continue
		}
		go func(peer string) {
			// Get entries to send based on nextIndex
			n.mu.RLock()
			nextIdx := n.nextIndex[peer]
			prevLogIndex := nextIdx - 1
			prevLogTerm := 0
			if prevLogIndex > 0 && prevLogIndex < len(n.log) {
				prevLogTerm = n.log[prevLogIndex].Term
			}

			// Get entries from nextIndex onwards
			var entries []LogEntry
			if nextIdx < len(n.log) {
				entries = make([]LogEntry, len(n.log)-nextIdx)
				copy(entries, n.log[nextIdx:])
			}

			leaderCommit := n.commitIndex
			n.mu.RUnlock()

			args := &AppendEntriesArgs{
				Term:         term,
				LeaderID:     leaderID,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: leaderCommit,
			}

			var reply AppendEntriesReply
			if err := n.sendAppendEntries(peer, args, &reply); err != nil {
				// RPC failed, will retry next heartbeat
				return
			}

			// Process reply
			n.mu.Lock()
			defer n.mu.Unlock()

			if n.state != StateLeader || n.currentTerm != term {
				return
			}

			if reply.Term > term {
				// Discovered higher term, step down
				n.currentTerm = reply.Term
				n.state = StateFollower
				n.votedFor = ""
				return
			}

			if reply.Success {
				// Update nextIndex and matchIndex
				newMatchIndex := prevLogIndex + len(entries)
				if newMatchIndex > n.matchIndex[peer] {
					n.matchIndex[peer] = newMatchIndex
					n.nextIndex[peer] = newMatchIndex + 1
				}

				// Try to advance commitIndex after successful replication
				// This is called after each successful AppendEntries response
				n.advanceCommitIndex()
			} else {
				// Log inconsistency, decrement nextIndex and retry
				// This happens when follower's log doesn't match leader's expectation
				if n.nextIndex[peer] > 1 {
					n.nextIndex[peer]--
				}
				// Don't advance commit index on failure
			}
		}(peer)
	}
}

// ApplyCh returns the apply channel
// This method is thread-safe and returns a read-only channel
// Returns nil if the channel has been closed
func (n *Node) ApplyCh() <-chan ApplyMsg {
	n.applyChMu.RLock()
	defer n.applyChMu.RUnlock()
	if n.applyChClosed || n.applyCh == nil {
		return nil
	}
	return n.applyCh
}

// ResetApplyCh recreates the apply channel (used after Stop)
func (n *Node) ResetApplyCh() {
	n.applyChMu.Lock()
	defer n.applyChMu.Unlock()
	// Create a new buffered channel
	n.applyCh = make(chan ApplyMsg, 100)
	n.applyChClosed = false
}

// GetState returns the current state
func (n *Node) GetState() (NodeState, int) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state, n.currentTerm
}

// IsLeader returns true if this node is the leader
func (n *Node) IsLeader() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state == StateLeader
}

// GetCommitIndex returns the current commit index
func (n *Node) GetCommitIndex() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.commitIndex
}

// IsStopped returns true if the node has been stopped
// Uses atomic flag for fast checks without acquiring lock
func (n *Node) IsStopped() bool {
	return atomic.LoadInt32(&n.stopped) == 1
}

// IsApplyChClosed returns true if the apply channel has been closed
func (n *Node) IsApplyChClosed() bool {
	n.applyChMu.RLock()
	defer n.applyChMu.RUnlock()
	return n.applyChClosed || n.applyCh == nil
}
