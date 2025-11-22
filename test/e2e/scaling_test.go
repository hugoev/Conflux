package e2e

import (
	"testing"
)

func TestScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()
	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)

	// Scale up
	f.CreateRaftCluster(5)
	f.WaitForPodsReady(5)

	// Scale down
	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)
}
