# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive unit test suite (70%+ coverage)
- Table-driven tests for all core components
- HTTP API documentation
- Architecture documentation with Mermaid diagrams
- Operations runbook
- Contributing guidelines
- MIT License

### Fixed
- Critical deadlock in vote collection logic
- Split vote issue with proper timeout randomization (150-300ms)
- Config test flag isolation using FlagSet
- Controller reconciliation loop

### Changed
- Election timeout range from 2000-4000ms to 150-300ms (Raft paper spec)
- Improved random seeding for election timeouts

## [0.1.0] - 2025-01-XX

### Added
- Initial Raft consensus implementation
  - Leader election
  - Log replication
  - Safety guarantees
- Key-value state machine
- HTTP API for KV operations
- Write-Ahead Log (WAL) for persistence
- Snapshot support
- Kubernetes Operator
  - RaftCluster CRD
  - StatefulSet management
  - DNS-based peer discovery
  - PersistentVolumeClaim support
- Observability
  - Prometheus metrics
  - Grafana dashboards
  - Structured logging with zap
- Docker and docker-compose support
- Kind/Kubernetes deployment

### Known Issues
- No authentication/authorization
- In-memory only (snapshots help but not unlimited)
- No client libraries yet

## Release Process

1. Update this CHANGELOG.md
2. Update version in relevant files
3. Create git tag: `git tag v0.1.0`
4. Push tag: `git push origin v0.1.0`
5. GitHub Actions will build and publish

## Version History

- **v0.1.0** - Initial release with core Raft functionality
- **Unreleased** - Testing and documentation improvements

[Unreleased]: https://github.com/hugovillarreal/conflux/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hugovillarreal/conflux/releases/tag/v0.1.0
