# Operations Guide

## Local Development

### Prerequisites

- Go 1.21+
- Docker
- kubectl
- kind or k3d

### Running Single Node Locally

```bash
# Build and run
go run cmd/raftnode/main.go --node-id=node-0 --port=8080

# Or use Make
make run-local
```

### Testing the API

```bash
# Put a value
curl -X PUT http://localhost:8080/kv \
  -H "Content-Type: application/json" \
  -d '{"key": "foo", "value": "bar"}'

# Get a value
curl http://localhost:8080/kv/foo

# Delete a value
curl -X DELETE http://localhost:8080/kv/foo

# Health check
curl http://localhost:8080/healthz

# Metrics
curl http://localhost:8080/metrics
```

## Kubernetes Deployment

### Setup Local Cluster

```bash
# Create kind cluster
kind create cluster --name conflux

# Or use k3d
k3d cluster create conflux
```

### Install CRD

```bash
kubectl apply -f deploy/crd/raftcluster.yaml
```

### Build and Push Images

```bash
# Build Raft node image
docker build -t hugo/raft-node:latest -f Dockerfile .

# Load into kind
kind load docker-image hugo/raft-node:latest --name conflux

# Or push to registry
docker push hugo/raft-node:latest
```

### Install Operator

```bash
# Install RBAC
kubectl apply -f deploy/operator/rbac.yaml

# Install Operator
kubectl apply -f deploy/operator/deployment.yaml
```

### Create RaftCluster

```bash
# Create cluster
kubectl apply -f deploy/samples/raftcluster.yaml

# Check status
kubectl get raftclusters
kubectl describe raftcluster example-cluster

# Check pods
kubectl get pods -l app=raft-node

# Check logs
kubectl logs -l app=raft-node -f
```

## Observability

### Install Prometheus

```bash
kubectl apply -f deploy/observability/prometheus.yaml
```

Access Prometheus UI:
```bash
kubectl port-forward svc/prometheus 9090:9090
# Open http://localhost:9090
```

### Install Grafana

```bash
kubectl apply -f deploy/observability/grafana.yaml
```

Access Grafana UI:
```bash
kubectl port-forward svc/grafana 3000:3000
# Open http://localhost:3000
# Login: admin/admin
```

### Key Metrics to Monitor

- `kv_requests_total`: Total requests by method and status
- `kv_request_duration_seconds`: Request latency
- `raft_is_leader`: Current leader (should be 1)
- `raft_term`: Current Raft term
- `raft_commit_index`: Log commit progress
- `raft_election_timeouts_total`: Election events

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name>

# Check logs
kubectl logs <pod-name>

# Check events
kubectl get events --sort-by='.lastTimestamp'
```

### Operator Not Reconciling

```bash
# Check operator logs
kubectl logs -l app=raft-operator

# Check operator status
kubectl get deployment raft-operator
```

### Cluster Not Forming

```bash
# Check peer configuration
kubectl exec <pod-name> -- env | grep PEERS

# Check DNS resolution
kubectl exec <pod-name> -- nslookup example-cluster-0.example-cluster

# Check Raft logs
kubectl logs <pod-name> | grep -i raft
```

## Backup and Recovery

### Manual Snapshot

```bash
# Trigger snapshot (future feature)
kubectl exec <pod-name> -- curl -X POST http://localhost:8080/snapshot
```

### Restore from Snapshot

```bash
# Copy snapshot to pod
kubectl cp snapshot.bin <pod-name>:/data/snapshot.bin

# Restart pod to load snapshot
kubectl delete pod <pod-name>
```

## Performance Tuning

### Resource Limits

Adjust in RaftCluster spec:
```yaml
spec:
  resources:
    cpu: "500m"
    memory: "512Mi"
```

### Raft Timeouts

Adjust via environment variables:
```yaml
env:
- name: ELECTION_TIMEOUT_MIN
  value: "200ms"
- name: ELECTION_TIMEOUT_MAX
  value: "400ms"
```

## Production Checklist

- [ ] Set appropriate resource limits
- [ ] Configure persistent storage
- [ ] Enable Prometheus monitoring
- [ ] Set up Grafana dashboards
- [ ] Configure log aggregation
- [ ] Set up alerting rules
- [ ] Test failover scenarios
- [ ] Document runbooks
- [ ] Set up backup strategy



