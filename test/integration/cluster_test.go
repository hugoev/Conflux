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

	// Wait for new leader election
	time.Sleep(500 * time.Millisecond) // Give time for election timeout

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

		// Wait for new leader
		time.Sleep(500 * time.Millisecond)
		newLeaderIdx, err := cluster.WaitForLeader(15 * time.Second)
		if err != nil {
			t.Fatalf("Failover %d failed: %v", i+1, err)
		}

		// Verify new leader is different
		if newLeaderIdx == leaderIdx {
			t.Errorf("Failover %d: new leader should be different", i+1)
		}

		// Restart old leader (becomes follower)
		if err := cluster.RestartNode(leaderIdx); err != nil {
			t.Fatalf("Failed to restart node-%d: %v", leaderIdx, err)
		}

		// Small delay for node to rejoin
		time.Sleep(200 * time.Millisecond)

		// Verify exactly one leader
		AssertLeaderElected(t, cluster)

		leaderIdx = newLeaderIdx
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
	cluster.StopNode(leaderIdx)
	stoppedNodes := 1
	for i := 0; i < len(cluster.Nodes) && stoppedNodes < 3; i++ {
		if i != leaderIdx {
			cluster.StopNode(i)
			stoppedNodes++
		}
	}

	// Wait for election timeout
	time.Sleep(1 * time.Second)

	// Cluster should NOT have a leader (no majority)
	leaderCount := cluster.CountLeaders()
	if leaderCount > 0 {
		t.Errorf("Cluster should not have leader without majority, got %d leaders", leaderCount)
	}
}
