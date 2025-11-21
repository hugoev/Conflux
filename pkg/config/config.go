package config

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the configuration for a Raft node
type Config struct {
	NodeID      string
	Port        int
	DataDir     string
	Peers       []string
	RaftPort    int
	EnableRaft  bool
}

// LoadConfig loads configuration from environment variables and command-line flags
func LoadConfig() (*Config, error) {
	cfg := &Config{
		DataDir:    "./data",
		Port:       8080,
		RaftPort:   9090,
		EnableRaft: false, // Start with single-node mode
	}

	flag.StringVar(&cfg.NodeID, "node-id", getEnv("NODE_ID", "node-0"), "Unique node identifier")
	flag.IntVar(&cfg.Port, "port", getEnvInt("PORT", 8080), "HTTP API port")
	flag.IntVar(&cfg.RaftPort, "raft-port", getEnvInt("RAFT_PORT", 9090), "Raft RPC port")
	flag.StringVar(&cfg.DataDir, "data-dir", getEnv("DATA_DIR", "./data"), "Data directory for WAL and snapshots")
	flag.BoolVar(&cfg.EnableRaft, "enable-raft", getEnvBool("ENABLE_RAFT", false), "Enable Raft consensus (multi-node mode)")
	flag.Parse()

	// Load peers from environment (comma-separated)
	if peersEnv := os.Getenv("PEERS"); peersEnv != "" {
		cfg.Peers = parsePeers(peersEnv)
	}

	return cfg, nil
}

// GetPeerAddress returns the address for a peer
func (c *Config) GetPeerAddress(peerID string) string {
	// Simple format: assume peers are in format host:raft-port
	// In production, this would be more sophisticated
	return fmt.Sprintf("%s:%d", peerID, c.RaftPort)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func parsePeers(peersEnv string) []string {
	if peersEnv == "" {
		return []string{}
	}
	// Simple parsing - split by comma
	// In production, this would handle more complex formats
	peers := []string{}
	// For now, treat as single peer or comma-separated list
	// This is a simplified implementation
	return peers
}

