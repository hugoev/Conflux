package integration

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
	"github.com/hugovillarreal/conflux/pkg/raft"
	"go.uber.org/zap"
)

// TestCluster represents a test Raft cluster
type TestCluster struct {
	Nodes   []*raft.Node
	Stores  []*kv.Store
	Ports   []int
	DataDir string
	t       *testing.T
	mu      sync.Mutex
}

// NewTestCluster creates a new test cluster with n nodes
func NewTestCluster(t *testing.T, n int) *TestCluster {
	t.Helper()

	// Create temporary directory for all nodes
	dataDir := t.TempDir()

	// Allocate ports for all nodes
	ports := allocatePorts(t, n)

	// Build peer list
	peers := make([]string, n)
	for i := 0; i < n; i++ {
		peers[i] = fmt.Sprintf("localhost:%d", ports[i])
	}

	cluster := &TestCluster{
		Nodes:   make([]*raft.Node, n),
		Stores:  make([]*kv.Store, n),
		Ports:   ports,
		DataDir: dataDir,
		t:       t,
	}

	// Create nodes
	for i := 0; i < n; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		nodeDataDir := filepath.Join(dataDir, nodeID)
		if err := os.MkdirAll(nodeDataDir, 0755); err != nil {
			t.Fatalf("Failed to create node data dir: %v", err)
		}

		// Use Error level logger to reduce noise and improve performance
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		logger, _ := config.Build()

		store := kv.NewStore()
		node := raft.NewNode(nodeID, peers, nodeDataDir, logger)

		// Wire up ApplyCh to Store (simulate main.go)
		go func(n *raft.Node, s *kv.Store) {
			for msg := range n.ApplyCh() {
				if msg.CommandValid {
					if cmd, ok := msg.Command.(*kv.Command); ok {
						s.Apply(cmd)
					} else if cmdMap, ok := msg.Command.(map[string]interface{}); ok {
						// Handle map conversion
						cmd := &kv.Command{}
						if typeStr, ok := cmdMap["type"].(string); ok {
							cmd.Type = kv.CommandType(typeStr)
						}
						if key, ok := cmdMap["key"].(string); ok {
							cmd.Key = key
						}
						if value, ok := cmdMap["value"].(string); ok {
							cmd.Value = value
						}
						s.Apply(cmd)
					}
				}
			}
		}(node, store)

		cluster.Nodes[i] = node
		cluster.Stores[i] = store
	}

	// Register cleanup
	t.Cleanup(func() {
		cluster.Shutdown()
	})

	return cluster
}

// Start starts all nodes in the cluster
func (c *TestCluster) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.Nodes {
		port := c.Ports[i]
		node.Start()
		if err := node.StartTransport(fmt.Sprintf(":%d", port)); err != nil {
			return fmt.Errorf("failed to start transport for node %d: %w", i, err)
		}
	}

	return nil
}

// Shutdown stops all nodes in the cluster
func (c *TestCluster) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.Nodes {
		if node != nil {
			node.Stop()
		}
	}
}

// WaitForLeader waits for a leader to be elected
func (c *TestCluster) WaitForLeader(timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		leaderIdx := c.GetLeaderIndex()
		if leaderIdx >= 0 {
			return leaderIdx, nil
		}
	}

	return -1, fmt.Errorf("no leader elected within %v", timeout)
}

// GetLeaderIndex returns the index of the current leader, or -1 if none
func (c *TestCluster) GetLeaderIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.Nodes {
		if node.IsLeader() {
			return i
		}
	}
	return -1
}

// GetLeader returns the current leader node, or nil if none
func (c *TestCluster) GetLeader() *raft.Node {
	idx := c.GetLeaderIndex()
	if idx < 0 {
		return nil
	}
	return c.Nodes[idx]
}

// CountLeaders returns the number of nodes that think they are leader
func (c *TestCluster) CountLeaders() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, node := range c.Nodes {
		if node.IsLeader() {
			count++
		}
	}
	return count
}

// StopNode stops a specific node by index
func (c *TestCluster) StopNode(idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if idx >= 0 && idx < len(c.Nodes) {
		c.Nodes[idx].Stop()
	}
}

// RestartNode restarts a specific node by index
func (c *TestCluster) RestartNode(idx int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if idx < 0 || idx >= len(c.Nodes) {
		return fmt.Errorf("invalid node index: %d", idx)
	}

	node := c.Nodes[idx]
	port := c.Ports[idx]

	// Stop if running
	node.Stop()

	// Small delay to ensure cleanup
	time.Sleep(100 * time.Millisecond)

	// Restart
	node.Start()
	return node.StartTransport(fmt.Sprintf(":%d", port))
}

// WaitForCommitIndex waits for all nodes to reach at least the given commit index
func (c *TestCluster) WaitForCommitIndex(minIndex int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		allReached := true
		for _, node := range c.Nodes {
			if node.GetCommitIndex() < minIndex {
				allReached = false
				break
			}
		}

		if allReached {
			return nil
		}
	}

	return fmt.Errorf("nodes did not reach commit index %d within %v", minIndex, timeout)
}

// allocatePorts allocates n available ports for testing
func allocatePorts(t *testing.T, n int) []int {
	t.Helper()

	ports := make([]int, n)
	listeners := make([]net.Listener, n)

	// Allocate ports
	for i := 0; i < n; i++ {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to allocate port: %v", err)
		}
		listeners[i] = listener
		ports[i] = listener.Addr().(*net.TCPAddr).Port
	}

	// Close listeners to free ports
	for _, listener := range listeners {
		listener.Close()
	}

	// Small delay to ensure ports are released
	time.Sleep(50 * time.Millisecond)

	return ports
}

// AssertLeaderElected asserts that exactly one leader exists
func AssertLeaderElected(t *testing.T, cluster *TestCluster) int {
	t.Helper()

	leaderCount := cluster.CountLeaders()
	if leaderCount != 1 {
		t.Fatalf("Expected exactly 1 leader, got %d", leaderCount)
	}

	leaderIdx := cluster.GetLeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("No leader found")
	}

	return leaderIdx
}

// AssertEventuallyTrue retries a condition until it's true or timeout
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		<-ticker.C
	}

	t.Fatalf("Condition not met within %v: %s", timeout, msg)
}
