# Kubernetes Operator Design

## Overview

The RaftCluster Operator manages the lifecycle of Raft-based key-value store clusters in Kubernetes.

## Custom Resource Definition

### RaftCluster Spec

```yaml
spec:
  replicas: 3                    # Desired number of nodes
  image: hugo/raft-node:latest    # Container image
  resources:                      # Resource limits
    cpu: "250m"
    memory: "256Mi"
  storage:                        # Storage configuration
    size: "1Gi"
  service:                        # Service configuration
    type: ClusterIP
    port: 8080
```

### RaftCluster Status

```yaml
status:
  phase: Healthy                  # Current phase
  readyReplicas: 3                # Number of ready pods
  leader: raft-node-0             # Current leader node ID
  conditions:                     # Conditions array
    - type: Ready
      status: "True"
      reason: AllReplicasReady
```

## Controller Architecture

### Reconcile Loop

The controller implements a reconcile loop that:

1. **Fetches** the RaftCluster object
2. **Computes** desired child resources:
   - StatefulSet (for pod management)
   - Service (for client access and peer discovery)
   - ConfigMap (for peer configuration)
   - PersistentVolumeClaims (for WAL/snapshots)
3. **Compares** actual vs desired state
4. **Creates/Updates/Deletes** resources as needed
5. **Updates** status with current state

### StatefulSet Management

The operator creates a StatefulSet to manage Raft nodes:

**Benefits:**
- Stable network identities (DNS names)
- Ordered deployment and scaling
- Persistent storage per pod

**Configuration:**
- Pod template with Raft node container
- Environment variables for node ID and peer list
- Volume mounts for WAL and snapshots

### Service Management

Two types of services:

1. **Headless Service** (for peer discovery):
   - `clusterIP: None`
   - Creates DNS entries for each pod
   - Used for Raft RPC communication

2. **ClusterIP Service** (for client access):
   - Load balances client requests
   - Exposes HTTP API port

### Peer Discovery

Peers are discovered via Kubernetes DNS:

```
{cluster-name}-{index}.{cluster-name}.{namespace}.svc.cluster.local:9090
```

Example for 3-node cluster:
- `example-cluster-0.example-cluster.default.svc.cluster.local:9090`
- `example-cluster-1.example-cluster.default.svc.cluster.local:9090`
- `example-cluster-2.example-cluster.default.svc.cluster.local:9090`

## Scaling Operations

### Scale Up

1. Update `spec.replicas` in RaftCluster
2. Operator detects change
3. Updates StatefulSet replica count
4. Kubernetes creates new pods
5. New pods join cluster via Raft membership changes (future)

### Scale Down

1. Update `spec.replicas` in RaftCluster
2. Operator detects change
3. Updates StatefulSet replica count
4. Kubernetes terminates pods (highest index first)
5. Remaining nodes handle reduced quorum

## Health Monitoring

### Status Updates

The operator periodically:

1. Queries StatefulSet for ready replicas
2. Checks pod health via `/healthz` endpoint
3. Queries leader via `/metrics` endpoint
4. Updates `status.readyReplicas` and `status.leader`

### Conditions

- **Ready**: All desired replicas are ready
- **Healthy**: Cluster is operational
- **Degraded**: Some replicas are not ready

## Rolling Updates

### Image Updates

1. Update `spec.image` in RaftCluster
2. Operator detects change
3. Updates StatefulSet pod template
4. Kubernetes performs rolling update (one pod at a time)
5. Each pod restarts with new image

### Configuration Updates

1. Update RaftCluster spec
2. Operator detects change
3. Updates StatefulSet or ConfigMap
4. Pods restart if needed (or reload config)

## RBAC Requirements

The operator needs permissions for:

- **RaftClusters**: get, list, watch, update, patch
- **StatefulSets**: get, list, watch, create, update, patch, delete
- **Services**: get, list, watch, create, update, patch, delete
- **Pods**: get, list, watch (for health checks)
- **ConfigMaps**: get, list, watch, create, update, patch, delete
- **PVCs**: get, list, watch, create, update, patch, delete

## Future Enhancements

- **Graceful Shutdown**: Drain nodes before termination
- **Backup/Restore**: Snapshot management
- **Multi-Cluster**: Cross-cluster replication
- **Auto-Scaling**: HPA/VPA integration
- **Chaos Testing**: Automated failure injection



