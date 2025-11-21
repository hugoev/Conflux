package raft

import (
	"testing"

	"go.uber.org/zap"
)

// createTestNode creates a node for testing
func createTestNode(t *testing.T, nodeID string, peers []string) *Node {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return NewNode(nodeID, peers, t.TempDir(), logger)
}

// TestRequestVote_TermComparison tests term comparison logic
func TestRequestVote_TermComparison(t *testing.T) {
	tests := []struct {
		name              string
		currentTerm       int
		requestTerm       int
		expectedGrant     bool
		expectedReplyTerm int // What reply.Term should be
		expectedNodeTerm  int // What node.currentTerm should be after call
	}{
		{
			name:              "reject lower term",
			currentTerm:       5,
			requestTerm:       3,
			expectedGrant:     false,
			expectedReplyTerm: 5,
			expectedNodeTerm:  5,
		},
		{
			name:              "accept equal term (first vote)",
			currentTerm:       5,
			requestTerm:       5,
			expectedGrant:     true,
			expectedReplyTerm: 5,
			expectedNodeTerm:  5,
		},
		{
			name:              "accept higher term",
			currentTerm:       3,
			requestTerm:       5,
			expectedGrant:     true,
			expectedReplyTerm: 3, // Reply.Term is set at start (line 127 in rpc.go), before term update
			expectedNodeTerm:  5, // But node.currentTerm gets updated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := createTestNode(t, "test-node", []string{})
			node.currentTerm = tt.currentTerm
			node.votedFor = "" // Haven't voted yet

			args := &RequestVoteArgs{
				Term:         tt.requestTerm,
				CandidateID:  "candidate",
				LastLogIndex: 0,
				LastLogTerm:  0,
			}
			reply := &RequestVoteReply{}

			err := node.RequestVote(args, reply)
			if err != nil {
				t.Fatalf("RequestVote failed: %v", err)
			}

			if reply.VoteGranted != tt.expectedGrant {
				t.Errorf("Expected VoteGranted=%v, got %v", tt.expectedGrant, reply.VoteGranted)
			}

			if reply.Term != tt.expectedReplyTerm {
				t.Errorf("Expected reply Term=%d, got %d", tt.expectedReplyTerm, reply.Term)
			}

			if node.currentTerm != tt.expectedNodeTerm {
				t.Errorf("Expected node currentTerm=%d, got %d", tt.expectedNodeTerm, node.currentTerm)
			}
		})
	}
}

// TestRequestVote_AlreadyVoted tests voting when already voted in term
func TestRequestVote_AlreadyVoted(t *testing.T) {
	node := createTestNode(t, "test-node", []string{})
	node.currentTerm = 5
	node.votedFor = "other-candidate"

	args := &RequestVoteArgs{
		Term:         5,
		CandidateID:  "new-candidate",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}
	reply := &RequestVoteReply{}

	err := node.RequestVote(args, reply)
	if err != nil {
		t.Fatalf("RequestVote failed: %v", err)
	}

	if reply.VoteGranted {
		t.Error("Should not grant vote when already voted for different candidate")
	}
}

// TestRequestVote_SameCandidate tests voting for same candidate twice
func TestRequestVote_SameCandidate(t *testing.T) {
	node := createTestNode(t, "test-node", []string{})
	node.currentTerm = 5
	node.votedFor = "candidate"

	args := &RequestVoteArgs{
		Term:         5,
		CandidateID:  "candidate",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}
	reply := &RequestVoteReply{}

	err := node.RequestVote(args, reply)
	if err != nil {
		t.Fatalf("RequestVote failed: %v", err)
	}

	if !reply.VoteGranted {
		t.Error("Should grant vote to same candidate")
	}
}

// TestIsLogUpToDate tests log comparison logic
func TestIsLogUpToDate(t *testing.T) {
	tests := []struct {
		name             string
		nodeLastLogIndex int
		nodeLastLogTerm  int
		candLastLogIndex int
		candLastLogTerm  int
		expectedUpToDate bool
	}{
		{
			name:             "candidate has higher term",
			nodeLastLogIndex: 5,
			nodeLastLogTerm:  3,
			candLastLogIndex: 3,
			candLastLogTerm:  4,
			expectedUpToDate: true,
		},
		{
			name:             "candidate has lower term",
			nodeLastLogIndex: 5,
			nodeLastLogTerm:  4,
			candLastLogIndex: 10,
			candLastLogTerm:  3,
			expectedUpToDate: false,
		},
		{
			name:             "same term, candidate has longer log",
			nodeLastLogIndex: 5,
			nodeLastLogTerm:  3,
			candLastLogIndex: 7,
			candLastLogTerm:  3,
			expectedUpToDate: true,
		},
		{
			name:             "same term, candidate has shorter log",
			nodeLastLogIndex: 7,
			nodeLastLogTerm:  3,
			candLastLogIndex: 5,
			candLastLogTerm:  3,
			expectedUpToDate: false,
		},
		{
			name:             "identical logs",
			nodeLastLogIndex: 5,
			nodeLastLogTerm:  3,
			candLastLogIndex: 5,
			candLastLogTerm:  3,
			expectedUpToDate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := createTestNode(t, "test-node", []string{})

			// Set up node's log
			if tt.nodeLastLogIndex > 0 {
				node.log = make([]LogEntry, tt.nodeLastLogIndex)
				node.log[tt.nodeLastLogIndex-1].Term = tt.nodeLastLogTerm
			}

			result := node.isLogUpToDate(tt.candLastLogIndex, tt.candLastLogTerm)
			if result != tt.expectedUpToDate {
				t.Errorf("Expected isLogUpToDate=%v, got %v", tt.expectedUpToDate, result)
			}
		})
	}
}

// TestAppendEntries_TermCheck tests term validation in AppendEntries
func TestAppendEntries_TermCheck(t *testing.T) {
	tests := []struct {
		name              string
		currentTerm       int
		requestTerm       int
		expectedSuccess   bool
		expectedReplyTerm int // What reply.Term should be (set at start of function)
		expectedNodeTerm  int // What node.currentTerm should be after call
	}{
		{
			name:              "reject lower term",
			currentTerm:       5,
			requestTerm:       3,
			expectedSuccess:   false,
			expectedReplyTerm: 5,
			expectedNodeTerm:  5,
		},
		{
			name:              "accept equal term",
			currentTerm:       5,
			requestTerm:       5,
			expectedSuccess:   true,
			expectedReplyTerm: 5,
			expectedNodeTerm:  5,
		},
		{
			name:              "accept higher term",
			currentTerm:       3,
			requestTerm:       5,
			expectedSuccess:   true,
			expectedReplyTerm: 3, // Reply.Term is set before updating
			expectedNodeTerm:  5, // But node.currentTerm gets updated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := createTestNode(t, "test-node", []string{})
			node.currentTerm = tt.currentTerm

			args := &AppendEntriesArgs{
				Term:         tt.requestTerm,
				LeaderID:     "leader",
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      []LogEntry{},
				LeaderCommit: 0,
			}
			reply := &AppendEntriesReply{}

			node.AppendEntries(args, reply)

			if reply.Success != tt.expectedSuccess {
				t.Errorf("Expected Success=%v, got %v", tt.expectedSuccess, reply.Success)
			}

			if reply.Term != tt.expectedReplyTerm {
				t.Errorf("Expected reply Term=%d, got %d", tt.expectedReplyTerm, reply.Term)
			}

			if node.currentTerm != tt.expectedNodeTerm {
				t.Errorf("Expected node currentTerm=%d, got %d", tt.expectedNodeTerm, node.currentTerm)
			}
		})
	}
}
