# Raft KV Store - Quick Start Guide

## Prerequisites

- Docker and Docker Compose installed
- Ports available: 8081-8083 (API), 919x (Raft RPC), 9090 (Prometheus), 3000 (Grafana)

## Start the Cluster

```bash
# Clone and navigate to the project
cd Conflux

# Start all services (3 Raft nodes + Prometheus + Grafana)
docker compose up -d

# Check status
docker compose ps

# View logs
docker logs raft-node-1 --tail 50
```

## Access Points

| Service | URL | Credentials |
|---------|-----|-------------|
| Node 1 API | http://localhost:8081 | - |
| Node 2 API | http://localhost:8082 | - |
| Node 3 API | http://localhost:8083 | - |
| Prometheus | http://localhost:9090 | - |
| Grafana | http://localhost:3000 | admin/admin |

## Test the KV Store

```bash
# Write a key
curl -X PUT http://localhost:8081/kv/mykey \
  -H "Content-Type: application/json" \
  -d '{"value":"hello-world"}'

# Read from same node
curl http://localhost:8081/kv/mykey

# Read from different node (tests replication)
curl http://localhost:8082/kv/mykey

# Delete a key
curl -X DELETE http://localhost:8081/kv/mykey
```

## Monitor the Cluster

### Prometheus Queries

Open http://localhost:9090 and try these queries:

```promql
# Find the leader
raft_is_leader == 1

# Current term across all nodes
raft_term

# Request rate
rate(kv_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(kv_request_duration_seconds_bucket[5m]))
```

### Grafana Dashboard

1. Open http://localhost:3000
2. Login with `admin/admin`
3. Import the dashboard from `deploy/grafana/dashboards/raft-overview.json`
4. Select Prometheus as datasource

## Test Persistence

```bash
# Write data
curl -X PUT http://localhost:8081/kv/persistent \
  -H "Content-Type: application/json" \
  -d '{"value":"survives-restart"}'

# Restart a node
docker compose restart raft-node-1

# Data still available
curl http://localhost:8081/kv/persistent
```

## Troubleshooting

### Check if Raft is enabled
```bash
docker logs raft-node-1 | grep "raft_enabled"
# Should show: "raft_enabled":true
```

### Find the leader
```bash
curl -s http://localhost:8081/metrics | grep raft_is_leader
curl -s http://localhost:8082/metrics | grep raft_is_leader
curl -s http://localhost:8083/metrics | grep raft_is_leader
# One should show: raft_is_leader 1
```

### View Prometheus targets
```bash
# All should be "up"
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {instance, health}'
```

### Port conflicts
If you see "address already in use" errors:
```bash
# Stop the cluster
docker compose down

# Check what's using the ports
lsof -i :9092  # or whatever port is conflicting

# Ports are configurable in docker-compose.yml
```

## Stop the Cluster

```bash
# Stop all services
docker compose down

# Stop and remove volumes (deletes all data)
docker compose down -v
```

## Architecture

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│ Raft Node 1 │  │ Raft Node 2 │  │ Raft Node 3 │
│   :8081     │  │   :8082     │  │   :8083     │
│   :9191     │  │   :9192     │  │   :9193     │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘
       │                │                │
       └────────────────┼────────────────┘
                        │ Raft Consensus
                        │
                   ┌────▼────┐
                   │Prometheus│
                   │  :9090   │
                   └────┬─────┘
                        │
                   ┌────▼────┐
                   │ Grafana │
                   │  :3000  │
                   └─────────┘
```

## Features

✅ **Distributed Consensus** - Raft algorithm for leader election and log replication  
✅ **Persistence** - WAL + snapshots for durability  
✅ **Observability** - Prometheus metrics + Grafana dashboards  
✅ **High Availability** - Survives node failures (2/3 nodes required)  
✅ **Containerized** - Easy deployment with Docker Compose  

## Next Steps

- Create custom Grafana dashboards
- Set up alerting rules in Prometheus
- Deploy to Kubernetes (see docs/design-operator.md)
- Add distributed tracing with Jaeger

