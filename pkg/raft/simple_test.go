package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSimpleRequestVote(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a single node
	peers := []string{"127.0.0.1:21001"}
	node := NewNode("test-node", []string{}, "/tmp/raft-transport-test", logger)
	node.Start()

	// Start transport
	listener, err := net.Listen("tcp", peers[0])
	if err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}
	if err := node.StartTransport(listener); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer func() {
		if err := node.Stop(); err != nil {
			t.Logf("Failed to stop node: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Send a RequestVote RPC
	args := RequestVoteArgs{
		Term:         1,
		CandidateID:  "test-candidate",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Failed to marshal args: %v", err)
	}

	url := fmt.Sprintf("http://%s/raft/request_vote", peers[0])
	t.Logf("Sending request to %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var reply RequestVoteReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		t.Fatalf("Failed to decode reply: %v", err)
	}

	t.Logf("Received reply: %+v", reply)
}
