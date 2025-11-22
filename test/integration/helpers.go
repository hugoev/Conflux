package integration

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
	"github.com/hugovillarreal/conflux/pkg/raft"
	"go.uber.org/zap"
)

// parseByteCommand parses a byte command like "PUT key value" into a Command
func parseByteCommand(cmdBytes []byte) *kv.Command {
	parts := strings.Fields(string(cmdBytes))
	if len(parts) < 2 {
		return nil
	}

	cmdType := strings.ToUpper(parts[0])
	cmd := &kv.Command{
		Type: kv.CommandType(cmdType),
		Key:  parts[1],
	}

	if len(parts) > 2 && cmdType == "PUT" {
		cmd.Value = strings.Join(parts[2:], " ")
	}

	return cmd
}

// TestCluster represents a test Raft cluster
type TestCluster struct {
	Nodes     []*raft.Node
	Stores    []*kv.Store
	Ports     []int
	Listeners []net.Listener
	DataDir   string
	t         *testing.T
	mu        sync.Mutex
}

// NewTestCluster creates a new test cluster with n nodes
func NewTestCluster(t *testing.T, n int) *TestCluster {
	t.Helper()

	// Create temporary directory for all nodes
	dataDir := t.TempDir()

	// Allocate ports and listeners
	ports, listeners := allocatePorts(t, n)

	// Build peer list
	peers := make([]string, n)
	for i := 0; i < n; i++ {
		peers[i] = fmt.Sprintf("127.0.0.1:%d", ports[i])
	}

	cluster := &TestCluster{
		Nodes:     make([]*raft.Node, n),
		Stores:    make([]*kv.Store, n),
		Ports:     ports,
		Listeners: listeners,
		DataDir:   dataDir,
		t:         t,
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

		// Filter out self from peers
		nodePeers := make([]string, 0, n-1)
		for j, p := range peers {
			if i != j {
				nodePeers = append(nodePeers, p)
			}
		}

		store := kv.NewStore()
		node := raft.NewNode(nodeID, nodePeers, nodeDataDir, logger)

		// Initialize persistence
		if err := node.InitializePersistence(); err != nil {
			t.Fatalf("Failed to initialize persistence for node %s: %v", nodeID, err)
		}

		// Wire up ApplyCh to Store (simulate main.go)
		// This goroutine will be recreated in Start() if node is restarted
		go func(n *raft.Node, s *kv.Store) {
			applyCh := n.ApplyCh()
			if applyCh == nil {
				return // Channel not ready or closed
			}
			for msg := range applyCh {
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
					} else if cmdBytes, ok := msg.Command.([]byte); ok {
						// Parse byte command like "PUT key value"
						cmd := parseByteCommand(cmdBytes)
						if cmd != nil {
							s.Apply(cmd)
						}
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
		// Always reset apply channel before starting (in case node was stopped)
		node.ResetApplyCh()

		// Reinitialize persistence if needed
		if err := node.InitializePersistence(); err != nil {
			return fmt.Errorf("failed to initialize persistence for node %d: %w", i, err)
		}

		node.Start()

		// Recreate apply goroutine (channel was reset, need new goroutine)
		// This goroutine applies committed log entries to the store
		store := c.Stores[i]
		go func(n *raft.Node, s *kv.Store) {
			applyCh := n.ApplyCh()
			if applyCh == nil {
				return // Channel not ready or closed
			}
			for msg := range applyCh {
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
					} else if cmdBytes, ok := msg.Command.([]byte); ok {
						// Parse byte command like "PUT key value"
						cmd := parseByteCommand(cmdBytes)
						if cmd != nil {
							s.Apply(cmd)
						}
					}
				}
			}
		}(node, store)

		// Use existing listener if available (from allocatePorts)
		// Note: StartTransport takes ownership of the listener? No, http.Server.Serve does not close it on error usually, but does on Shutdown.
		// However, we need to handle restarts. If we restart, we need a NEW listener because the old one is closed.

		var listener net.Listener
		if c.Listeners[i] != nil {
			listener = c.Listeners[i]
			// Clear it so we don't reuse it blindly on restart if we didn't mean to
		} else {
			// This case happens on RestartNode where we need a new listener
			var err error
			listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.Ports[i]))
			if err != nil {
				return fmt.Errorf("failed to bind for node %d: %w", i, err)
			}
		}

		if err := node.StartTransport(listener); err != nil {
			return fmt.Errorf("failed to start transport for node %d: %w", i, err)
		}
	}

	return nil
}

// Shutdown stops all nodes in the cluster
func (c *TestCluster) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.Nodes {
		if node != nil {
			if err := node.Stop(); err != nil {
				// Log error but continue shutdown
				fmt.Printf("Failed to stop node: %v\n", err)
			}
		}
		// Clear listeners after shutdown so they can be recreated on restart
		c.Listeners[i] = nil
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
		if node != nil && !node.IsStopped() && node.IsLeader() {
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
// Only counts nodes that are actually running (not stopped)
func (c *TestCluster) CountLeaders() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, node := range c.Nodes {
		if node == nil {
			continue
		}
		// Only count leaders that are not stopped
		if !node.IsStopped() && node.IsLeader() {
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
		if err := c.Nodes[idx].Stop(); err != nil {
			c.t.Logf("Failed to stop node %d: %v", idx, err)
		}
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

	// Stop the node
	if err := node.Stop(); err != nil {
		c.t.Logf("Failed to stop node during restart: %v", err)
	}

	// Wait longer to ensure cleanup and port release
	time.Sleep(500 * time.Millisecond)

	// Reset apply channel (since Stop closed it)
	node.ResetApplyCh()

	// Reinitialize persistence (WAL, snapshot, recovery)
	if err := node.InitializePersistence(); err != nil {
		return fmt.Errorf("failed to initialize persistence for restart: %w", err)
	}

	// Restart the node's main loops
	node.Start()

	// Recreate apply goroutine for restarted node
	store := c.Stores[idx]
	go func(n *raft.Node, s *kv.Store) {
		applyCh := n.ApplyCh()
		if applyCh == nil {
			return // Channel not ready or closed
		}
		for msg := range applyCh {
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
				} else if cmdBytes, ok := msg.Command.([]byte); ok {
					// Parse byte command like "PUT key value"
					cmd := parseByteCommand(cmdBytes)
					if cmd != nil {
						s.Apply(cmd)
					}
				}
			}
		}
	}(node, store)

	// Create a new listener for the restart
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("failed to bind for restart: %w", err)
	}

	// Start transport with the new listener
	if err := node.StartTransport(listener); err != nil {
		return fmt.Errorf("failed to start transport for restart: %w", err)
	}

	return nil
}

// WaitForCommitIndex waits for all active (non-stopped) nodes to reach at least the given commit index
func (c *TestCluster) WaitForCommitIndex(minIndex int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		allReached := true
		c.mu.Lock()
		for _, node := range c.Nodes {
			// Skip stopped nodes - they don't participate in commit decisions
			if node != nil && node.IsStopped() {
				continue
			}
			if node == nil || node.GetCommitIndex() < minIndex {
				allReached = false
				c.mu.Unlock()
				break
			}
		}
		c.mu.Unlock()

		if allReached {
			// Give additional time for apply loop to process
			time.Sleep(200 * time.Millisecond)
			return nil
		}
	}

	return fmt.Errorf("nodes did not reach commit index %d within %v", minIndex, timeout)
}

// WaitForDataReplication waits for a key to appear in all stores
// Only checks stores for nodes that are not stopped
func (c *TestCluster) WaitForDataReplication(key, expectedValue string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		allHaveData := true
		c.mu.Lock()
		for i, store := range c.Stores {
			// Skip stopped nodes
			if c.Nodes[i] != nil && c.Nodes[i].IsStopped() {
				continue
			}
			value, exists := store.Get(key)
			if !exists || value != expectedValue {
				allHaveData = false
				c.mu.Unlock()
				break
			}
		}
		c.mu.Unlock()

		if allHaveData {
			return nil
		}
	}

	return fmt.Errorf("data for key %s did not replicate to all nodes within %v", key, timeout)
}

// allocatePorts allocates n available ports for testing
func allocatePorts(t *testing.T, n int) ([]int, []net.Listener) {
	t.Helper()

	ports := make([]int, n)
	listeners := make([]net.Listener, n)

	// Allocate ports
	for i := 0; i < n; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to allocate port: %v", err)
		}
		listeners[i] = listener
		ports[i] = listener.Addr().(*net.TCPAddr).Port
	}

	// Do NOT close listeners here. We pass them to the nodes.

	return ports, listeners
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
