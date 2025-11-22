package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
)

// TestNetworkPartition tests behavior during network partitions
func TestNetworkPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network partition test in short mode")
	}

	// Create 5-node cluster for better partition scenarios
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

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Write initial data
	key := "partition-test"
	value := "before-partition"
	if err := leaderStore.Put(key, value); err != nil {
		t.Fatalf("Failed to put key: %v", err)
	}
	cmd := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key,
		Value: value,
	}
	if err := leader.Propose(cmd); err != nil {
		t.Fatalf("Failed to propose: %v", err)
	}

	// Wait for replication
	time.Sleep(1 * time.Second)

	// Partition: Stop 2 nodes (minority partition)
	// Majority (3 nodes) should continue operating
	stoppedNodes := []int{}
	for i := 0; i < len(cluster.Nodes) && len(stoppedNodes) < 2; i++ {
		if i != leaderIdx {
			cluster.StopNode(i)
			stoppedNodes = append(stoppedNodes, i)
		}
	}

	t.Logf("Stopped nodes: %v (minority partition)", stoppedNodes)

	// Wait a bit for partition to be detected
	time.Sleep(500 * time.Millisecond)

	// Majority should still have a leader
	currentLeaderIdx := cluster.GetLeaderIndex()
	if currentLeaderIdx < 0 {
		t.Fatal("Majority partition lost leader")
	}

	// Write to majority partition
	majorityLeader := cluster.Nodes[currentLeaderIdx]
	majorityStore := cluster.Stores[currentLeaderIdx]

	key2 := "partition-test-2"
	value2 := "during-partition"
	if err := majorityStore.Put(key2, value2); err != nil {
		t.Fatalf("Failed to put key in majority: %v", err)
	}
	cmd2 := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key2,
		Value: value2,
	}
	if err := majorityLeader.Propose(cmd2); err != nil {
		t.Fatalf("Failed to propose in majority: %v", err)
	}

	// Wait for replication in majority
	time.Sleep(1 * time.Second)

	// Verify majority has both keys
	if val, exists := majorityStore.Get(key); !exists || val != value {
		t.Errorf("Majority missing initial key: exists=%v, val=%s", exists, val)
	}
	if val, exists := majorityStore.Get(key2); !exists || val != value2 {
		t.Errorf("Majority missing partition key: exists=%v, val=%s", exists, val)
	}

	// Restart stopped nodes (partition heals)
	for _, idx := range stoppedNodes {
		if err := cluster.RestartNode(idx); err != nil {
			t.Fatalf("Failed to restart node-%d: %v", idx, err)
		}
		// Small delay between restarts
		time.Sleep(200 * time.Millisecond)
	}

	t.Log("Partition healed, waiting for nodes to catch up...")
	time.Sleep(2 * time.Second)

	// Verify all nodes eventually have both keys (with shorter timeout per node)
	for i, store := range cluster.Stores {
		// Skip stopped nodes that weren't restarted
		isStopped := false
		for _, stoppedIdx := range stoppedNodes {
			if i == stoppedIdx {
				isStopped = false // It was restarted
				break
			}
		}
		if isStopped {
			continue
		}

		AssertEventuallyTrue(t, func() bool {
			val1, ok1 := store.Get(key)
			val2, ok2 := store.Get(key2)
			return ok1 && val1 == value && ok2 && val2 == value2
		}, 5*time.Second, fmt.Sprintf("Node %d did not catch up after partition", i))
	}
}

// TestSplitBrainPrevention ensures no split-brain scenarios occur
func TestSplitBrainPrevention(t *testing.T) {
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

	// Stop leader and one follower simultaneously (simulating network partition)
	// This leaves only one node, which should NOT become leader (no majority)
	cluster.StopNode(leaderIdx)
	followerIdx := (leaderIdx + 1) % 3
	cluster.StopNode(followerIdx)

	t.Logf("Stopped leader node-%d and follower node-%d", leaderIdx, followerIdx)

	// Wait for election timeout
	time.Sleep(2 * time.Second)

	// Remaining node should NOT be leader (no majority)
	leaderCount := cluster.CountLeaders()
	if leaderCount > 0 {
		t.Errorf("Split brain detected: single node became leader without majority (count=%d)", leaderCount)
	}

	// Restart one node to form majority
	if err := cluster.RestartNode(followerIdx); err != nil {
		t.Fatalf("Failed to restart node-%d: %v", followerIdx, err)
	}

	// Now we have 2 nodes, but still no majority in 3-node cluster
	// Wait a bit
	time.Sleep(1 * time.Second)

	// Still should not have leader (2/3 is not majority)
	leaderCount = cluster.CountLeaders()
	if leaderCount > 0 {
		t.Errorf("Split brain: 2 nodes elected leader without majority")
	}

	// Restart the original leader to form majority
	if err := cluster.RestartNode(leaderIdx); err != nil {
		t.Fatalf("Failed to restart original leader: %v", err)
	}

	// Now should have leader
	newLeaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election after majority restored failed: %v", err)
	}

	t.Logf("Leader elected after majority restored: node-%d", newLeaderIdx)
	AssertLeaderElected(t, cluster)
}

// TestConcurrentOperations tests concurrent operations across multiple clients
func TestConcurrentOperations(t *testing.T) {
	cluster := NewTestCluster(t, 3)

	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Concurrent writes from multiple "clients"
	numClients := 10
	numWritesPerClient := 20
	errors := make(chan error, numClients*numWritesPerClient)

	for clientID := 0; clientID < numClients; clientID++ {
		go func(cid int) {
			for i := 0; i < numWritesPerClient; i++ {
				key := fmt.Sprintf("client-%d-key-%d", cid, i)
				value := fmt.Sprintf("value-%d-%d", cid, i)

				if err := leaderStore.Put(key, value); err != nil {
					errors <- fmt.Errorf("client %d write %d failed: %w", cid, i, err)
					continue
				}

				cmd := &kv.Command{
					Type:  kv.CommandPut,
					Key:   key,
					Value: value,
				}
				if err := leader.Propose(cmd); err != nil {
					errors <- fmt.Errorf("client %d propose %d failed: %w", cid, i, err)
				}
			}
		}(clientID)
	}

	// Wait for all writes
	time.Sleep(3 * time.Second)

	// Check for errors
	close(errors)
	errorCount := 0
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent operation error: %v", err)
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fatalf("Too many concurrent operation errors: %d", errorCount)
	}

	// Wait for replication
	time.Sleep(2 * time.Second)

	// Verify all writes were replicated
	for clientID := 0; clientID < numClients; clientID++ {
		for i := 0; i < numWritesPerClient; i++ {
			key := fmt.Sprintf("client-%d-key-%d", clientID, i)
			expectedValue := fmt.Sprintf("value-%d-%d", clientID, i)

			for nodeIdx, store := range cluster.Stores {
				value, exists := store.Get(key)
				if !exists {
					t.Errorf("Node %d missing key %s", nodeIdx, key)
					continue
				}
				if value != expectedValue {
					t.Errorf("Node %d key %s: got %s, want %s", nodeIdx, key, value, expectedValue)
				}
			}
		}
	}
}

// TestPerformanceThroughput tests write throughput under load
func TestPerformanceThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	cluster := NewTestCluster(t, 3)

	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	leaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Measure write throughput
	numWrites := 1000
	startTime := time.Now()

	for i := 0; i < numWrites; i++ {
		key := fmt.Sprintf("perf-key-%d", i)
		value := fmt.Sprintf("value-%d", i)

		if err := leaderStore.Put(key, value); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}

		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   key,
			Value: value,
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Propose %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(startTime)
	throughput := float64(numWrites) / elapsed.Seconds()

	t.Logf("Write throughput: %.2f ops/sec (%d writes in %v)", throughput, numWrites, elapsed)

	// Wait for replication and commit
	if err := cluster.WaitForCommitIndex(numWrites, 60*time.Second); err != nil {
		t.Fatalf("Replication failed: %v", err)
	}

	// Wait for data to be applied to all stores - check sample keys first
	// Check a few keys to ensure replication is working
	sampleKeys := []int{0, numWrites / 4, numWrites / 2, numWrites * 3 / 4, numWrites - 1}
	for _, idx := range sampleKeys {
		key := fmt.Sprintf("perf-key-%d", idx)
		expectedValue := fmt.Sprintf("value-%d", idx)
		if err := cluster.WaitForDataReplication(key, expectedValue, 10*time.Second); err != nil {
			t.Fatalf("Sample key %s did not replicate: %v", key, err)
		}
	}

	// Now verify all writes with retries for missing keys
	missingKeys := make(map[int]bool)
	for i := 0; i < numWrites; i++ {
		missingKeys[i] = true
	}

	// Retry verification with exponential backoff
	maxRetries := 5
	for retry := 0; retry < maxRetries && len(missingKeys) > 0; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry) * time.Second)
		}

		for i := range missingKeys {
			key := fmt.Sprintf("perf-key-%d", i)
			expectedValue := fmt.Sprintf("value-%d", i)
			allMatch := true

			for nodeIdx, store := range cluster.Stores {
				value, exists := store.Get(key)
				if !exists || value != expectedValue {
					allMatch = false
					break
				}
			}

			if allMatch {
				delete(missingKeys, i)
			}
		}
	}

	// Report any remaining missing keys
	if len(missingKeys) > 0 {
		for i := range missingKeys {
			key := fmt.Sprintf("perf-key-%d", i)
			for nodeIdx, store := range cluster.Stores {
				value, exists := store.Get(key)
				if !exists {
					t.Errorf("Node %d missing key %s", nodeIdx, key)
				} else if value != fmt.Sprintf("value-%d", i) {
					t.Errorf("Node %d key %s: got %s, want value-%d", nodeIdx, key, value, i)
				}
			}
		}
		if len(missingKeys) > 100 {
			t.Fatalf("Too many keys missing (%d), test unreliable", len(missingKeys))
		}
	}
}
