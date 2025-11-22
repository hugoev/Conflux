# Conflux - Distributed Raft KV Store

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Build Status](https://github.com/hugovillarreal/conflux/actions/workflows/test.yml/badge.svg)](https://github.com/hugovillarreal/conflux/actions/workflows/test.yml)
[![E2E Tests](https://github.com/hugovillarreal/conflux/actions/workflows/e2e.yml/badge.svg)](https://github.com/hugovillarreal/conflux/actions/workflows/e2e.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hugovillarreal/conflux)](https://goreportcard.com/report/github.com/hugovillarreal/conflux)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Raft consensus implementation and Kubernetes Operator in Go.

## ✨ Features

- **🔐 Strong Consistency** - Raft consensus ensures linearizable reads and writes
- **🚀 High Availability** - Automatic leader election and failure recovery
- **💾 Durable Storage** - Write-Ahead Log (WAL) and snapshotting for persistence
- **☸️ Kubernetes Native** - Custom operator for declarative cluster management
- **📊 Full Observability** - Prometheus metrics, Grafana dashboards, structured logging
- **🔄 Automatic Scaling** - Dynamic cluster resizing via Kubernetes StatefulSets
- **🛡️ Production Ready** - Comprehensive testing (70%+ coverage), panic recovery, health checks
- **🧪 Comprehensive Testing** - Unit, Integration, and E2E test suites


## 🚀 Quick Start

### Prerequisites

- **Go** 1.21 or later
- **Docker** (for containerized deployment)
- **kubectl** and **kind** (for Kubernetes deployment)

### Local Development (Single Node)

```bash
# Clone the repository
git clone https://github.com/hugovillarreal/conflux.git
cd conflux

# Run a single node
go run cmd/raftnode/main.go \
  --node-id=node-0 \
  --port=8080 \
  --data-dir=./data

# Test the API
curl -X PUT http://localhost:8080/kv/hello \
  -H "Content-Type: application/json" \
  -d '{"value":"world"}'

curl http://localhost:8080/kv/hello
# Output: {"value":"world"}
```

### Docker Compose (3-Node Cluster)

```bash
# Start 3-node cluster with observability
docker-compose up -d

# Check cluster health
curl http://localhost:8080/health

# Write to leader
curl -X PUT http://localhost:8080/kv/test \
  -H "Content-Type: application/json" \
  -d '{"value":"distributed!"}'

# Read from any node
curl http://localhost:8081/kv/test
```

### Kubernetes Deployment

```bash
# Create kind cluster
kind create cluster --name raft-test

# Deploy operator
cd operator
make deploy

# Create RaftCluster
kubectl apply -f config/samples/raft_v1alpha1_raftcluster.yaml

# Verify cluster
kubectl get raftclusters
kubectl get pods -l app=raft

# Access the cluster
kubectl port-forward raft-sample-0 8080:8080
curl http://localhost:8080/kv/test
```

## 📚 Documentation

- **[API Reference](docs/api/http-api.md)** - HTTP API endpoints and examples
- **[Architecture](docs/architecture/README.md)** - System design and Raft implementation
- **[User Guide](docs/guides/quickstart.md)** - Deployment and configuration
- **[Operator Guide](docs/operator/README.md)** - Kubernetes operator usage
- **[Operations Runbook](docs/operations/runbook.md)** - Troubleshooting and maintenance
- **[Development Guide](docs/development/setup.md)** - Contributing and local setup

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Client Applications                   │
└────────────────────┬────────────────────────────────────┘
                     │ HTTP/gRPC
         ┌───────────┴───────────┐
         │                       │
    ┌────▼────┐            ┌────▼────┐            ┌─────────┐
    │ Node 0  │◄──Raft────►│ Node 1  │◄──Raft────►│ Node 2  │
    │ Leader  │            │Follower │            │Follower │
    └────┬────┘            └────┬────┘            └────┬────┘
         │                      │                      │
    ┌────▼────┐            ┌────▼────┐            ┌────▼────┐
    │   WAL   │            │   WAL   │            │   WAL   │
    │Snapshot │            │Snapshot │            │Snapshot │
    └─────────┘            └─────────┘            └─────────┘
```

### Components

- **Raft Consensus** - Leader election, log replication, safety guarantees
- **KV State Machine** - In-memory key-value store with atomic operations
- **Persistence Layer** - WAL for durability, snapshots for compaction
- **HTTP API** - RESTful interface for client operations
- **Kubernetes Operator** - Automated cluster lifecycle management
- **Observability** - Prometheus metrics, Grafana dashboards, structured logs

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./pkg/raft -v

# Run benchmarks
go test ./pkg/raft -bench=. -benchmem
```

**Test Coverage:**
- `pkg/config`: 94.7%
- `pkg/kv`: 65.2%
- `pkg/raft`: 57.0%
- **Overall**: ~70%

## 📊 Monitoring

Access Grafana dashboard (when using docker-compose):
```bash
open http://localhost:3000
# Default credentials: admin/admin
```

**Available Metrics:**
- Raft state (leader/follower/candidate)
- Election timeouts and leader changes
- Log replication lag
- Commit index progression
- RPC latencies
- KV operation throughput

## 🛠️ Development

```bash
# Install dependencies
go mod download

# Run linter
make lint

# Build binary
make build

# Build Docker image
make docker-build

# Run operator locally
cd operator
make run
```

## 📁 Project Structure

```
conflux/
├── cmd/
│   └── raftnode/           # Main application binary
├── pkg/
│   ├── api/                # HTTP server and handlers
│   ├── config/             # Configuration management
│   ├── kv/                 # Key-value state machine
│   ├── metrics/            # Prometheus metrics
│   ├── raft/               # Raft consensus implementation
│   ├── snapshot/           # Snapshot management
│   └── wal/                # Write-Ahead Log
├── operator/               # Kubernetes operator
│   ├── api/v1alpha1/       # CRD definitions
│   ├── controllers/        # Reconciliation logic
│   └── config/             # Operator manifests
├── docs/                   # Documentation
├── deploy/                 # Deployment manifests
└── test/                   # Integration and E2E tests
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Raft Consensus Algorithm](https://raft.github.io/) by Diego Ongaro and John Ousterhout
- [etcd](https://etcd.io/) for Raft implementation inspiration
- [Kubernetes](https://kubernetes.io/) for operator patterns

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/hugovillarreal/conflux/issues)
- **Discussions**: [GitHub Discussions](https://github.com/hugovillarreal/conflux/discussions)
- **Documentation**: [docs/](docs/)

---

**Built with ❤️ using Go and Kubernetes**
