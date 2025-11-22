package e2e

import (
	"testing"
)

func TestOperatorDeployment(t *testing.T) {
	// Skip if short mode
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	// Setup environment
	f.Setup()

	// Create RaftCluster
	f.CreateRaftCluster(3)

	// Verify pods ready
	f.WaitForPodsReady(3)

	// Additional verification can be added here
	// e.g., check logs, port-forward and check API, etc.
}
