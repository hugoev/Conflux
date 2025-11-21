# Architecture

## Overview

This project implements a Raft-based distributed key-value store with a Kubernetes Operator for cluster management.

## Three-Layer Architecture

### 1. Data Plane: Distributed KV Store

The core Raft-based key-value store implemented in Go.

**Components:**
- **Raft Consensus**: Leader election, log replication, majority-based commit
- **State Machine**: In-memory key-value store that applies Raft log entries
- **API Layer**: HTTP/gRPC endpoints for client operations
- **Persistence**: Write-ahead log (WAL) and snapshots for durability

**Key Features:**
- Linearizable writes (all writes go through Raft leader)
- Read operations can be served from followers (eventual consistency)
- Automatic leader election and failover
- Peer discovery via Kubernetes Service DNS

### 2. Control Plane: Kubernetes Operator

A Kubernetes Operator that manages `RaftCluster` custom resources.

**Responsibilities:**
- Create/update/delete StatefulSets based on desired replica count
- Manage Services for client access and peer discovery
- Inject peer configuration via environment variables
- Monitor cluster health and update status
- Handle rolling updates and scaling operations

**Reconcile Loop:**
1. Fetch RaftCluster CRD
2. Compute desired state (StatefulSet, Service, ConfigMap)
3. Compare with actual state
4. Create/update/delete resources as needed
5. Update status with ready replicas, leader, conditions

### 3. Platform Layer: Observability

Full observability stack for production-grade operations.

**Components:**
- **Prometheus**: Metrics collection and storage
- **Grafana**: Dashboards for visualization
- **Jaeger/OpenTelemetry**: Distributed tracing

**Metrics Exposed:**
- Request-level: `kv_requests_total`, `kv_request_duration_seconds`
- Raft-level: `raft_is_leader`, `raft_term`, `raft_commit_index`
- Storage: `wal_size_bytes`, `snapshot_size_bytes`

## Data Flow

### Write Operation

```
Client → HTTP API → Raft Leader → Append to Log → Replicate to Followers
                                                      ↓
                                              Majority Commit
                                                      ↓
                                              Apply to State Machine
                                                      ↓
                                              Return Success to Client
```

### Read Operation

```
Client → HTTP API → (Leader or Follower) → Read from State Machine → Return Value
```

## Cluster Topology

```
┌─────────────────────────────────────┐
│      RaftCluster CRD                │
│  spec.replicas: 3                   │
└──────────────┬──────────────────────┘
               │
               │ watched by
               ↓
┌─────────────────────────────────────┐
│   Operator (Reconcile Loop)         │
└──────────────┬──────────────────────┘
               │
               │ creates
               ↓
┌─────────────────────────────────────┐
│   StatefulSet (3 replicas)          │
│   - raft-node-0                      │
│   - raft-node-1                      │
│   - raft-node-2                      │
└──────────────┬──────────────────────┘
               │
               │ pods run
               ↓
┌─────────────────────────────────────┐
│   Raft KV Store (Go binary)          │
│   - Raft consensus                   │
│   - HTTP API (port 8080)             │
│   - Raft RPC (port 9090)             │
└─────────────────────────────────────┘
```

## Peer Discovery

Nodes discover each other via Kubernetes DNS:
- Service name: `{cluster-name}`
- Pod DNS: `{cluster-name}-{index}.{cluster-name}.{namespace}.svc.cluster.local`
- Raft port: 9090

## Persistence

- **WAL**: Write-ahead log stored on PersistentVolume
- **Snapshots**: Periodic snapshots to avoid unbounded log growth
- **Recovery**: On restart, node replays WAL and applies latest snapshot

## Future Enhancements

- Dynamic membership changes (add/remove nodes)
- Cross-cluster replication
- Service mesh integration (Linkerd/Envoy)
- Advanced chaos testing scenarios

