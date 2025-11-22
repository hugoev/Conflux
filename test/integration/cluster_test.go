package integration

import (
	"testing"
	"time"
)

func TestClusterFormation(t *testing.T) {
	tests := []struct {
		name      string
		nodeCount int
		timeout   time.Duration
	}{
		{
			name:      "3-node cluster",
			nodeCount: 3,
			timeout:   15 * time.Second,
		},
		{
			name:      "5-node cluster",
			nodeCount: 5,
			timeout:   10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create cluster
			cluster := NewTestCluster(t, tt.nodeCount)

			// Start all nodes
			if err := cluster.Start(); err != nil {
				t.Fatalf("Failed to start cluster: %v", err)
			}

			// Wait for leader election
			leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
			if err != nil {
				t.Fatalf("Leader election failed: %v", err)
			}

			t.Logf("Leader elected: node-%d", leaderIdx)

			// Verify exactly one leader
			AssertLeaderElected(t, cluster)

			// Verify all other nodes are followers
			for i, node := range cluster.Nodes {
				if i == leaderIdx {
					if !node.IsLeader() {
						t.Errorf("Node %d should be leader", i)
					}
				} else {
					if node.IsLeader() {
						t.Errorf("Node %d should not be leader", i)
					}
				}
			}
		})
	}
}

func TestLeaderElection(t *testing.T) {
	// Create 3-node cluster
	cluster := NewTestCluster(t, 3)

	// Start all nodes
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Wait for initial leader
	initialLeader, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Initial leader election failed: %v", err)
	}

	t.Logf("Initial leader: node-%d", initialLeader)

	// Stop the leader
	cluster.StopNode(initialLeader)
	t.Logf("Stopped leader node-%d", initialLeader)

	// Wait for new leader election (need longer timeout for election)
	time.Sleep(2 * time.Second) // Give time for election timeout

	newLeader, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("New leader election failed: %v", err)
	}

	t.Logf("New leader elected: node-%d", newLeader)

	// Verify new leader is different
	if newLeader == initialLeader {
		t.Error("New leader should be different from stopped leader")
	}

	// Verify exactly one leader
	AssertLeaderElected(t, cluster)
}

func TestLeaderFailover(t *testing.T) {
	// Create 3-node cluster
	cluster := NewTestCluster(t, 3)

	// Start all nodes
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Wait for leader
	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	// Perform multiple failovers
	for i := 0; i < 3; i++ {
		t.Logf("Failover iteration %d, current leader: node-%d", i+1, leaderIdx)

		// Stop current leader
		cluster.StopNode(leaderIdx)

		// Wait for leader to stop and new leader to be elected
		// HTTP server shutdown is fast (1s timeout), election timeout is 150-300ms
		// So 2 seconds should be enough for shutdown + election
		time.Sleep(2 * time.Second)
		newLeaderIdx, err := cluster.WaitForLeader(30 * time.Second)
		if err != nil {
			t.Fatalf("Failover %d failed: no leader elected within timeout: %v", i+1, err)
		}

		// Verify new leader is different
		if newLeaderIdx == leaderIdx {
			t.Errorf("Failover %d: new leader should be different", i+1)
		}

		// Restart old leader (becomes follower)
		if err := cluster.RestartNode(leaderIdx); err != nil {
			t.Fatalf("Failed to restart node-%d: %v", leaderIdx, err)
		}

		// Give time for node to rejoin and stabilize
		// Restarted node needs to:
		// 1. Recover state from WAL/snapshots (fast, in-memory)
		// 2. Start HTTP server and reconnect (fast, ~200ms)
		// 3. Receive heartbeats from current leader (50ms interval)
		// 4. Sync state and catch up on log entries
		// Leader needs to discover the restarted peer and adjust nextIndex
		time.Sleep(2 * time.Second)

		// Verify exactly one leader exists
		AssertLeaderElected(t, cluster)

		// Update leaderIdx for next iteration
		// Note: After restarting, the old leader might become leader again if it has more up-to-date log
		// So we need to get the current leader, not assume it's newLeaderIdx
		// Wait a bit to ensure the cluster has stabilized after restart
		// The restarted node needs time to catch up and the cluster needs to stabilize
		time.Sleep(1 * time.Second)
		currentLeader, err := cluster.WaitForLeader(15 * time.Second)
		if err != nil {
			t.Fatalf("Failed to get leader after restart in iteration %d: %v", i+1, err)
		}
		leaderIdx = currentLeader
	}
}

func TestSplitVotePrevention(t *testing.T) {
	// Run multiple times to test randomization
	for i := 0; i < 5; i++ {
		t.Run("iteration", func(t *testing.T) {
			// Create 3-node cluster
			cluster := NewTestCluster(t, 3)

			// Start all nodes simultaneously
			if err := cluster.Start(); err != nil {
				t.Fatalf("Failed to start cluster: %v", err)
			}

			// Wait for leader election
			_, err := cluster.WaitForLeader(10 * time.Second)
			if err != nil {
				t.Fatalf("Leader election failed (possible split vote): %v", err)
			}

			// Verify exactly one leader (no split brain)
			leaderCount := cluster.CountLeaders()
			if leaderCount != 1 {
				t.Fatalf("Expected 1 leader, got %d (split brain detected)", leaderCount)
			}
		})
	}
}

func TestHeartbeats(t *testing.T) {
	// Create 3-node cluster
	cluster := NewTestCluster(t, 3)

	// Start all nodes
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Wait for leader
	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	// Wait for several heartbeat intervals
	time.Sleep(2 * time.Second)

	// Verify leader is still the same (heartbeats working)
	currentLeaderIdx := cluster.GetLeaderIndex()
	if currentLeaderIdx != leaderIdx {
		t.Errorf("Leader changed unexpectedly: %d -> %d", leaderIdx, currentLeaderIdx)
	}

	// Verify still exactly one leader
	AssertLeaderElected(t, cluster)
}

func TestMinorityNodeFailure(t *testing.T) {
	// Create 5-node cluster
	cluster := NewTestCluster(t, 5)

	// Start all nodes
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Wait for leader
	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	// Stop 2 non-leader nodes (minority)
	stoppedNodes := 0
	for i := 0; i < len(cluster.Nodes) && stoppedNodes < 2; i++ {
		if i != leaderIdx {
			cluster.StopNode(i)
			t.Logf("Stopped node-%d", i)
			stoppedNodes++
		}
	}

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Cluster should still have a leader (majority available)
	currentLeaderIdx := cluster.GetLeaderIndex()
	if currentLeaderIdx < 0 {
		t.Fatal("Cluster lost leader despite having majority")
	}

	// Verify exactly one leader
	AssertLeaderElected(t, cluster)
}

func TestMajorityNodeFailure(t *testing.T) {
	// Create 5-node cluster
	cluster := NewTestCluster(t, 5)

	// Start all nodes
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Wait for leader
	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	// Stop 3 nodes including leader (majority)
	// Stop them all at once to minimize race conditions
	// If we stop them sequentially, there's a window where elections can succeed
	stoppedIndices := []int{leaderIdx}
	stoppedNodes := 1
	for i := 0; i < len(cluster.Nodes) && stoppedNodes < 3; i++ {
		if i != leaderIdx {
			stoppedIndices = append(stoppedIndices, i)
			stoppedNodes++
		}
	}

	// Stop all nodes in quick succession (but still sequentially to avoid port conflicts)
	// Note: Even though we stop them quickly, there's still a small window for race conditions
	for _, idx := range stoppedIndices {
		cluster.StopNode(idx)
		// Small delay between stops to ensure proper cleanup
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for nodes to fully stop (HTTP servers to shut down, RPCs to be rejected)
	// This is critical - we need to ensure stopped nodes cannot grant votes
	// Reduced wait time - HTTP server shutdown is fast (1s timeout)
	time.Sleep(3 * time.Second) // Wait for HTTP servers to fully shut down

	// Wait for election timeout and state updates
	// Give enough time for any election attempts to complete and fail
	// Nodes need time to realize they can't get majority votes
	// With only 2 nodes remaining, they need 3 votes (majority of 5), so elections should fail
	// Election timeout is 150-300ms, so 2-3 seconds should be enough
	time.Sleep(3 * time.Second) // Wait for election attempts to fail

	// Cluster should NOT have a leader (no majority)
	// With only 2 nodes remaining out of 5, there's no majority (need 3)
	// A leader might have been elected BEFORE we stopped the nodes
	// In Raft, leaders don't automatically step down when they can't reach majority
	// However, they should eventually step down when they can't get responses from followers
	// or when election timeouts occur and new elections fail

	// Create a set of stopped node indices for quick lookup
	stoppedSet := make(map[int]bool)
	for _, idx := range stoppedIndices {
		stoppedSet[idx] = true
	}

	// Wait for any pre-existing leader to step down
	// Leaders should step down when they can't get AppendEntries responses from majority
	// Heartbeat interval is 50ms, so a few seconds should be enough
	time.Sleep(3 * time.Second) // Wait for leader to realize it can't reach majority

	// Check multiple times to catch any transient leader states
	// Give extra time between checks to ensure any transient leaders step down
	for i := 0; i < 5; i++ {
		leaderCount := cluster.CountLeaders()
		if leaderCount > 0 {
			// Check if the leader is one of the stopped nodes (shouldn't happen, but verify)
			leaderIdx := cluster.GetLeaderIndex()
			if leaderIdx >= 0 && stoppedSet[leaderIdx] {
				t.Errorf("Stopped node %d is still leader - this should not happen", leaderIdx)
				break
			}

			if i < 4 {
				// Wait longer and check again - might be transient
				// A leader might exist from before we stopped nodes, but it should step down
				// when it realizes it can't reach majority (heartbeats fail)
				time.Sleep(2 * time.Second)
				continue
			}
			// Final check - if there's still a leader after all this time, it's a problem
			// The leader should have stepped down when it couldn't reach majority
			t.Logf("Warning: Leader still exists after stopping majority nodes")
			t.Errorf("Cluster should not have leader without majority, got %d leaders (leaderIdx=%d)", leaderCount, leaderIdx)
		} else {
			break // No leader found, test passes
		}
	}
}
