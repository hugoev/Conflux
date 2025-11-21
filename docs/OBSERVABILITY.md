# Observability Guide

This guide explains how to monitor and observe your Raft cluster using Prometheus and Grafana.

## Architecture

```
Raft Nodes (3x)
    │
    └── /metrics endpoint (HTTP API port)
         │
         v
    Prometheus (scrapes every 15s)
         │
         v
     Grafana (visualization)
```

## Quick Start

### Start the Observability Stack

```bash
# Start all services (Raft nodes + Prometheus + Grafana)
docker-compose up -d

# Check all services are running
docker-compose ps
```

### Access Dashboards

- **Grafana**: http://localhost:3000
  - Username: `admin`
  - Password: `admin`
  
- **Prometheus**: http://localhost:9090

## Available Metrics

### Raft Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `raft_is_leader` | Gauge | 1 if node is leader, 0 otherwise |
| `raft_term` | Gauge | Current Raft term |
| `raft_commit_index` | Gauge | Current commit index |
| `raft_log_entries_total` | Counter | Total log entries created |
| `raft_election_timeouts_total` | Counter | Total election timeouts |

### KV Store Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `kv_requests_total` | Counter | Total KV requests by method and status |
| `kv_request_duration_seconds` | Histogram | KV request latency distribution |

### Storage Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `wal_size_bytes` | Gauge | Size of WAL in bytes |
| `snapshot_size_bytes` | Gauge | Size of latest snapshot |

## Useful Queries

### Prometheus Queries

**Find the current leader:**
```promql
raft_is_leader == 1
```

**Request rate (last 5 minutes):**
```promql
rate(kv_requests_total[5m])
```

**95th percentile latency:**
```promql
histogram_quantile(0.95, rate(kv_request_duration_seconds_bucket[5m]))
```

**Election timeout rate:**
```promql
rate(raft_election_timeouts_total[5m])
```

**Log growth rate:**
```promql
rate(raft_log_entries_total[5m])
```

## Grafana Dashboard

The cluster includes a pre-built dashboard at:
- `deploy/grafana/dashboards/raft-overview.json`

### Importing the Dashboard

1. Open Grafana (http://localhost:3000)
2. Login with `admin/admin`
3. Go to Dashboards → Import
4. Upload `raft-overview.json`
5. Select Prometheus as datasource

### Dashboard Panels

1. **Leader Status** - Shows which node is currently leader
2. **Current Term** - Raft term across nodes
3. **Commit Index** - Log replication progress
4. **Election Timeouts** - Election activity
5. **KV Request Rate** - Throughput metrics
6. **KV Request Latency** - Performance metrics
7. **WAL Size** - Storage usage
8. **Snapshot Size** - Compaction metrics

## Alerting (Future)

You can configure Prometheus alerting rules for:

- **No Leader**: Alert when no node has `raft_is_leader == 1`
- **High Latency**: Alert when p95 latency > threshold
- **Election Storm**: Alert when election timeout rate is high
- **Storage Growth**: Alert when WAL/snapshot sizes grow too fast

Example alert rule:
```yaml
groups:
  - name: raft
    rules:
      - alert: NoLeader
        expr: sum(raft_is_leader) == 0
        for: 30s
        annotations:
          summary: "Raft cluster has no leader"
```

## Troubleshooting

### Metrics not appearing

1. Check if nodes are running:
   ```bash
   docker-compose ps
   ```

2. Check if metrics endpoint responds:
   ```bash
   curl http://localhost:8081/metrics
   ```

3. Check Prometheus targets:
   - Open http://localhost:9090/targets
   - All targets should be "UP"

### Grafana shows "No data"

1. Verify Prometheus datasource is configured
2. Check time range (last 5 minutes)
3. Ensure nodes have been running long enough to collect data

### High cardinality warnings

If you see warnings about high cardinality metrics, consider:
- Reducing scrape frequency in `prometheus.yml`
- Adding recording rules for frequently-used queries
- Limiting label values

## Performance Impact

- **Scrape interval**: 15 seconds (configurable in `prometheus.yml`)
- **Metrics overhead**: < 1% CPU, < 10MB memory per node
- **Storage**: ~100MB per day for 3 nodes (default retention: 15 days)

## Next Steps

- Create custom dashboards for your use case
- Set up alerting rules
- Export metrics to external systems (e.g., Datadog, New Relic)
- Add distributed tracing with Jaeger
