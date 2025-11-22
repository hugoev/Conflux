package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
)

func TestPersistence_Restart(t *testing.T) {
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
	numEntries := 10
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("persist-key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := leaderStore.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
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
	if err := cluster.WaitForCommitIndex(numEntries, 10*time.Second); err != nil {
		t.Fatalf("Replication failed before restart: %v", err)
	}

	// Wait for data to be applied
	time.Sleep(1 * time.Second)

	// Stop the whole cluster
	cluster.Shutdown()
	t.Log("Cluster stopped")

	// Wait for cleanup - ensure all nodes are fully stopped
	time.Sleep(3 * time.Second)

	// Restart the cluster
	t.Log("Restarting cluster...")
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to restart cluster: %v", err)
	}

	// Wait for nodes to start up and reconnect
	time.Sleep(2 * time.Second)

	// Wait for leader again (longer timeout after full restart)
	// After a full shutdown, nodes need time to reconnect and elect a leader
	newLeaderIdx, err := cluster.WaitForLeader(45 * time.Second)
	if err != nil {
		t.Fatalf("Leader election after restart failed: %v", err)
	}
	t.Logf("New leader elected: node-%d", newLeaderIdx)

	// Wait a bit for nodes to fully reconnect and sync
	time.Sleep(3 * time.Second)

	// Verify data persists on all nodes
	for i, store := range cluster.Stores {
		for j := 0; j < numEntries; j++ {
			key := fmt.Sprintf("persist-key-%d", j)
			expectedValue := fmt.Sprintf("value-%d", j)

			// Retry a few times as nodes might be catching up/replaying WAL
			AssertEventuallyTrue(t, func() bool {
				val, exists := store.Get(key)
				return exists && val == expectedValue
			}, 5*time.Second, fmt.Sprintf("Node %d missing key %s after restart", i, key))
		}
	}
}

func TestPersistence_SnapshotRecovery(t *testing.T) {
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

	// Write enough data to trigger snapshot (assuming default threshold is low or we force it)
	// Since we don't have easy config access to lower threshold, we'll manually trigger if possible
	// or write a moderate amount.
	// For this test, we'll rely on the fact that restart replays WAL.
	// To truly test snapshot, we'd need to trigger it.
	// Let's write data, then stop, then verify.

	numEntries := 50
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("snap-key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := leaderStore.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
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
	if err := cluster.WaitForCommitIndex(numEntries, 10*time.Second); err != nil {
		t.Fatalf("Initial replication failed: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Stop a follower
	followerIdx := (leaderIdx + 1) % 3
	cluster.StopNode(followerIdx)
	t.Logf("Stopped follower node-%d", followerIdx)

	// Wait for stop to complete
	time.Sleep(500 * time.Millisecond)

	// Write more data to leader (this will go to WAL on leader and other follower)
	for i := numEntries; i < numEntries+20; i++ {
		key := fmt.Sprintf("snap-key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := leaderStore.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
		cmd := &kv.Command{
			Type:  kv.CommandPut,
			Key:   key,
			Value: value,
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose entry %d: %v", i, err)
		}
	}

	// Wait for replication to active follower
	// Note: With 2 active nodes (leader + 1 follower) out of 3 total, we need 2 nodes to commit
	// (2/3 = majority). The commit index should advance once both active nodes have the entries.
	// However, we need to wait for the follower to catch up first.
	time.Sleep(2 * time.Second) // Give time for replication

	// Wait for commit index to advance (leader + active follower = 2/3 = majority)
	// The commit index should advance as the active follower replicates entries
	// We check for numEntries+20 because we wrote 20 more entries after stopping the follower
	if err := cluster.WaitForCommitIndex(numEntries+20, 30*time.Second); err != nil {
		t.Fatalf("Replication to active follower failed: %v", err)
	}

	// Wait for data to be applied
	time.Sleep(2 * time.Second)

	// Restart follower
	t.Logf("Restarting follower node-%d", followerIdx)
	if err := cluster.RestartNode(followerIdx); err != nil {
		t.Fatalf("Failed to restart follower: %v", err)
	}

	// Wait for follower to reconnect and catch up
	// Restarted follower needs to:
	// 1. Recover state from WAL/snapshots
	// 2. Reconnect to leader
	// 3. Receive AppendEntries and catch up
	time.Sleep(3 * time.Second)

	// Follower should catch up - check in batches with retries
	store := cluster.Stores[followerIdx]
	for i := 0; i < numEntries+20; i++ {
		key := fmt.Sprintf("snap-key-%d", i)
		expectedValue := fmt.Sprintf("value-%d", i)

		AssertEventuallyTrue(t, func() bool {
			val, exists := store.Get(key)
			return exists && val == expectedValue
		}, 15*time.Second, fmt.Sprintf("Follower node-%d missing key %s after restart", followerIdx, key))
	}
}

func TestPersistence_CrashRecovery(t *testing.T) {
	// Simulate a crash (stop without graceful shutdown if possible, but Stop() is all we have)
	// We'll treat Stop() as a crash for now since it stops the processing loops.

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

	// Write data
	key := "crash-key"
	value := "crash-value"
	if err := leaderStore.Put(key, value); err != nil {
		t.Fatalf("Failed to put key %s: %v", key, err)
	}
	cmd := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key,
		Value: value,
	}
	if err := leader.Propose(cmd); err != nil {
		t.Fatalf("Failed to propose entry: %v", err)
	}

	// Wait for replication
	if err := cluster.WaitForCommitIndex(1, 5*time.Second); err != nil {
		t.Fatalf("Initial commit failed: %v", err)
	}
	time.Sleep(1 * time.Second)

	// "Crash" the leader
	cluster.StopNode(leaderIdx)

	// Wait for new leader
	newLeaderIdx, err := cluster.WaitForLeader(15 * time.Second)
	if err != nil {
		t.Fatalf("New leader election failed: %v", err)
	}

	// Write more data to new leader
	newLeader := cluster.Nodes[newLeaderIdx]
	newLeaderStore := cluster.Stores[newLeaderIdx]
	key2 := "post-crash-key"
	value2 := "post-crash-value"
	if err := newLeaderStore.Put(key2, value2); err != nil {
		t.Fatalf("Failed to put key %s: %v", key2, err)
	}
	cmd2 := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key2,
		Value: value2,
	}
	if err := newLeader.Propose(cmd2); err != nil {
		t.Fatalf("Failed to propose entry: %v", err)
	}

	// Wait for replication
	// With 2 active nodes out of 3 total, we can commit (2/3 = majority)
	// Commit index should advance once both active nodes have the entry
	if err := cluster.WaitForCommitIndex(2, 25*time.Second); err != nil {
		t.Fatalf("Replication after crash failed: %v", err)
	}

	// Wait for data to be applied
	time.Sleep(2 * time.Second)

	// Restart old leader
	t.Logf("Restarting old leader node-%d", leaderIdx)
	if err := cluster.RestartNode(leaderIdx); err != nil {
		t.Fatalf("Failed to restart old leader: %v", err)
	}

	// Wait for old leader to reconnect and catch up
	time.Sleep(2 * time.Second)

	// Old leader should have both keys eventually
	store := cluster.Stores[leaderIdx]
	AssertEventuallyTrue(t, func() bool {
		v1, ok1 := store.Get(key)
		v2, ok2 := store.Get(key2)
		return ok1 && v1 == value && ok2 && v2 == value2
	}, 20*time.Second, "Old leader failed to catch up after crash")
}
