# Testing Guide

This document describes the comprehensive testing strategy for the Conflux Raft KV Store project.

## Test Levels

We use a three-tier testing strategy:

1. **Unit Tests** (`pkg/...`): Fast, isolated tests for individual components
2. **Integration Tests** (`test/integration/...`): Multi-node cluster tests running in-process
3. **E2E Tests** (`test/e2e/...`): Full system tests running in a Kubernetes cluster (Kind)

## Running Tests

### Unit Tests

```bash
# Run all unit tests
make test-unit

# Or directly
go test -v -race -cover ./pkg/...

# With coverage report
make coverage-html
```

### Integration Tests

```bash
# Run integration tests
make test-integration

# Or directly
go test -v -race -timeout=20m ./test/integration/...
```

**Integration Test Scenarios:**
- Cluster formation and leader election
- Leader failover and recovery
- Log replication and consistency
- Network partitions and split-brain prevention
- Persistence and crash recovery
- Concurrent operations
- Performance benchmarks

### E2E Tests

```bash
# Run E2E tests (requires Docker and Kind)
make test-e2e

# Or directly
go test -v -timeout=30m ./test/e2e/...
```

**E2E Test Scenarios:**
- Operator deployment and CRUD operations
- RaftCluster lifecycle management
- Status updates and health checks
- Error handling and recovery
- Multiple cluster instances
- Scaling operations

### All Tests

```bash
# Run all tests (excluding E2E)
make test-all

# Run E2E separately
make test-e2e
```

## Test Coverage

Current coverage targets:
- `pkg/config`: 94.7%
- `pkg/kv`: 65.2%
- `pkg/raft`: 57.0%
- **Overall**: ~70%

View coverage reports:
```bash
make coverage-view
```

## Integration Test Details

### Test Utilities

The `test/integration/test_utils.go` file provides helper functions:
- `RetryWithBackoff`: Retry operations with exponential backoff
- `WaitForCondition`: Wait for conditions to become true
- `VerifyConsistency`: Verify data consistency across nodes
- `WriteData` / `ReadData`: Helper functions for data operations
- `MeasureLatency`: Measure operation latency
- `BenchmarkThroughput`: Benchmark operation throughput

### Test Scenarios

#### Cluster Formation
- Tests cluster startup and leader election
- Verifies exactly one leader is elected
- Tests with different cluster sizes (3, 5 nodes)

#### Leader Election
- Tests leader election after startup
- Verifies leader stability with heartbeats
- Tests split-vote prevention

#### Leader Failover
- Tests leader failure and new leader election
- Verifies multiple failover scenarios
- Tests leader restart and rejoin

#### Network Partitions
- Tests behavior during network partitions
- Verifies majority partition continues operating
- Tests partition healing and data sync

#### Split-Brain Prevention
- Ensures no split-brain scenarios occur
- Verifies majority quorum requirements
- Tests single node isolation

#### Log Replication
- Tests log replication to all followers
- Verifies commit index advancement
- Tests replication after failover

#### Persistence
- Tests data persistence across restarts
- Verifies WAL and snapshot recovery
- Tests crash recovery scenarios

#### Concurrent Operations
- Tests concurrent writes from multiple clients
- Verifies data consistency under load
- Tests race conditions

#### Performance
- Benchmarks write throughput
- Measures operation latency
- Tests under various load conditions

## E2E Test Details

### Framework

The `test/e2e/framework.go` provides:
- Kind cluster setup and teardown
- Operator deployment
- RaftCluster CR management
- Pod readiness checks
- Status verification

### Test Scenarios

#### Operator Deployment
- Verifies operator deployment
- Checks StatefulSet creation
- Verifies Service creation

#### CRUD Operations
- Tests RaftCluster creation
- Tests scaling (up and down)
- Tests deletion and cleanup

#### Status Updates
- Verifies status field updates
- Checks ReadyReplicas tracking
- Verifies Leader field updates

#### Error Handling
- Tests invalid configurations
- Tests pod deletion and recovery
- Tests resource cleanup

#### Multiple Instances
- Tests multiple RaftCluster instances
- Verifies resource isolation
- Tests concurrent operations

## CI/CD Integration

Tests are automatically run in CI/CD pipeline:

1. **Lint**: Code formatting and linting checks
2. **Unit Tests**: Fast unit tests with coverage
3. **Integration Tests**: Comprehensive integration tests
4. **Operator Tests**: Operator-specific tests
5. **Security Scan**: Security vulnerability scanning
6. **Build**: Docker image builds
7. **E2E Tests**: Full end-to-end tests (on main branch)

### GitHub Actions Workflows

- `.github/workflows/ci.yml`: Comprehensive CI pipeline
- `.github/workflows/test.yml`: Legacy test workflow (kept for compatibility)
- `.github/workflows/e2e.yml`: E2E test workflow
- `.github/workflows/build.yml`: Build workflow

## Best Practices

### Writing Tests

1. **Use table-driven tests** for multiple scenarios
2. **Use helpers** from `test_utils.go` for common operations
3. **Clean up resources** using `t.Cleanup()`
4. **Use timeouts** appropriately for async operations
5. **Log important events** using `t.Logf()`

### Test Organization

- Group related tests in the same file
- Use descriptive test names
- Add comments for complex test scenarios
- Keep tests independent and isolated

### Performance Considerations

- Use `testing.Short()` to skip long-running tests
- Use appropriate timeouts
- Clean up resources promptly
- Use `t.Parallel()` where appropriate

## Troubleshooting

### Integration Tests Failing

1. Check for port conflicts
2. Verify sufficient system resources
3. Check for race conditions (run with `-race`)
4. Increase timeouts if needed

### E2E Tests Failing

1. Verify Docker and Kind are installed
2. Check Kind cluster status
3. Review operator logs
4. Check pod status and events

### Coverage Issues

1. Run `make coverage-html` to view detailed coverage
2. Focus on uncovered critical paths
3. Add tests for edge cases
4. Maintain minimum 70% overall coverage

## Continuous Improvement

- Regularly review and update test scenarios
- Add tests for new features
- Improve test performance
- Increase test coverage
- Add more E2E scenarios
