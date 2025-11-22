# Deployment Guide

## Overview

This guide covers deploying Conflux in various environments from development to production.

## Deployment Options

### 1. Local Development

**Use Case**: Development, testing, debugging

**Setup**:
```bash
go run cmd/raftnode/main.go \
  --node-id=node-0 \
  --port=8080 \
  --data-dir=./data
```

**Pros**:
- Fast iteration
- Easy debugging
- No dependencies

**Cons**:
- Single node only
- No high availability
- Manual management

### 2. Docker Compose

**Use Case**: Local multi-node testing, demos

**Setup**:
```bash
docker-compose up -d
```

**Configuration** (`docker-compose.yml`):
```yaml
version: '3.8'
services:
  raft-node-0:
    image: hugo/raft-node:latest
    environment:
      - NODE_ID=raft-node-0
      - PEERS=raft-node-0:9090,raft-node-1:9090,raft-node-2:9090
      - ENABLE_RAFT=true
    ports:
      - "8080:8080"
    volumes:
      - ./data/node-0:/data
```

**Pros**:
- Multi-node cluster
- Observability stack included
- Easy to reset

**Cons**:
- Not production-ready
- Limited scalability
- Manual orchestration

### 3. Kubernetes (Recommended for Production)

**Use Case**: Production deployments, auto-scaling, high availability

**Prerequisites**:
- Kubernetes cluster (1.20+)
- kubectl configured
- Operator deployed

**Deployment Steps**:

#### Step 1: Deploy the Operator

```bash
cd operator

# Install CRDs
make install

# Deploy operator
make deploy

# Verify operator is running
kubectl get deployment -n raft-operator-system
```

#### Step 2: Create RaftCluster

```yaml
# raftcluster.yaml
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: production-cluster
  namespace: default
spec:
  replicas: 5  # Use odd numbers: 3, 5, or 7
  image: hugo/raft-node:v1.0.0
  imagePullPolicy: IfNotPresent
  
  resources:
    requests:
      memory: "512Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "500m"
  
  storage:
    size: "10Gi"
    storageClassName: "fast-ssd"
  
  nodeSelector:
    workload: raft
  
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: raft
          topologyKey: kubernetes.io/hostname
```

Apply:
```bash
kubectl apply -f raftcluster.yaml
```

#### Step 3: Verify Deployment

```bash
# Check RaftCluster status
kubectl get raftclusters

# Check pods
kubectl get pods -l app=raft

# Check services
kubectl get svc -l app=raft

# Check PVCs
kubectl get pvc -l app=raft
```

#### Step 4: Access the Cluster

**Via Port Forward**:
```bash
kubectl port-forward production-cluster-0 8080:8080
curl http://localhost:8080/health
```

**Via Service** (within cluster):
```bash
curl http://production-cluster-0.production-cluster:8080/health
```

**Via Ingress** (external access):
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: raft-ingress
spec:
  rules:
  - host: raft.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: production-cluster
            port:
              number: 8080
```

## Production Considerations

### High Availability

**Cluster Size**:
- **3 nodes**: Tolerates 1 failure
- **5 nodes**: Tolerates 2 failures (recommended)
- **7 nodes**: Tolerates 3 failures (large deployments)

**Anti-Affinity**:
```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchLabels:
          app: raft
      topologyKey: kubernetes.io/hostname
```

### Storage

**PersistentVolumeClaims**:
```yaml
storage:
  size: "50Gi"
  storageClassName: "fast-ssd"  # Use SSD for better performance
```

**Backup Strategy**:
- Regular snapshots of PVCs
- WAL archiving to S3/GCS
- Point-in-time recovery capability

### Resource Limits

**Memory**:
- Minimum: 256Mi
- Recommended: 512Mi - 1Gi
- Large datasets: 2Gi+

**CPU**:
- Minimum: 100m
- Recommended: 250m - 500m
- High throughput: 1000m+

**Disk I/O**:
- Use SSD storage classes
- Provision IOPS appropriately
- Monitor disk latency

### Networking

**Service Configuration**:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: production-cluster
spec:
  clusterIP: None  # Headless service for StatefulSet
  selector:
    app: raft
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: raft
    port: 9090
    targetPort: 9090
```

**Network Policies** (optional):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: raft-network-policy
spec:
  podSelector:
    matchLabels:
      app: raft
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: raft
    ports:
    - protocol: TCP
      port: 9090
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
```

### Monitoring

**Prometheus ServiceMonitor**:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: raft-metrics
spec:
  selector:
    matchLabels:
      app: raft
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

**Grafana Dashboard**:
- Import dashboard from `deploy/grafana/dashboard.json`
- Configure Prometheus datasource
- Set up alerts

### Security

**Pod Security**:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  capabilities:
    drop:
    - ALL
```

**Network Encryption** (future):
- TLS for Raft RPC
- TLS for HTTP API
- mTLS for inter-node communication

## Scaling

### Horizontal Scaling

```bash
# Scale to 5 nodes
kubectl patch raftcluster production-cluster \
  --type='merge' \
  -p '{"spec":{"replicas":5}}'

# Verify scaling
kubectl get pods -l app=raft -w
```

**Important**: Always use odd numbers (3, 5, 7) for quorum.

### Vertical Scaling

```bash
# Update resources
kubectl patch raftcluster production-cluster \
  --type='merge' \
  -p '{"spec":{"resources":{"limits":{"memory":"2Gi"}}}}'
```

## Upgrades

### Rolling Update

```bash
# Update image version
kubectl patch raftcluster production-cluster \
  --type='merge' \
  -p '{"spec":{"image":"hugo/raft-node:v1.1.0"}}'

# Monitor rollout
kubectl rollout status statefulset/production-cluster

# Verify new version
kubectl exec production-cluster-0 -- /root/raftnode --version
```

### Rollback

```bash
# Rollback to previous version
kubectl patch raftcluster production-cluster \
  --type='merge' \
  -p '{"spec":{"image":"hugo/raft-node:v1.0.0"}}'
```

## Multi-Region Deployment

For geo-distributed deployments:

**Considerations**:
- Increased latency (50-100ms+ cross-region)
- Network reliability
- Cost of cross-region traffic

**Configuration**:
```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            app: raft
        topologyKey: topology.kubernetes.io/zone
```

## Disaster Recovery

See [Disaster Recovery Guide](../operations/disaster-recovery.md) for:
- Backup procedures
- Restore procedures
- Failover strategies

## See Also

- [Quick Start Guide](quickstart.md)
- [Configuration Reference](configuration.md)
- [Operations Runbook](../operations/runbook.md)
- [Monitoring Guide](monitoring.md)
