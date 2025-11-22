package raft

// LogEntry represents a single log entry (defined in node.go)
// This file will contain log management functions

// getLastLogIndex returns the index of the last log entry
func (n *Node) getLastLogIndex() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.getLastLogIndexLocked()
}

func (n *Node) getLastLogIndexLocked() int {
	return len(n.log) - 1
}

// getLastLogTerm returns the term of the last log entry
func (n *Node) getLastLogTerm() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.getLastLogTermLocked()
}

func (n *Node) getLastLogTermLocked() int {
	if len(n.log) == 0 {
		return 0
	}
	return n.log[len(n.log)-1].Term
}
