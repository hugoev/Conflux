package raft

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestTransport(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	node := NewNode("test-node", []string{}, "/tmp/raft-transport-test", logger)
	port := 20001
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Start transport
	if err := node.StartTransport(addr); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Check if server is listening
	url := fmt.Sprintf("http://%s/raft/request_vote", addr)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest { // We expect 400 because body is empty
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	t.Log("Transport is working")
}
