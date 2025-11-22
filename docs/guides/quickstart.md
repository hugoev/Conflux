# Quick Start Guide

## Introduction

This guide will help you get Conflux up and running in under 5 minutes.

## Prerequisites

- **Go** 1.21+ (for local development)
- **Docker** (for containerized deployment)
- **kubectl** and **kind** (for Kubernetes deployment)

## Option 1: Local Single Node (Fastest)

Perfect for development and testing.

```bash
# Clone the repository
git clone https://github.com/hugoev/Conflux.git
cd Conflux

# Build the binary
go build -o raftnode cmd/raftnode/main.go

# Run a single node
./raftnode \
  --node-id=node-0 \
  --port=8080 \
  --data-dir=./data

# In another terminal, test it
curl -X PUT http://localhost:8080/kv/hello \
  -H "Content-Type: application/json" \
  -d '{"value":"world"}'

curl http://localhost:8080/kv/hello
# Output: {"value":"world"}
```

## Option 2: Docker Compose (Recommended)

Best for local multi-node testing with observability.

```bash
# Start 3-node cluster with Prometheus and Grafana
docker-compose up -d

# Wait for cluster to stabilize (~30 seconds)
sleep 30

# Check cluster health
curl http://localhost:8080/health | jq '.'

# Write some data
curl -X PUT http://localhost:8080/kv/user:1 \
  -H "Content-Type: application/json" \
  -d '{"value":"Alice"}'

# Read from any node
curl http://localhost:8081/kv/user:1 | jq '.'

# Access Grafana dashboard
open http://localhost:3000
# Login: admin/admin
```

## Option 3: Kubernetes (Production-Like)

Full production setup with operator.

```bash
# Create kind cluster
kind create cluster --name raft-demo

# Load Docker image into kind
docker build -t hugo/raft-node:latest .
kind load docker-image hugo/raft-node:latest --name raft-demo

# Deploy the operator
cd operator
make deploy

# Wait for operator to be ready
kubectl wait --for=condition=available deployment/raft-operator-controller-manager \
  -n raft-operator-system --timeout=120s

# Create a RaftCluster
kubectl apply -f config/samples/raft_v1alpha1_raftcluster.yaml

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=raft --timeout=180s

# Check cluster status
kubectl get raftclusters
kubectl get pods -l app=raft

# Port-forward to access the cluster
kubectl port-forward raft-sample-0 8080:8080

# In another terminal, test it
curl -X PUT http://localhost:8080/kv/test \
  -H "Content-Type: application/json" \
  -d '{"value":"kubernetes!"}'

curl http://localhost:8080/kv/test
```

## Basic Operations

### Writing Data

```bash
curl -X PUT http://localhost:8080/kv/mykey \
  -H "Content-Type: application/json" \
  -d '{"value":"myvalue"}'
```

### Reading Data

```bash
curl http://localhost:8080/kv/mykey
```

### Deleting Data

```bash
curl -X DELETE http://localhost:8080/kv/mykey
```

### Checking Health

```bash
curl http://localhost:8080/health | jq '.'
```

## Monitoring

### Prometheus Metrics

```bash
# View raw metrics
curl http://localhost:8080/metrics

# Key metrics to watch
curl http://localhost:8080/metrics | grep raft_state
curl http://localhost:8080/metrics | grep raft_term
curl http://localhost:8080/metrics | grep raft_commit_index
```

### Grafana Dashboard

When using docker-compose:

1. Open http://localhost:3000
2. Login with `admin/admin`
3. Navigate to Dashboards → Raft Cluster
4. View real-time metrics

## Common Tasks

### Finding the Leader

```bash
# Check health on all nodes
for port in 8080 8081 8082; do
  echo "Port $port:"
  curl -s http://localhost:$port/health | jq '.state'
done
```

### Simulating Leader Failure

```bash
# Using docker-compose
docker-compose stop raft-node-0

# Watch new leader election
docker-compose logs -f raft-node-1 raft-node-2 | grep "Became leader"

# Restart the node
docker-compose start raft-node-0
```

### Viewing Logs

```bash
# Docker Compose
docker-compose logs -f raft-node-0

# Kubernetes
kubectl logs -f raft-sample-0

# Follow all pods
kubectl logs -f -l app=raft
```

## Troubleshooting

### No Leader Elected

**Symptom**: All nodes show `state: "candidate"` or `state: "follower"`

**Solution**:
```bash
# Check logs for election activity
docker-compose logs | grep -E "(Starting election|Won election)"

# Restart all nodes
docker-compose restart
```

### Connection Refused

**Symptom**: `curl: (7) Failed to connect`

**Solution**:
```bash
# Check if service is running
docker-compose ps

# Check if port is correct
curl http://localhost:8080/health
```

### Data Not Persisting

**Symptom**: Data lost after restart

**Solution**:
```bash
# Check if volumes are mounted (docker-compose)
docker-compose exec raft-node-0 ls -la /data

# For Kubernetes, check PVCs
kubectl get pvc
```

## Next Steps

- **Learn More**: Read the [Architecture Documentation](../architecture/README.md)
- **Deploy to Production**: See [Deployment Guide](deployment.md)
- **Monitor Your Cluster**: Check [Monitoring Guide](monitoring.md)
- **Troubleshoot Issues**: Consult [Operations Runbook](../operations/runbook.md)

## Cleanup

### Docker Compose

```bash
docker-compose down -v
```

### Kubernetes

```bash
kubectl delete raftcluster raft-sample
kind delete cluster --name raft-demo
```

## Getting Help

- **Documentation**: [docs/](../)
- **Issues**: [GitHub Issues](https://github.com/hugoev/Conflux/issues)
- **API Reference**: [HTTP API](../api/http-api.md)
