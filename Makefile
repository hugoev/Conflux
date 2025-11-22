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

# Operator proxy targets
.PHONY: install
install:
	cd operator && make install

.PHONY: deploy
deploy:
	cd operator && make deploy IMG=$(IMG)


.PHONY: test-unit test-integration test-e2e test-all test-operator
test-unit:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage-unit.out -covermode=atomic -timeout=10m ./pkg/...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -timeout=20m -coverprofile=coverage-integration.out ./test/integration/...

test-e2e:
	@echo "Running E2E tests..."
	@echo "Note: E2E tests require Docker and Kind"
	go test -v -timeout=30m -coverprofile=coverage-e2e.out ./test/e2e/...

test-operator:
	@echo "Running operator tests..."
	cd operator && go test -v -race -timeout=10m -coverprofile=../coverage-operator.out ./...

test-all: test-unit test-integration test-operator
	@echo "All tests completed (excluding E2E - run 'make test-e2e' separately)"

test-coverage:
	@echo "Generating combined coverage report..."
	@go test -coverprofile=coverage.out ./pkg/... ./test/integration/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -run=^$$ ./pkg/...

.PHONY: lint lint-fix
lint:
	@echo "Running linter..."
	golangci-lint run --timeout=5m ./...

lint-fix:
	@echo "Fixing linting issues..."
	golangci-lint run --fix ./...

.PHONY: coverage coverage-html coverage-view
coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | tail -1

coverage-html: coverage
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

coverage-view: coverage-html
	@echo "Opening coverage report..."
	@open coverage.html || xdg-open coverage.html || echo "Please open coverage.html manually"

.PHONY: verify
verify: lint test-unit
	@echo "Verification complete"

.PHONY: fmt fmt-check
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@go mod tidy

fmt-check:
	@echo "Checking code formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		gofmt -s -d .; \
		exit 1; \
	fi

