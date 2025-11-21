package raft

import (
	"testing"
	"time"
)

// TestResetElectionTimeout_Range verifies timeout is within expected range
func TestResetElectionTimeout_Range(t *testing.T) {
	node := &Node{}

	minExpected := 150 * time.Millisecond
	maxExpected := 300 * time.Millisecond

	// Test multiple times to ensure randomization works
	for i := 0; i < 100; i++ {
		timeout := node.resetElectionTimeout()

		if timeout < minExpected {
			t.Errorf("Timeout %v is less than minimum %v", timeout, minExpected)
		}

		if timeout > maxExpected {
			t.Errorf("Timeout %v is greater than maximum %v", timeout, maxExpected)
		}
	}
}

// TestResetElectionTimeout_Randomization verifies timeouts are actually random
func TestResetElectionTimeout_Randomization(t *testing.T) {
	node := &Node{}

	// Collect 100 samples
	samples := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		timeout := node.resetElectionTimeout()
		samples[timeout] = true
	}

	// Should have at least 50 unique values (randomization working)
	if len(samples) < 50 {
		t.Errorf("Expected at least 50 unique timeout values, got %d (poor randomization)", len(samples))
	}
}

// BenchmarkResetElectionTimeout benchmarks timeout generation
func BenchmarkResetElectionTimeout(b *testing.B) {
	node := &Node{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = node.resetElectionTimeout()
	}
}
