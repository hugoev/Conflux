package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestOperatorDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()

	// Create RaftCluster
	f.CreateRaftCluster(3)

	// Verify pods ready
	f.WaitForPodsReady(3)

	// Verify StatefulSet exists and has correct replicas
	ctx := context.Background()
	sts := &appsv1.StatefulSet{}
	key := types.NamespacedName{
		Name:      "raftcluster-sample",
		Namespace:  "default",
	}

	if err := f.KubeClient.RESTClient().Get().
		Namespace(key.Namespace).
		Resource("statefulsets").
		Name(key.Name).
		Do(ctx).
		Into(sts); err != nil {
		t.Fatalf("Failed to get StatefulSet: %v", err)
	}

	if *sts.Spec.Replicas != 3 {
		t.Errorf("StatefulSet replicas: got %d, want 3", *sts.Spec.Replicas)
	}

	// Verify Service exists
	svc := &corev1.Service{}
	svcKey := types.NamespacedName{
		Name:      "raftcluster-sample",
		Namespace: "default",
	}

	if err := f.KubeClient.RESTClient().Get().
		Namespace(svcKey.Namespace).
		Resource("services").
		Name(svcKey.Name).
		Do(ctx).
		Into(svc); err != nil {
		t.Fatalf("Failed to get Service: %v", err)
	}

	if svc.Spec.ClusterIP != "None" {
		t.Errorf("Service should be headless (ClusterIP=None), got %s", svc.Spec.ClusterIP)
	}
}

func TestRaftClusterCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()

	// Create RaftCluster
	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)

	// Update RaftCluster (scale up)
	t.Log("Scaling up to 5 replicas...")
	patch := `{"spec":{"replicas":5}}`
	if err := f.runCommand("kubectl", "patch", "raftcluster", "raftcluster-sample",
		"--type=merge", "-p", patch); err != nil {
		t.Fatalf("Failed to patch replicas: %v", err)
	}

	f.WaitForPodsReady(5)

	// Update RaftCluster (scale down)
	t.Log("Scaling down to 3 replicas...")
	patch = `{"spec":{"replicas":3}}`
	if err := f.runCommand("kubectl", "patch", "raftcluster", "raftcluster-sample",
		"--type=merge", "-p", patch); err != nil {
		t.Fatalf("Failed to patch replicas: %v", err)
	}

	f.WaitForPodsReady(3)

	// Delete RaftCluster
	t.Log("Deleting RaftCluster...")
	if err := f.runCommand("kubectl", "delete", "raftcluster", "raftcluster-sample",
		"--wait=true", "--timeout=120s"); err != nil {
		t.Fatalf("Failed to delete RaftCluster: %v", err)
	}

	// Verify StatefulSet is deleted
	ctx := context.Background()
	sts := &appsv1.StatefulSet{}
	key := types.NamespacedName{
		Name:      "raftcluster-sample",
		Namespace: "default",
	}

	// Wait for deletion
	time.Sleep(2 * time.Second)

	err := f.KubeClient.RESTClient().Get().
		Namespace(key.Namespace).
		Resource("statefulsets").
		Name(key.Name).
		Do(ctx).
		Into(sts)

	if err == nil {
		t.Error("StatefulSet should be deleted but still exists")
	}
}

func TestRaftClusterStatusUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()

	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)

	// Check RaftCluster status using kubectl
	maxWait := 30 * time.Second
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	statusUpdated := false
	for time.Now().Before(deadline) {
		<-ticker.C

		// Use kubectl to get status
		cmd := exec.Command("kubectl", "get", "raftcluster", "raftcluster-sample",
			"-o", "jsonpath={.status.readyReplicas}")
		output, err := cmd.Output()
		if err != nil {
			t.Logf("Failed to get RaftCluster status (retrying): %v", err)
			continue
		}

		var readyReplicas int32
		if _, err := fmt.Sscanf(string(output), "%d", &readyReplicas); err == nil {
			if readyReplicas == 3 {
				statusUpdated = true
				t.Logf("Status updated: ReadyReplicas=%d", readyReplicas)
				break
			}
		}
	}

	if !statusUpdated {
		t.Errorf("Status not updated within %v", maxWait)
	}
}

func TestRaftClusterErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()

	// Create RaftCluster with invalid image (should handle gracefully)
	t.Log("Creating RaftCluster with invalid image...")
	invalidYAML := `
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: raftcluster-invalid
spec:
  replicas: 1
  image: "invalid-image:does-not-exist"
`

	// Write to temp file and apply
	if err := f.runCommand("kubectl", "apply", "-f", "-", fmt.Sprintf("<<EOF\n%s\nEOF", invalidYAML)); err != nil {
		// This might fail, which is expected
		t.Logf("Expected error creating invalid cluster: %v", err)
	}

	// Create valid cluster
	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)

	// Delete a pod to test recovery
	t.Log("Deleting a pod to test recovery...")
	if err := f.runCommand("kubectl", "delete", "pod", "raftcluster-sample-0",
		"--wait=false"); err != nil {
		t.Logf("Failed to delete pod (may not exist): %v", err)
	}

	// Wait for pod to be recreated
	time.Sleep(5 * time.Second)

	// Verify pod is recreated
	ctx := context.Background()
	podList := &corev1.PodList{}
	if err := f.KubeClient.RESTClient().Get().
		Namespace("default").
		Resource("pods").
		Param("labelSelector", "app=raft,raftcluster=raftcluster-sample").
		Do(ctx).
		Into(podList); err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(podList.Items) != 3 {
		t.Errorf("Expected 3 pods after deletion, got %d", len(podList.Items))
	}
}

func TestRaftClusterMultipleInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	f := NewFramework(t)
	defer f.Teardown()

	f.Setup()

	// Create first cluster
	f.CreateRaftCluster(3)
	f.WaitForPodsReady(3)

	// Create second cluster
	t.Log("Creating second RaftCluster...")
	secondYAML := `
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: raftcluster-sample-2
spec:
  replicas: 3
  image: "raft-operator:e2e"
`

	if err := f.runCommand("kubectl", "apply", "-f", "-", fmt.Sprintf("<<EOF\n%s\nEOF", secondYAML)); err != nil {
		t.Fatalf("Failed to create second cluster: %v", err)
	}

	// Wait for second cluster
	time.Sleep(10 * time.Second)

	// Verify both clusters exist
	ctx := context.Background()
	stsList := &appsv1.StatefulSetList{}
	if err := f.KubeClient.RESTClient().Get().
		Namespace("default").
		Resource("statefulsets").
		Do(ctx).
		Into(stsList); err != nil {
		t.Fatalf("Failed to list StatefulSets: %v", err)
	}

	clusterCount := 0
	for _, sts := range stsList.Items {
		if sts.Name == "raftcluster-sample" || sts.Name == "raftcluster-sample-2" {
			clusterCount++
		}
	}

	if clusterCount != 2 {
		t.Errorf("Expected 2 clusters, found %d", clusterCount)
	}

	// Clean up second cluster
	if err := f.runCommand("kubectl", "delete", "raftcluster", "raftcluster-sample-2",
		"--wait=true", "--timeout=60s"); err != nil {
		t.Logf("Failed to delete second cluster: %v", err)
	}
}
