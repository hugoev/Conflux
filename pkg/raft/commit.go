package raft

import (
	"sort"

	"go.uber.org/zap"
)

// advanceCommitIndex updates commitIndex based on matchIndex
// Must be called with lock held
func (n *Node) advanceCommitIndex() {
	// Find the highest index replicated on majority of servers
	// matchIndex contains the highest log index replicated on each server

	if n.state != StateLeader {
		return
	}

	// Collect all matchIndex values
	matches := make([]int, 0, len(n.peers))
	for _, peer := range n.peers {
		matches = append(matches, n.matchIndex[peer])
	}

	// Sort in descending order
	sort.Sort(sort.Reverse(sort.IntSlice(matches)))

	// The median is the highest index replicated on majority
	majorityIndex := len(matches) / 2
	newCommitIndex := matches[majorityIndex]

	// Only commit entries from current term (Raft safety requirement)
	if newCommitIndex > n.commitIndex && n.log[newCommitIndex].Term == n.currentTerm {
		n.logger.Info("Advancing commit index",
			zap.Int("old", n.commitIndex),
			zap.Int("new", newCommitIndex),
			zap.Int("term", n.currentTerm))
		n.commitIndex = newCommitIndex
	}
}
