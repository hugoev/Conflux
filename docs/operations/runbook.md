# Operations Runbook

## Overview

This runbook provides step-by-step procedures for common operational tasks and troubleshooting scenarios.

## Daily Operations

### Checking Cluster Health

```bash
# Check all pods are running
kubectl get pods -l app=raft

# Check RaftCluster status
kubectl get raftclusters

# Check individual node health
kubectl exec raft-sample-0 -- curl localhost:8080/health
```

**Expected Output**:
```json
{
  "status": "healthy",
  "node_id": "raft-sample-0",
  "state": "leader",
  "term": 5,
  "commit_index": 1234
}
```

### Identifying the Leader

```bash
# Check which node is the leader
kubectl logs -l app=raft --tail=50 | grep "Became leader"

# Or check health endpoint on all nodes
for i in 0 1 2; do
  echo "Node $i:"
  kubectl exec raft-sample-$i -- curl -s localhost:8080/health | jq '.state'
done
```

### Monitoring Metrics

```bash
# Port-forward to Grafana
kubectl port-forward svc/grafana 3000:3000

# Open dashboard
open http://localhost:3000

# Or query Prometheus directly
kubectl port-forward svc/prometheus 9090:9090
```

## Troubleshooting

### No Leader Elected

**Symptoms**:
- All nodes show `state: "candidate"` or `state: "follower"`
- Write operations fail with "not leader"
- Logs show continuous elections

**Diagnosis**:
```bash
# Check election logs
kubectl logs -l app=raft --tail=100 | grep -E "(Starting election|Won election|Granted vote)"

# Check network connectivity
kubectl exec raft-sample-0 -- nc -zv raft-sample-1.raft-sample 9090
```

**Common Causes**:
1. **Network partition** - Nodes can't communicate
2. **Split votes** - Insufficient randomization (fixed in latest version)
3. **Insufficient replicas** - Need odd number (3, 5, 7)

**Resolution**:
```bash
# Check pod network
kubectl exec raft-sample-0 -- ping raft-sample-1.raft-sample

# Restart pods to trigger new election
kubectl delete pod -l app=raft

# Verify DNS resolution
kubectl exec raft-sample-0 -- nslookup raft-sample-1.raft-sample
```

### Pod Crash Loop

**Symptoms**:
- Pod status shows `CrashLoopBackOff`
- Frequent restarts

**Diagnosis**:
```bash
# Check pod events
kubectl describe pod raft-sample-0

# Check logs from previous crash
kubectl logs raft-sample-0 --previous

# Check resource usage
kubectl top pod raft-sample-0
```

**Common Causes**:
1. **OOM (Out of Memory)** - Increase memory limits
2. **Panic in code** - Check logs for stack trace
3. **Failed health checks** - Adjust probe timings

**Resolution**:
```bash
# Increase memory limits
kubectl edit raftcluster raft-sample
# Update spec.resources.limits.memory

# Adjust health probe timings
kubectl edit statefulset raft-sample
# Update readinessProbe.initialDelaySeconds
```

### Data Loss After Restart

**Symptoms**:
- Keys missing after pod restart
- Commit index reset to 0

**Diagnosis**:
```bash
# Check PVC status
kubectl get pvc

# Check if data directory is mounted
kubectl exec raft-sample-0 -- ls -la /data

# Check WAL files
kubectl exec raft-sample-0 -- ls -la /data/wal
```

**Common Causes**:
1. **No PVC configured** - Data stored in ephemeral storage
2. **PVC deleted** - Persistent volume removed
3. **WAL corruption** - Disk errors

**Resolution**:
```bash
# Ensure PVCs are created
kubectl get pvc -l app=raft

# If missing, update RaftCluster to enable persistence
kubectl edit raftcluster raft-sample

# Restore from backup (if available)
# See backup-restore.md
```

### High Write Latency

**Symptoms**:
- Write operations taking >100ms
- Timeout errors from clients

**Diagnosis**:
```bash
# Check Raft metrics
kubectl port-forward raft-sample-0 8080:8080
curl localhost:8080/metrics | grep raft_append_entries_latency

# Check network latency between nodes
kubectl exec raft-sample-0 -- ping -c 10 raft-sample-1.raft-sample
```

**Common Causes**:
1. **Network latency** - Cross-region deployment
2. **Disk I/O** - Slow WAL writes
3. **Large log entries** - Optimize data size

**Resolution**:
```bash
# Use faster storage class
kubectl edit pvc raft-sample-0
# Change storageClassName to faster option

# Reduce log entry size
# Implement client-side compression

# Co-locate nodes in same region/zone
kubectl edit raftcluster raft-sample
# Add nodeAffinity rules
```

### Leader Keeps Changing

**Symptoms**:
- Frequent "Became leader" messages
- Unstable cluster
- Write failures

**Diagnosis**:
```bash
# Check election frequency
kubectl logs -l app=raft --since=5m | grep "Became leader" | wc -l

# Check heartbeat failures
kubectl logs -l app=raft | grep "AppendEntries failed"
```

**Common Causes**:
1. **Network instability** - Packet loss
2. **Resource contention** - CPU throttling
3. **Clock skew** - Time synchronization issues

**Resolution**:
```bash
# Increase heartbeat interval (requires code change)
# Check network quality
kubectl exec raft-sample-0 -- ping -c 100 raft-sample-1.raft-sample

# Increase CPU limits
kubectl edit raftcluster raft-sample
```

## Maintenance Tasks

### Rolling Restart

```bash
# Restart pods one at a time
for i in 0 1 2; do
  kubectl delete pod raft-sample-$i
  kubectl wait --for=condition=ready pod/raft-sample-$i --timeout=120s
  sleep 10
done
```

### Scaling the Cluster

```bash
# Scale to 5 nodes
kubectl patch raftcluster raft-sample --type='merge' -p '{"spec":{"replicas":5}}'

# Wait for new pods
kubectl wait --for=condition=ready pod -l app=raft --timeout=180s

# Verify all nodes joined
kubectl logs -l app=raft | grep "peers"
```

**⚠️ Important**: Always use odd numbers (3, 5, 7) for quorum

### Upgrading the Image

```bash
# Update image version
kubectl patch raftcluster raft-sample --type='merge' \
  -p '{"spec":{"image":"hugo/raft-node:v2.0.0"}}'

# Monitor rollout
kubectl rollout status statefulset/raft-sample

# Verify new version
kubectl exec raft-sample-0 -- /root/raftnode --version
```

### Taking a Backup

```bash
# Create snapshot on leader
LEADER=$(kubectl get pods -l app=raft -o json | \
  jq -r '.items[] | select(.metadata.name | contains("0")) | .metadata.name')

kubectl exec $LEADER -- curl -X POST localhost:8080/admin/snapshot

# Copy snapshot file
kubectl cp $LEADER:/data/snapshots/latest.snap ./backup-$(date +%Y%m%d).snap
```

## Emergency Procedures

### Complete Cluster Failure

**Scenario**: All nodes down, no leader

```bash
# 1. Check if any data is recoverable
kubectl get pvc -l app=raft

# 2. Delete and recreate cluster
kubectl delete raftcluster raft-sample
kubectl apply -f config/samples/raft_v1alpha1_raftcluster.yaml

# 3. Restore from backup (if available)
# See backup-restore.md

# 4. Verify cluster health
kubectl wait --for=condition=ready pod -l app=raft --timeout=180s
```

### Split Brain Prevention

Raft prevents split brain by design (majority quorum), but if you suspect it:

```bash
# Check term numbers - should be consistent
for i in 0 1 2; do
  echo "Node $i term:"
  kubectl exec raft-sample-$i -- curl -s localhost:8080/health | jq '.term'
done

# Check commit indices - should be close
for i in 0 1 2; do
  echo "Node $i commit_index:"
  kubectl exec raft-sample-$i -- curl -s localhost:8080/health | jq '.commit_index'
done
```

## Monitoring Alerts

### Recommended Alerts

1. **No Leader** (Critical)
   ```promql
   sum(raft_state == 2) == 0
   ```

2. **High Election Frequency** (Warning)
   ```promql
   rate(raft_term[5m]) > 0.1
   ```

3. **Replication Lag** (Warning)
   ```promql
   raft_commit_index - raft_last_applied > 100
   ```

4. **Pod Not Ready** (Critical)
   ```promql
   kube_pod_status_ready{pod=~"raft-.*"} == 0
   ```

## Performance Tuning

### Optimize for Throughput

```yaml
# Increase batch size (requires code change)
# Reduce fsync frequency (trade durability for speed)
# Use faster storage (NVMe SSDs)
```

### Optimize for Latency

```yaml
# Co-locate nodes in same AZ
# Use low-latency network
# Reduce election timeout (150-300ms default)
```

## See Also

- [Backup and Restore](backup-restore.md)
- [Scaling Guide](scaling.md)
- [Architecture Documentation](../architecture/README.md)
- [Troubleshooting Guide](troubleshooting.md)
