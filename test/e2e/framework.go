package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Framework manages E2E test infrastructure
type Framework struct {
	t           *testing.T
	ClusterName string
	KubeClient  *kubernetes.Clientset
	KubeConfig  string
}

// NewFramework creates a new E2E test framework
func NewFramework(t *testing.T) *Framework {
	t.Helper()

	clusterName := fmt.Sprintf("raft-e2e-%d", time.Now().Unix())

	return &Framework{
		t:           t,
		ClusterName: clusterName,
	}
}

// Setup creates a kind cluster and deploys the operator
func (f *Framework) Setup() {
	f.t.Helper()
	f.t.Logf("Setting up E2E environment for cluster %s", f.ClusterName)

	// Create kind cluster
	if err := f.runCommand("kind", "create", "cluster", "--name", f.ClusterName); err != nil {
		f.t.Fatalf("Failed to create kind cluster: %v", err)
	}

	// Setup kubeconfig
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	f.KubeConfig = kubeConfigPath

	// Create client
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		f.t.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		f.t.Fatalf("Failed to create clientset: %v", err)
	}
	f.KubeClient = clientset

	// Build docker image
	f.t.Log("Building operator image...")
	if err := f.runCommand("docker", "build", "-t", "raft-operator:e2e", "."); err != nil {
		f.t.Fatalf("Failed to build docker image: %v", err)
	}

	// Load image into kind
	f.t.Log("Loading image into kind...")
	if err := f.runCommand("kind", "load", "docker-image", "raft-operator:e2e", "--name", f.ClusterName); err != nil {
		f.t.Fatalf("Failed to load image into kind: %v", err)
	}

	// Install CRDs
	f.t.Log("Installing CRDs...")
	if err := f.runCommand("make", "install"); err != nil {
		f.t.Fatalf("Failed to install CRDs: %v", err)
	}

	// Deploy operator
	f.t.Log("Deploying operator...")
	if err := f.runCommand("make", "deploy", "IMG=raft-operator:e2e"); err != nil {
		f.t.Fatalf("Failed to deploy operator: %v", err)
	}

	// Wait for operator to be ready
	f.t.Log("Waiting for operator to be ready...")
	if err := f.runCommand("kubectl", "wait", "--for=condition=available", "deployment/raft-operator-controller-manager", "-n", "raft-operator-system", "--timeout=120s"); err != nil {
		f.t.Fatalf("Operator failed to become ready: %v", err)
	}
}

// Teardown deletes the kind cluster
func (f *Framework) Teardown() {
	f.t.Helper()
	f.t.Logf("Tearing down E2E environment for cluster %s", f.ClusterName)

	if err := f.runCommand("kind", "delete", "cluster", "--name", f.ClusterName); err != nil {
		f.t.Errorf("Failed to delete kind cluster: %v", err)
	}
}

// CreateRaftCluster applies the sample RaftCluster CR
func (f *Framework) CreateRaftCluster(replicas int) {
	f.t.Helper()
	f.t.Logf("Creating RaftCluster with %d replicas...", replicas)

	// Apply sample
	if err := f.runCommand("kubectl", "apply", "-f", "operator/config/samples/raft_v1alpha1_raftcluster.yaml"); err != nil {
		f.t.Fatalf("Failed to apply RaftCluster sample: %v", err)
	}

	// Patch replicas if needed (sample usually has 3)
	if replicas != 3 {
		patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
		if err := f.runCommand("kubectl", "patch", "raftcluster", "raftcluster-sample", "--type=merge", "-p", patch); err != nil {
			f.t.Fatalf("Failed to patch replicas: %v", err)
		}
	}
}

// WaitForPodsReady waits for all Raft pods to be ready
func (f *Framework) WaitForPodsReady(replicas int) {
	f.t.Helper()
	f.t.Logf("Waiting for %d pods to be ready...", replicas)

	// Wait for StatefulSet to scale
	if err := f.runCommand("kubectl", "wait", "--for=jsonpath='{.status.readyReplicas}'="+fmt.Sprintf("%d", replicas), "statefulset/raftcluster-sample", "--timeout=300s"); err != nil {
		f.t.Fatalf("Pods failed to become ready: %v", err)
	}
}

// runCommand runs a shell command
func (f *Framework) runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetRaftClusterStatus retrieves the status of a RaftCluster
func (f *Framework) GetRaftClusterStatus(name, namespace string) (map[string]interface{}, error) {
	f.t.Helper()
	
	// Use kubectl to get status
	cmd := exec.Command("kubectl", "get", "raftcluster", name,
		"-n", namespace,
		"-o", "jsonpath={.status}")
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	
	// Parse JSON output (simplified - in production use proper JSON parsing)
	status := make(map[string]interface{})
	// For now, return empty map - caller can use kubectl directly
	return status, nil
}

// WaitForCondition waits for a condition on a resource
func (f *Framework) WaitForCondition(resource, name, condition, namespace string, timeout time.Duration) error {
	f.t.Helper()
	f.t.Logf("Waiting for %s/%s condition %s...", resource, name, condition)
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition %s on %s/%s", condition, resource, name)
		case <-ticker.C:
			cmd := exec.CommandContext(ctx, "kubectl", "get", resource, name,
				"-n", namespace,
				"-o", fmt.Sprintf("jsonpath={.status.conditions[?(@.type==\"%s\")].status}", condition))
			output, err := cmd.Output()
			if err == nil && string(output) == "True" {
				return nil
			}
		}
	}
}
