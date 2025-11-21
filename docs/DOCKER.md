# Docker Deployment Guide

## Quick Start

### Prerequisites
- Docker installed and running
- Docker Compose installed

### Build and Run

```bash
# Build the Docker image
./scripts/build-docker.sh

# Start 3-node cluster
./scripts/run-cluster.sh

# Check cluster status
docker-compose ps

# View logs
docker-compose logs -f

# Stop cluster
docker-compose down
```

## Testing the Cluster

### Check Health
```bash
curl http://localhost:8081/health
curl http://localhost:8082/health
curl http://localhost:8083/health
```

### Write Data
```bash
# Write to node 1
curl -X PUT http://localhost:8081/kv/mykey \
  -H "Content-Type: application/json" \
  -d '{"value":"hello-raft"}'
```

### Read Data (should be replicated)
```bash
# Read from node 2
curl http://localhost:8082/kv/mykey

# Read from node 3
curl http://localhost:8083/kv/mykey
```

### Check Leader
```bash
# Check metrics to see which node is leader
for port in 8081 8082 8083; do
  echo "Node on port $port:"
  curl -s http://localhost:$port/metrics | grep raft_state
done
```

## Architecture

The docker-compose.yml creates a 3-node Raft cluster:

- **raft-node-1**: Exposed on ports 8081 (API) and 9091 (Raft)
- **raft-node-2**: Exposed on ports 8082 (API) and 9092 (Raft)
- **raft-node-3**: Exposed on ports 8083 (API) and 9093 (Raft)

All nodes communicate via the `raft-network` bridge network.

## Configuration

Each node is configured via environment variables:

- `NODE_ID`: Unique identifier for the node
- `RAFT_PORT`: Port for Raft RPC communication (9090)
- `API_PORT`: Port for HTTP API (8080)
- `PEERS`: Comma-separated list of all cluster members

## Troubleshooting

### View logs for specific node
```bash
docker-compose logs -f raft-node-1
```

### Restart a node
```bash
docker-compose restart raft-node-2
```

### Rebuild after code changes
```bash
docker-compose down
./scripts/build-docker.sh
./scripts/run-cluster.sh
```

### Clean up everything
```bash
docker-compose down -v
docker rmi conflux-raft:latest
```
