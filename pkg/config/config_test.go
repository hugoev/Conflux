package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Test with no arguments and no environment variables
	cfg, err := LoadConfigWithArgs([]string{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.NodeID == "" {
		t.Error("NodeID should have a default value")
	}

	if cfg.Port != 8080 {
		t.Errorf("Expected default Port to be 8080, got %d", cfg.Port)
	}

	if cfg.RaftPort != 9090 {
		t.Errorf("Expected default RaftPort to be 9090, got %d", cfg.RaftPort)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("Expected default DataDir to be './data', got '%s'", cfg.DataDir)
	}

	if cfg.EnableRaft {
		t.Error("Expected EnableRaft to be false by default")
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("NODE_ID", "test-node")
	os.Setenv("PORT", "9090")
	os.Setenv("RAFT_PORT", "9091")
	os.Setenv("DATA_DIR", "/tmp/data")
	os.Setenv("ENABLE_RAFT", "true")
	os.Setenv("PEERS", "peer1:9090,peer2:9090,peer3:9090")

	defer func() {
		os.Unsetenv("NODE_ID")
		os.Unsetenv("PORT")
		os.Unsetenv("RAFT_PORT")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("ENABLE_RAFT")
		os.Unsetenv("PEERS")
	}()

	cfg, err := LoadConfigWithArgs([]string{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.NodeID != "test-node" {
		t.Errorf("Expected NodeID to be 'test-node', got '%s'", cfg.NodeID)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected Port to be 9090, got %d", cfg.Port)
	}

	if cfg.RaftPort != 9091 {
		t.Errorf("Expected RaftPort to be 9091, got %d", cfg.RaftPort)
	}

	if cfg.DataDir != "/tmp/data" {
		t.Errorf("Expected DataDir to be '/tmp/data', got '%s'", cfg.DataDir)
	}

	if !cfg.EnableRaft {
		t.Error("Expected EnableRaft to be true")
	}

	if len(cfg.Peers) != 3 {
		t.Errorf("Expected 3 peers, got %d", len(cfg.Peers))
	}
}

func TestLoadConfig_FromFlags(t *testing.T) {
	args := []string{
		"-node-id=flag-node",
		"-port=7070",
		"-raft-port=7071",
		"-data-dir=/flag/data",
		"-enable-raft=true",
	}

	cfg, err := LoadConfigWithArgs(args)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.NodeID != "flag-node" {
		t.Errorf("Expected NodeID to be 'flag-node', got '%s'", cfg.NodeID)
	}

	if cfg.Port != 7070 {
		t.Errorf("Expected Port to be 7070, got %d", cfg.Port)
	}

	if cfg.RaftPort != 7071 {
		t.Errorf("Expected RaftPort to be 7071, got %d", cfg.RaftPort)
	}

	if cfg.DataDir != "/flag/data" {
		t.Errorf("Expected DataDir to be '/flag/data', got '%s'", cfg.DataDir)
	}

	if !cfg.EnableRaft {
		t.Error("Expected EnableRaft to be true")
	}
}

func TestParsePeers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single peer",
			input:    "peer1:9090",
			expected: []string{"peer1:9090"},
		},
		{
			name:     "multiple peers",
			input:    "peer1:9090,peer2:9090,peer3:9090",
			expected: []string{"peer1:9090", "peer2:9090", "peer3:9090"},
		},
		{
			name:     "peers with spaces",
			input:    "peer1:9090 , peer2:9090 , peer3:9090",
			expected: []string{"peer1:9090", "peer2:9090", "peer3:9090"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePeers(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d peers, got %d", len(tt.expected), len(result))
				return
			}
			for i, peer := range result {
				if peer != tt.expected[i] {
					t.Errorf("Expected peer[%d] to be '%s', got '%s'", i, tt.expected[i], peer)
				}
			}
		})
	}
}

func TestGetPeerAddress(t *testing.T) {
	cfg := &Config{
		RaftPort: 9090,
	}

	addr := cfg.GetPeerAddress("peer1")
	expected := "peer1:9090"
	if addr != expected {
		t.Errorf("Expected address to be '%s', got '%s'", expected, addr)
	}
}

