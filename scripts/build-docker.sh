#!/bin/bash
set -e

echo "Building Conflux Raft Docker image..."
docker build -t conflux-raft:latest .

echo "✅ Build complete!"
echo "Image: conflux-raft:latest"
docker images | grep conflux-raft
