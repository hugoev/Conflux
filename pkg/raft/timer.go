package raft

import (
	"math/rand"
	"time"
)

// resetElectionTimeout resets the election timeout to a random value
// Each node must have different random timeouts to avoid split votes
func (n *Node) resetElectionTimeout() time.Duration {
	// Raft paper recommends 150-300ms range
	// Using 150-300ms for elections
	min := 150 * time.Millisecond
	max := 300 * time.Millisecond

	// Use time-based seed for better randomization across nodes
	// Each node will have different startup time, giving different seeds
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	return min + time.Duration(rng.Int63n(int64(max-min)))
}

