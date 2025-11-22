package raft

import (
	"fmt"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestLeaderElection(t *testing.T) {
	// Create 2 nodes (simplified for debugging)
	nodes := make([]*Node, 2)
	ports := []int{20001, 20002}
	peers := []string{
		fmt.Sprintf("127.0.0.1:%d", ports[0]),
		fmt.Sprintf("127.0.0.1:%d", ports[1]),
	}

	logger, _ := zap.NewDevelopment()

	// Cleanup
	defer func() {
		for _, node := range nodes {
			if node != nil {
				if err := node.Stop(); err != nil {
					t.Logf("Failed to stop node: %v", err)
				}
			}
		}
	}()

	for i := 0; i < 2; i++ {
		nodeID := peers[i] // Use address as ID for simplicity in this test
		nodes[i] = NewNode(nodeID, peers, "/tmp/raft-test-"+nodeID, logger)
		nodes[i].Start()
		// Start transport
		listener, err := net.Listen("tcp", peers[i])
		if err != nil {
			t.Fatalf("Failed to bind: %v", err)
		}
		if err := nodes[i].StartTransport(listener); err != nil {
			t.Fatalf("Failed to start transport: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Give it a moment to bind
	}

	// Wait for election (election timeout is 2-4s, so wait longer)
	time.Sleep(6 * time.Second)

	// Check for leader
	leaders := 0
	for _, n := range nodes {
		if n.IsLeader() {
			leaders++
			t.Logf("Node %s is leader", n.nodeID)
		}
	}

	if leaders != 1 {
		t.Errorf("Expected 1 leader, got %d", leaders)
	}
}
