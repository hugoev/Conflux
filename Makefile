.PHONY: build test clean run-local operator-install operator-run

# Build the raft node binary
build:
	go build -o bin/raftnode cmd/raftnode/main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/ data/

# Run locally (single node)
run-local:
	go run cmd/raftnode/main.go --node-id=node-0 --port=8080

# Install operator dependencies
operator-install:
	cd operator && kubebuilder init --domain hugo.dev --repo github.com/hugovillarreal/conflux/operator

# Run operator locally
operator-run:
	cd operator && make run

# Docker build
docker-build:
	docker build -t hugo/raft-node:latest -f Dockerfile .

# Kind cluster setup
kind-create:
	kind create cluster --name conflux

kind-delete:
	kind delete cluster --name conflux

