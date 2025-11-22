# Kubernetes Operator Guide

## Overview

The Conflux Operator automates the deployment and management of Raft clusters on Kubernetes using the `RaftCluster` Custom Resource Definition (CRD).

## Installation

### Prerequisites

- Kubernetes 1.20+
- kubectl configured
- Cluster admin permissions

### Install the Operator

```bash
# Clone the repository
git clone https://github.com/hugoev/Conflux.git
cd Conflux/operator

# Install CRDs
make install

# Deploy the operator
make deploy

# Verify installation
kubectl get deployment -n raft-operator-system
kubectl get crd raftclusters.raft.conflux.io
```

## RaftCluster CRD Reference

### Basic Example

```yaml
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  replicas: 3
  image: hugo/raft-node:latest
```

### Complete Specification

```yaml
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: production-cluster
  namespace: production
  labels:
    environment: production
    team: platform
spec:
  # Number of Raft nodes (must be odd: 3, 5, 7)
  replicas: 5
  
  # Container image
  image: hugo/raft-node:v1.0.0
  imagePullPolicy: IfNotPresent
  
  # Resource requests and limits
  resources:
    requests:
      memory: "512Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "500m"
  
  # Persistent storage configuration
  storage:
    enabled: true
    size: "10Gi"
    storageClassName: "fast-ssd"
  
  # Node selector for pod placement
  nodeSelector:
    workload: raft
    disk-type: ssd
  
  # Pod affinity/anti-affinity
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: raft
          topologyKey: kubernetes.io/hostname
  
  # Tolerations for tainted nodes
  tolerations:
  - key: "workload"
    operator: "Equal"
    value: "raft"
    effect: "NoSchedule"
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.replicas` | int | Yes | Number of Raft nodes (must be odd) |
| `spec.image` | string | Yes | Container image to use |
| `spec.imagePullPolicy` | string | No | Image pull policy (default: IfNotPresent) |
| `spec.resources` | object | No | Resource requests and limits |
| `spec.storage.enabled` | bool | No | Enable persistent storage (default: true) |
| `spec.storage.size` | string | No | PVC size (default: 1Gi) |
| `spec.storage.storageClassName` | string | No | Storage class name |
| `spec.nodeSelector` | map | No | Node selector labels |
| `spec.affinity` | object | No | Pod affinity rules |
| `spec.tolerations` | array | No | Pod tolerations |

## Common Operations

### Creating a Cluster

```bash
kubectl apply -f - <<EOF
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: my-cluster
spec:
  replicas: 3
  image: hugo/raft-node:latest
EOF

# Wait for cluster to be ready
kubectl wait --for=condition=ready pod -l app=raft --timeout=180s
```

### Viewing Cluster Status

```bash
# Get RaftCluster resource
kubectl get raftclusters

# Describe for detailed status
kubectl describe raftcluster my-cluster

# Check pods
kubectl get pods -l app=raft

# Check services
kubectl get svc -l app=raft
```

### Scaling a Cluster

```bash
# Scale to 5 nodes
kubectl patch raftcluster my-cluster \
  --type='merge' \
  -p '{"spec":{"replicas":5}}'

# Verify scaling
kubectl get pods -l app=raft -w
```

### Updating the Image

```bash
# Update to new version
kubectl patch raftcluster my-cluster \
  --type='merge' \
  -p '{"spec":{"image":"hugo/raft-node:v1.1.0"}}'

# Monitor rollout
kubectl rollout status statefulset/my-cluster
```

### Deleting a Cluster

```bash
# Delete the RaftCluster
kubectl delete raftcluster my-cluster

# Verify cleanup
kubectl get pods -l app=raft
kubectl get pvc -l app=raft
```

**Note**: PVCs are NOT automatically deleted. Delete manually if needed:
```bash
kubectl delete pvc -l app=raft
```

## Operator Behavior

### Reconciliation Loop

The operator continuously reconciles the desired state (RaftCluster spec) with the actual state:

1. **Create**: Creates StatefulSet, Service, and PVCs
2. **Update**: Updates StatefulSet when spec changes
3. **Scale**: Adds/removes pods when replicas change
4. **Delete**: Cleans up resources when RaftCluster is deleted

### StatefulSet Management

The operator creates a StatefulSet with:
- Headless service for DNS-based peer discovery
- PersistentVolumeClaims for each pod
- Environment variables for configuration
- Health probes (liveness and readiness)

**Example StatefulSet** (created by operator):
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-cluster
spec:
  serviceName: my-cluster
  replicas: 3
  selector:
    matchLabels:
      app: raft
  template:
    metadata:
      labels:
        app: raft
    spec:
      containers:
      - name: raft-node
        image: hugo/raft-node:latest
        env:
        - name: NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: PEERS
          value: "my-cluster-0.my-cluster:9090,my-cluster-1.my-cluster:9090,my-cluster-2.my-cluster:9090"
        - name: ENABLE_RAFT
          value: "true"
        volumeMounts:
        - name: data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
```

### DNS-Based Peer Discovery

Pods discover each other via DNS:
- Service: `<cluster-name>.<namespace>.svc.cluster.local`
- Pod: `<pod-name>.<service-name>.<namespace>.svc.cluster.local`

Example:
- `my-cluster-0.my-cluster.default.svc.cluster.local:9090`
- `my-cluster-1.my-cluster.default.svc.cluster.local:9090`
- `my-cluster-2.my-cluster.default.svc.cluster.local:9090`

## Troubleshooting

### Operator Not Starting

```bash
# Check operator logs
kubectl logs -n raft-operator-system \
  deployment/raft-operator-controller-manager

# Check CRD installation
kubectl get crd raftclusters.raft.conflux.io
```

### Cluster Not Creating

```bash
# Check RaftCluster events
kubectl describe raftcluster my-cluster

# Check operator logs
kubectl logs -n raft-operator-system \
  deployment/raft-operator-controller-manager -f
```

### Pods Not Starting

```bash
# Check pod events
kubectl describe pod my-cluster-0

# Check pod logs
kubectl logs my-cluster-0

# Check PVC status
kubectl get pvc
```

### Reconciliation Loop

If the operator keeps updating the StatefulSet:

```bash
# Check operator logs for reconciliation messages
kubectl logs -n raft-operator-system \
  deployment/raft-operator-controller-manager | grep "Reconciling"

# This was fixed in recent versions - ensure you're on latest
```

## Advanced Configuration

### Custom Service Account

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: raft-sa
---
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: my-cluster
spec:
  replicas: 3
  image: hugo/raft-node:latest
  serviceAccountName: raft-sa
```

### Init Containers

```yaml
spec:
  initContainers:
  - name: setup
    image: busybox
    command: ['sh', '-c', 'chown -R 1000:1000 /data']
    volumeMounts:
    - name: data
      mountPath: /data
```

### Environment Variables

```yaml
spec:
  env:
  - name: LOG_LEVEL
    value: "debug"
  - name: CUSTOM_CONFIG
    value: "value"
```

## Uninstalling the Operator

```bash
# Delete all RaftClusters first
kubectl delete raftclusters --all

# Uninstall operator
cd operator
make undeploy

# Remove CRDs
make uninstall
```

## See Also

- [Deployment Guide](../guides/deployment.md)
- [Operations Runbook](../operations/runbook.md)
- [API Reference](../api/http-api.md)
