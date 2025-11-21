# Raft-based Distributed Key-Value Store

A production-ready Raft-based distributed key-value store in Go, managed by a Kubernetes Operator, with full observability.

## Architecture

Three main layers:

1. **Distributed KV Store (Data Plane)** - Go implementation with Raft consensus
2. **Kubernetes Operator (Control Plane)** - Manages RaftCluster CRDs
3. **Observability + Mesh (Platform Layer)** - Prometheus, Grafana, Jaeger

## Features

- ✅ Raft consensus for linearizable writes
- ✅ HTTP/gRPC API for key-value operations
- ✅ Persistent WAL + snapshots for durability
- ✅ Kubernetes Operator for cluster management
- ✅ Full observability (metrics, traces, dashboards)
- ✅ Peer discovery and automatic healing

## Quick Start

### Prerequisites

- Go 1.21+
- Docker
- kubectl
- kind or k3d (for local Kubernetes)

### Local Development (Single Node)

```bash
# Run a single node locally
go run cmd/raftnode/main.go --node-id=node-0 --port=8080
```

### Kubernetes Deployment

```bash
# Install CRD
kubectl apply -f deploy/crd/

# Install Operator
kubectl apply -f deploy/operator/

# Create a RaftCluster
kubectl apply -f deploy/samples/raftcluster.yaml
```

## Project Structure

```
.
├── cmd/raftnode/          # Main binary for Raft KV node
├── pkg/
│   ├── raft/              # Raft implementation
│   ├── kv/                # Key/value state machine
│   ├── api/               # HTTP/gRPC server
│   ├── config/            # Configuration
│   └── metrics/           # Prometheus metrics
├── operator/              # Kubernetes Operator
├── deploy/                # Deployment manifests
├── docs/                  # Documentation
└── terraform/             # Infrastructure as code
```

## Roadmap

- [x] MVP v0: Single-node KV store
- [ ] MVP v1: Multi-node Raft consensus
- [ ] MVP v2: Containerization
- [ ] MVP v3: Kubernetes Operator
- [ ] MVP v4: Persistence (WAL + snapshots)
- [ ] MVP v5: Full observability

## License

MIT

