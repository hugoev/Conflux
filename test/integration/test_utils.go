package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugovillarreal/conflux/pkg/kv"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	ClusterSize      int
	ElectionTimeout  time.Duration
	ReplicationDelay time.Duration
	TestTimeout      time.Duration
}

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		ClusterSize:      3,
		ElectionTimeout:  15 * time.Second,
		ReplicationDelay: 1 * time.Second,
		TestTimeout:      30 * time.Second,
	}
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(t *testing.T, maxRetries int, initialDelay time.Duration, fn func() error) error {
	t.Helper()

	var lastErr error
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
			if i < maxRetries-1 {
				t.Logf("Retry %d/%d failed, waiting %v: %v", i+1, maxRetries, delay, err)
				time.Sleep(delay)
				delay *= 2 // Exponential backoff
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// WaitForCondition waits for a condition to become true
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, description string) {
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

	t.Fatalf("Condition not met within %v: %s", timeout, description)
}

// VerifyConsistency verifies that all stores have the same keys and values
func VerifyConsistency(t *testing.T, cluster *TestCluster, expectedKeys map[string]string) {
	t.Helper()

	for key, expectedValue := range expectedKeys {
		for nodeIdx, store := range cluster.Stores {
			value, exists := store.Get(key)
			if !exists {
				t.Errorf("Node %d missing key %s", nodeIdx, key)
				continue
			}
			if value != expectedValue {
				t.Errorf("Node %d key %s: got %s, want %s", nodeIdx, key, value, expectedValue)
			}
		}
	}
}

// WriteData writes data to the leader and waits for replication
func WriteData(t *testing.T, cluster *TestCluster, key, value string) error {
	t.Helper()

	leaderIdx := cluster.GetLeaderIndex()
	if leaderIdx < 0 {
		return fmt.Errorf("no leader available")
	}

	leader := cluster.Nodes[leaderIdx]
	store := cluster.Stores[leaderIdx]

	if err := store.Put(key, value); err != nil {
		return fmt.Errorf("failed to put key: %w", err)
	}

	cmd := &kv.Command{
		Type:  kv.CommandPut,
		Key:   key,
		Value: value,
	}
	if err := leader.Propose(cmd); err != nil {
		return fmt.Errorf("failed to propose: %w", err)
	}

	// Wait for replication
	time.Sleep(500 * time.Millisecond)

	return nil
}

// ReadData reads data from all nodes and verifies consistency
func ReadData(t *testing.T, cluster *TestCluster, key, expectedValue string) {
	t.Helper()

	for nodeIdx, store := range cluster.Stores {
		value, exists := store.Get(key)
		if !exists {
			t.Errorf("Node %d missing key %s", nodeIdx, key)
			continue
		}
		if value != expectedValue {
			t.Errorf("Node %d key %s: got %s, want %s", nodeIdx, key, value, expectedValue)
		}
	}
}

// MeasureLatency measures the latency of an operation
func MeasureLatency(t *testing.T, operation func() error) (time.Duration, error) {
	t.Helper()

	start := time.Now()
	err := operation()
	latency := time.Since(start)

	if err != nil {
		return latency, err
	}

	t.Logf("Operation latency: %v", latency)
	return latency, nil
}

// BenchmarkThroughput benchmarks the throughput of operations
func BenchmarkThroughput(t *testing.T, numOps int, operation func(int) error) (float64, error) {
	t.Helper()

	start := time.Now()

	for i := 0; i < numOps; i++ {
		if err := operation(i); err != nil {
			return 0, fmt.Errorf("operation %d failed: %w", i, err)
		}
	}

	elapsed := time.Since(start)
	throughput := float64(numOps) / elapsed.Seconds()

	t.Logf("Throughput: %.2f ops/sec (%d operations in %v)", throughput, numOps, elapsed)
	return throughput, nil
}
