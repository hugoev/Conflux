package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
)

func TestLogReplication(t *testing.T) {
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

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Propose several entries
	numEntries := 10
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)

		// Write to leader
		if err := leaderStore.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}

		// Propose to Raft as Command object
		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   key,
			Value: value,
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose entry %d: %v", i, err)
		}
	}

	// Wait for replication and commit
	expectedCommitIndex := numEntries
	if err := cluster.WaitForCommitIndex(expectedCommitIndex, 10*time.Second); err != nil {
		t.Fatalf("Replication failed: %v", err)
	}

	// Wait for data to be applied to all stores
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key-%d", i)
		expectedValue := fmt.Sprintf("value-%d", i)
		if err := cluster.WaitForDataReplication(key, expectedValue, 5*time.Second); err != nil {
			t.Errorf("Data replication failed for key %s: %v", key, err)
		}
	}

	// Verify data consistency across all stores
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key-%d", i)
		expectedValue := fmt.Sprintf("value-%d", i)

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

func TestReplicationAfterFailover(t *testing.T) {
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

	// Write some data to initial leader
	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("before-failover-%d", i)
		value := fmt.Sprintf("value-%d", i)
		leaderStore.Put(key, value)
		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   key,
			Value: value,
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose: %v", err)
		}
	}

	// Wait for replication
	time.Sleep(1 * time.Second)

	// Stop the leader
	cluster.StopNode(leaderIdx)
	t.Logf("Stopped leader node-%d", leaderIdx)

	// Wait for new leader
	time.Sleep(500 * time.Millisecond)
	newLeaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("New leader election failed: %v", err)
	}

	t.Logf("New leader: node-%d", newLeaderIdx)

	// Write data to new leader
	newLeader := cluster.Nodes[newLeaderIdx]
	newLeaderStore := cluster.Stores[newLeaderIdx]

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("after-failover-%d", i)
		value := fmt.Sprintf("value-%d", i)
		newLeaderStore.Put(key, value)
		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   key,
			Value: value,
		}
		if err := newLeader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose: %v", err)
		}
	}

	// Wait for replication
	time.Sleep(1 * time.Second)

	// Verify data on remaining nodes
	for nodeIdx, store := range cluster.Stores {
		if nodeIdx == leaderIdx {
			continue // Skip stopped node
		}

		// Check pre-failover data
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("before-failover-%d", i)
			if _, exists := store.Get(key); !exists {
				t.Errorf("Node %d missing pre-failover key %s", nodeIdx, key)
			}
		}

		// Check post-failover data
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("after-failover-%d", i)
			if _, exists := store.Get(key); !exists {
				t.Errorf("Node %d missing post-failover key %s", nodeIdx, key)
			}
		}
	}
}

func TestCommitIndexAdvancement(t *testing.T) {
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

	leader := cluster.Nodes[leaderIdx]

	// Get initial commit index
	initialCommitIndex := leader.GetCommitIndex()

	// Propose entries
	numEntries := 5
	for i := 0; i < numEntries; i++ {
		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   fmt.Sprintf("entry-%d", i),
			Value: fmt.Sprintf("value-%d", i),
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose entry: %v", err)
		}
	}

	// Wait for commit index to advance
	expectedCommitIndex := initialCommitIndex + numEntries
	AssertEventuallyTrue(t, func() bool {
		return leader.GetCommitIndex() >= expectedCommitIndex
	}, 5*time.Second, "commit index did not advance")

	// Verify all nodes advanced commit index
	for i, node := range cluster.Nodes {
		commitIndex := node.GetCommitIndex()
		if commitIndex < expectedCommitIndex {
			t.Errorf("Node %d commit index %d < expected %d", i, commitIndex, expectedCommitIndex)
		}
	}
}

func TestReadAfterWrite(t *testing.T) {
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

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Write data
	key := "test-key"
	value := "test-value"
	leaderStore.Put(key, value)
	cmd := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key,
		Value: value,
	}
	if err := leader.Propose(cmd); err != nil {
		t.Fatalf("Failed to propose: %v", err)
	}

	// Wait for replication
	time.Sleep(500 * time.Millisecond)

	// Read from all nodes
	for i, store := range cluster.Stores {
		readValue, exists := store.Get(key)
		if !exists {
			t.Errorf("Node %d: key not found", i)
			continue
		}
		if readValue != value {
			t.Errorf("Node %d: got %s, want %s", i, readValue, value)
		}
	}
}

func TestConcurrentWrites(t *testing.T) {
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

	leader := cluster.Nodes[leaderIdx]
	leaderStore := cluster.Stores[leaderIdx]

	// Concurrent writes
	numWrites := 20
	done := make(chan bool, numWrites)

	for i := 0; i < numWrites; i++ {
		go func(idx int) {
			key := fmt.Sprintf("concurrent-key-%d", idx)
			value := fmt.Sprintf("value-%d", idx)
			leaderStore.Put(key, value)
			cmd := &kv.Command{
				Type:  kv.CommandPut,
				Key:   key,
				Value: value,
			}
			if err := leader.Propose(cmd); err != nil {
				t.Errorf("Failed to propose: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < numWrites; i++ {
		<-done
	}

	// Wait for replication
	time.Sleep(2 * time.Second)

	// Verify all writes replicated
	for i := 0; i < numWrites; i++ {
		key := fmt.Sprintf("concurrent-key-%d", i)
		expectedValue := fmt.Sprintf("value-%d", i)

		for nodeIdx, store := range cluster.Stores {
			value, exists := store.Get(key)
			if !exists {
				t.Errorf("Node %d missing concurrent key %s", nodeIdx, key)
				continue
			}
			if value != expectedValue {
				t.Errorf("Node %d key %s: got %s, want %s", nodeIdx, key, value, expectedValue)
			}
		}
	}
}
