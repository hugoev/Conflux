package config

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the configuration for a Raft node
type Config struct {
	NodeID     string
	Port       int
	DataDir    string
	Peers      []string
	RaftPort   int
	EnableRaft bool
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
	// Split by comma and trim spaces
	peers := []string{}
	for _, peer := range splitAndTrim(peersEnv, ",") {
		if peer != "" {
			peers = append(peers, peer)
		}
	}
	return peers
}

func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
