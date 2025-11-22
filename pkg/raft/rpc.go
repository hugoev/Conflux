package raft

import "go.uber.org/zap"

// AppendEntriesArgs is the RPC argument for AppendEntries
type AppendEntriesArgs struct {
	Term         int
	LeaderID     string
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

// AppendEntriesReply is the RPC reply for AppendEntries
type AppendEntriesReply struct {
	Term    int
	Success bool
}

// RequestVoteArgs is the RPC argument for RequestVote
type RequestVoteArgs struct {
	Term         int
	CandidateID  string
	LastLogIndex int
	LastLogTerm  int
}

// RequestVoteReply is the RPC reply for RequestVote
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// AppendEntries handles AppendEntries RPC
func (n *Node) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	n.mu.Lock()
	defer n.mu.Unlock()

	reply.Term = n.currentTerm
	reply.Success = false

	// 1. Reply false if term < currentTerm
	if args.Term < n.currentTerm {
		return
	}

	// If RPC request contains term > currentTerm, set currentTerm = term, convert to follower
	if args.Term > n.currentTerm {
		n.currentTerm = args.Term
		n.votedFor = ""
		n.state = StateFollower
		// Signal to main loop to step down if leader/candidate
		select {
		case n.electionResetEvent <- struct{}{}:
		default:
		}
	}

	// If we are candidate, we step down because we found a leader
	if n.state == StateCandidate {
		n.state = StateFollower
	}

	// Reset election timer
	select {
	case n.electionResetEvent <- struct{}{}:
	default:
	}

	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm
	// (Log matching property)
	// Note: We need to handle the case where prevLogIndex is 0 (start of log)
	if args.PrevLogIndex > 0 {
		lastIndex := len(n.log) - 1
		if args.PrevLogIndex > lastIndex {
			return
		}
		if n.log[args.PrevLogIndex].Term != args.PrevLogTerm {
			return
		}
	}

	// 3. If an existing entry conflicts with a new one (same index but different terms),
	// delete the existing entry and all that follow it
	// 4. Append any new entries not already in the log
	// We can optimize this by finding the first conflict
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + 1 + i
		if index < len(n.log) {
			if n.log[index].Term != entry.Term {
				n.log = n.log[:index]
				n.log = append(n.log, entry)
				// Persist to WAL
				if err := n.persistLogEntry(entry); err != nil {
					n.logger.Error("Failed to persist log entry", zap.Error(err))
				}
			}
		} else {
			n.log = append(n.log, entry)
			// Persist to WAL
			if err := n.persistLogEntry(entry); err != nil {
				n.logger.Error("Failed to persist log entry", zap.Error(err))
			}
		}
	}

	// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > n.commitIndex {
		lastNewEntryIndex := args.PrevLogIndex + len(args.Entries)
		if args.LeaderCommit < lastNewEntryIndex {
			n.commitIndex = args.LeaderCommit
		} else {
			n.commitIndex = lastNewEntryIndex
		}
		// TODO: Signal state machine to apply entries
	}

	reply.Success = true
}

// RequestVote handles RequestVote RPC
func (n *Node) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	reply.Term = n.currentTerm
	reply.VoteGranted = false

	// 1. Reply false if term < currentTerm
	if args.Term < n.currentTerm {
		return nil
	}

	// If RPC request contains term > currentTerm, set currentTerm = term, convert to follower
	if args.Term > n.currentTerm {
		n.currentTerm = args.Term
		n.votedFor = ""
		n.state = StateFollower
		select {
		case n.electionResetEvent <- struct{}{}:
		default:
		}
	}

	// 2. If votedFor is null or candidateId, and candidate's log is at least as up-to-date as receiver's log, grant vote
	if (n.votedFor == "" || n.votedFor == args.CandidateID) && n.isLogUpToDate(args.LastLogIndex, args.LastLogTerm) {
		n.votedFor = args.CandidateID
		reply.VoteGranted = true
		n.logger.Info("Granted vote", zap.String("candidate", args.CandidateID), zap.Int("term", args.Term))
		// Granting vote resets election timer
		select {
		case n.electionResetEvent <- struct{}{}:
		default:
		}
	} else {
		n.logger.Info("Denied vote", zap.String("candidate", args.CandidateID), zap.Int("term", args.Term), zap.String("votedFor", n.votedFor))
	}
	return nil
}

// isLogUpToDate checks if candidate's log is at least as up-to-date as receiver's log
func (n *Node) isLogUpToDate(candidateLastLogIndex, candidateLastLogTerm int) bool {
	lastLogIndex := n.getLastLogIndexLocked()
	lastLogTerm := n.getLastLogTermLocked()

	if candidateLastLogTerm > lastLogTerm {
		return true
	}
	if candidateLastLogTerm == lastLogTerm && candidateLastLogIndex >= lastLogIndex {
		return true
	}
	return false
}
