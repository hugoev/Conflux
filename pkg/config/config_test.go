package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test default values
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.NodeID == "" {
		t.Error("NodeID should have a default value")
	}

	if cfg.Port == 0 {
		t.Error("Port should have a default value")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("NODE_ID", "test-node")
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("NODE_ID")
	defer os.Unsetenv("PORT")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.NodeID != "test-node" {
		t.Errorf("Expected NodeID to be 'test-node', got '%s'", cfg.NodeID)
	}
}

