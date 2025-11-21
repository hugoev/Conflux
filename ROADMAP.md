# 4-Week Roadmap

This document outlines the development roadmap for the Raft-based Distributed Key-Value Store.

## Week 1: Raft + KV Core ✅

**Status**: MVP v0 Complete

### Completed
- [x] In-memory KV store implementation
- [x] Command structure (PUT, DELETE)
- [x] HTTP API server (PUT, GET, DELETE)
- [x] Health check endpoint
- [x] Prometheus metrics integration
- [x] Basic Raft node structure
- [x] RPC definitions (AppendEntries, RequestVote)
- [x] Local single-node testing

### Next Steps (MVP v1)
- [ ] Implement full Raft consensus:
  - [ ] Leader election logic
  - [ ] Election timeouts and heartbeats
  - [ ] Log replication
  - [ ] Majority-based commit
- [ ] Client redirects (followers redirect to leader)
- [ ] Multi-node local testing (3 processes)

## Week 2: Kubernetes + Basic Operator

**Status**: In Progress

### Completed
- [x] Dockerfile for containerization
- [x] RaftCluster CRD definition
- [x] Operator controller structure
- [x] StatefulSet creation logic
- [x] Service creation logic
- [x] Peer discovery via DNS
- [x] RBAC manifests
- [x] Deployment manifests

### Next Steps
- [ ] Complete Raft implementation (from Week 1)
- [ ] Test containerized nodes in kind/k3d
- [ ] Operator status updates (readyReplicas, leader)
- [ ] Health checks and node replacement
- [ ] End-to-end testing: CRD → Operator → Pods → Raft cluster

## Week 3: Observability + Durability

**Status**: Planned

### Tasks
- [ ] Add comprehensive Prometheus metrics:
  - [ ] Request-level metrics (already started)
  - [ ] Raft-level metrics (term, commit index, etc.)
  - [ ] Storage metrics (WAL size, snapshot size)
- [ ] Deploy Prometheus to cluster
- [ ] Create Grafana dashboards:
  - [ ] Cluster health overview
  - [ ] Request latency (p50/p95/p99)
  - [ ] Raft metrics (leader, term, replication lag)
- [ ] Implement WAL (write-ahead log):
  - [ ] Append-only log file
  - [ ] Log entry serialization
  - [ ] Log replay on startup
- [ ] Implement snapshotting:
  - [ ] Periodic snapshots
  - [ ] Snapshot compression
  - [ ] Snapshot restore on startup
- [ ] Validate recovery:
  - [ ] Kill pod, restart, verify state persists
  - [ ] Test with multiple restarts

## Week 4: Polish + "Wow" Features

**Status**: Planned

### Tasks
- [ ] Add distributed tracing:
  - [ ] OpenTelemetry instrumentation
  - [ ] Jaeger deployment
  - [ ] Trace Raft RPCs and KV operations
- [ ] Enhanced status conditions:
  - [ ] Ready condition
  - [ ] Healthy condition
  - [ ] Degraded condition
- [ ] Chaos testing:
  - [ ] Random pod kills
  - [ ] Network partitions
  - [ ] Leader failures
  - [ ] Verify availability and consistency
- [ ] Documentation:
  - [ ] Comprehensive README
  - [ ] Architecture diagrams
  - [ ] Sample kubectl commands
  - [ ] Troubleshooting guide
- [ ] Future work section:
  - [ ] Dynamic membership changes
  - [ ] Cross-cluster replication
  - [ ] Service mesh integration (Linkerd/Envoy)
  - [ ] Advanced monitoring and alerting

## MVP Milestones

### MVP v0 ✅
Single-node KV store with HTTP API and metrics

### MVP v1 (Current Focus)
Multi-node Raft consensus with leader election and replication

### MVP v2
Containerized nodes running in Kubernetes (manual YAML)

### MVP v3
Kubernetes Operator managing RaftCluster CRDs

### MVP v4
Persistence with WAL + snapshots

### MVP v5
Full observability (Prometheus, Grafana, Jaeger)

## Success Criteria

By the end of Week 4, the system should:

1. ✅ Run a 3-node Raft cluster in Kubernetes
2. ✅ Handle client requests with linearizable consistency
3. ✅ Survive pod failures and automatically recover
4. ✅ Expose comprehensive metrics and traces
5. ✅ Persist state across restarts
6. ✅ Be manageable via Kubernetes Operator
7. ✅ Have production-grade observability

## Notes

- Focus on correctness over performance initially
- Test thoroughly at each milestone
- Document as you go
- Keep the codebase clean and maintainable

