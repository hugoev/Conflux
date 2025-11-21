#!/bin/bash
set -e

echo "Starting 3-node Raft cluster..."
docker-compose up -d

echo ""
echo "Waiting for nodes to start..."
sleep 5

echo ""
echo "✅ Cluster started!"
echo ""
echo "Node status:"
docker-compose ps

echo ""
echo "Access points:"
echo "  Node 1: http://localhost:8081"
echo "  Node 2: http://localhost:8082"
echo "  Node 3: http://localhost:8083"
echo ""
echo "To view logs: docker-compose logs -f"
echo "To stop: docker-compose down"
