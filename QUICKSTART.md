# Quick Start Guide

Get up and running with the Raft-based Distributed Key-Value Store in minutes.

## Prerequisites

- Go 1.21+
- Docker (for containerization)
- kubectl (for Kubernetes)
- kind or k3d (for local Kubernetes cluster)

## Option 1: Local Single Node (MVP v0)

### Step 1: Run the Node

```bash
# Clone and navigate to project
cd /Users/hugovillarreal/Documents/Conflux

# Run single node
go run cmd/raftnode/main.go --node-id=node-0 --port=8080
```

### Step 2: Test the API

In another terminal:

```bash
# Put a value
curl -X PUT http://localhost:8080/kv \
  -H "Content-Type: application/json" \
  -d '{"key": "hello", "value": "world"}'

# Get the value
curl http://localhost:8080/kv/hello

# Check health
curl http://localhost:8080/healthz

# View metrics
curl http://localhost:8080/metrics
```

## Option 2: Kubernetes Deployment

### Step 1: Create Local Cluster

```bash
# Using kind
kind create cluster --name conflux

# Or using k3d
k3d cluster create conflux
```

### Step 2: Build and Load Image

```bash
# Build the image
docker build -t hugo/raft-node:latest -f Dockerfile .

# Load into kind
kind load docker-image hugo/raft-node:latest --name conflux

# Or push to a registry (if using k3d or remote cluster)
# docker push hugo/raft-node:latest
```

### Step 3: Install CRD

```bash
kubectl apply -f deploy/crd/raftcluster.yaml
```

### Step 4: Install Operator

```bash
# Install RBAC
kubectl apply -f deploy/operator/rbac.yaml

# Build operator image (if needed)
cd operator
# ... build operator image ...

# Install operator
kubectl apply -f deploy/operator/deployment.yaml
```

### Step 5: Create RaftCluster

```bash
# Create a 3-node cluster
kubectl apply -f deploy/samples/raftcluster.yaml

# Check status
kubectl get raftclusters
kubectl get pods -l app=raft-node

# View logs
kubectl logs -l app=raft-node -f
```

### Step 6: Access the Cluster

```bash
# Port forward to a pod
kubectl port-forward <pod-name> 8080:8080

# Or create a service (already created by operator)
kubectl port-forward svc/example-cluster 8080:8080

# Test the API
curl -X PUT http://localhost:8080/kv \
  -H "Content-Type: application/json" \
  -d '{"key": "test", "value": "value"}'
```

## Option 3: Observability Stack

### Install Prometheus

```bash
kubectl apply -f deploy/observability/prometheus.yaml

# Access Prometheus
kubectl port-forward svc/prometheus 9090:9090
# Open http://localhost:9090
```

### Install Grafana

```bash
kubectl apply -f deploy/observability/grafana.yaml

# Access Grafana
kubectl port-forward svc/grafana 3000:3000
# Open http://localhost:3000
# Login: admin/admin
```

## Troubleshooting

### Node Won't Start

```bash
# Check logs
kubectl logs <pod-name>

# Check events
kubectl describe pod <pod-name>
```

### Operator Not Working

```bash
# Check operator logs
kubectl logs -l app=raft-operator

# Check CRD
kubectl get crd raftclusters.infra.hugo.dev
```

### Build Errors

```bash
# Ensure dependencies are installed
go mod tidy

# Clean and rebuild
go clean -cache
go build ./...
```

## Next Steps

1. Read the [Architecture Documentation](docs/architecture.md)
2. Review the [Operations Guide](docs/operations.md)
3. Check the [Roadmap](ROADMAP.md) for upcoming features
4. Explore the codebase starting with `cmd/raftnode/main.go`

## Development Tips

- Use `make run-local` for quick local testing
- Check `docs/` for detailed design documents
- Run tests with `go test ./...`
- Build with `go build ./cmd/raftnode/...`

## Getting Help

- Review the documentation in `docs/`
- Check the project structure in `PROJECT_SUMMARY.md`
- See `ROADMAP.md` for development status

