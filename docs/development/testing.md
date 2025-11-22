# Testing Guide

This document describes the testing strategy and how to run tests for the Raft Operator project.

## Test Levels

We use three levels of testing:

1.  **Unit Tests** (`pkg/...`): Fast, isolated tests for individual components.
2.  **Integration Tests** (`test/integration/...`): Multi-node cluster tests running in-process.
3.  **E2E Tests** (`test/e2e/...`): Full system tests running in a Kubernetes cluster (Kind).

## Running Tests

### Unit Tests

Run all unit tests:
```bash
make test-unit
# or
go test -v -race -cover ./pkg/...
```

### Integration Tests

Run integration tests (requires ~30s):
```bash
make test-integration
# or
go test -v -race ./test/integration/...
```

These tests spin up a local 3-node Raft cluster in-memory and verify:
- Leader election
- Log replication
- Persistence (WAL/Snapshots)
- Failover scenarios

### E2E Tests

Run E2E tests (requires Docker and Kind, takes ~10-15m):
```bash
make test-e2e
# or
go test -v -timeout 20m ./test/e2e/...
```

These tests:
1.  Create a Kind cluster
2.  Build and load the operator image
3.  Deploy the operator
4.  Create a `RaftCluster` CR
5.  Verify pod readiness and cluster formation

## Writing Tests

### Unit Tests
- Use table-driven tests.
- Mock dependencies where possible (though we prefer real dependencies for core logic).
- Place tests in `_test.go` files next to the code.

### Integration Tests
- Use `test/integration/helpers.go` for cluster setup.
- Use `NewTestCluster(t, n)` to create a cluster.
- Ensure proper cleanup with `t.Cleanup()`.

### E2E Tests
- Use `test/e2e/framework.go`.
- Use `NewFramework(t)` to get a test framework instance.
- Call `f.Setup()` and `defer f.Teardown()`.

## CI/CD

Tests are run automatically on GitHub Actions:
- **Unit & Integration**: On every push and PR.
- **E2E**: On push to main and PRs with label `e2e`.
