package raft

import (
	"math/rand"
	"time"
)

// resetElectionTimeout resets the election timeout to a random value
func (n *Node) resetElectionTimeout() time.Duration {
	// Random timeout between 150ms and 300ms
	min := 2000 * time.Millisecond
	max := 4000 * time.Millisecond
	return min + time.Duration(rand.Int63n(int64(max-min)))
}
