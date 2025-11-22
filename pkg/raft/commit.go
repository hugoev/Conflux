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

	// Collect all matchIndex values (including leader itself)
	matches := make([]int, 0, len(n.peers)+1)
	// Include leader's own matchIndex (leader always has all its entries)
	matches = append(matches, n.getLastLogIndexLocked())
	// Include all peer matchIndex values
	for _, peer := range n.peers {
		if peer != n.nodeID {
			matches = append(matches, n.matchIndex[peer])
		}
	}

	// Sort in descending order
	sort.Sort(sort.Reverse(sort.IntSlice(matches)))

	// Safety check
	if len(matches) == 0 {
		return // No nodes to check (shouldn't happen, but be safe)
	}

	// Find the highest index where at least majority of nodes have replicated it
	// Majority = (totalNodes + 1) / 2 where totalNodes = len(peers) + 1 (including leader)
	// Note: We use total configured cluster size, not just active nodes
	totalNodes := len(n.peers) + 1 // Include leader
	majorityCount := totalNodes/2 + 1
	newCommitIndex := n.commitIndex

	// Check each index from commitIndex+1 to the highest matchIndex
	if len(matches) == 0 {
		return // No matches, can't commit anything
	}
	maxIndex := matches[0] // Highest matchIndex
	for idx := n.commitIndex + 1; idx <= maxIndex; idx++ {
		// Count how many nodes have replicated this index or higher
		count := 0
		for _, matchIdx := range matches {
			if matchIdx >= idx {
				count++
			}
		}

		// If majority have replicated this index, and it's from current term, we can commit it
		if count >= majorityCount {
			// Safety check: ensure we have a log entry at this index
			if idx < len(n.log) {
				// Only commit entries from current term (Raft safety requirement)
				if n.log[idx].Term == n.currentTerm {
					newCommitIndex = idx
				} else {
					// Can't commit entries from previous terms (Raft safety requirement)
					// Stop here as we can't commit higher indices
					break
				}
			} else {
				// Index beyond log length, can't commit
				break
			}
		} else {
			// Can't commit higher indices if we don't have majority
			break
		}
	}

	// Update commitIndex if we found a new commit index
	if newCommitIndex > n.commitIndex {
		n.logger.Info("Advancing commit index",
			zap.Int("old", n.commitIndex),
			zap.Int("new", newCommitIndex),
			zap.Int("term", n.currentTerm),
			zap.Int("majority", majorityCount),
			zap.Int("total_nodes", len(matches)))
		n.commitIndex = newCommitIndex
	}
}
