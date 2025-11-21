# Contributing to Conflux

Thank you for your interest in contributing to Conflux! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## Getting Started

### Prerequisites

- **Go** 1.21 or later
- **Docker** for building images
- **kubectl** and **kind** for Kubernetes testing
- **make** for build automation

### Development Setup

```bash
# Clone the repository
git clone https://github.com/hugovillarreal/conflux.git
cd conflux

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the binary
make build

# Run locally
./raftnode --node-id=node-0 --port=8080
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

**Branch naming**:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `test/` - Test additions/improvements
- `refactor/` - Code refactoring

### 2. Make Changes

Follow the [code style guidelines](#code-style) and ensure:
- All tests pass
- Code is properly formatted
- New code has tests
- Documentation is updated

### 3. Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./pkg/raft -v

# Run linter
make lint
```

### 4. Commit Changes

```bash
git add .
git commit -m "feat: add amazing feature"
```

**Commit message format**:
```
<type>: <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `test`: Tests
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `chore`: Maintenance

**Example**:
```
feat: add snapshot compression

Implement gzip compression for snapshots to reduce
storage requirements by ~70%.

Closes #123
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub with:
- Clear description of changes
- Link to related issues
- Screenshots (if UI changes)
- Test results

## Code Style

### Go Code

Follow standard Go conventions:

```go
// ✅ Good
func (n *Node) processEntry(entry LogEntry) error {
    if entry.Term < n.currentTerm {
        return ErrStaleTerm
    }
    // ...
}

// ❌ Bad
func (n *Node) ProcessEntry(e LogEntry) error {
    if e.Term<n.currentTerm{return ErrStaleTerm}
    // ...
}
```

**Guidelines**:
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable names
- Add godoc comments for exported functions
- Keep functions small and focused
- Use table-driven tests

### Testing

All new code must include tests:

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    Type
        expected Type
    }{
        {
            name:     "case 1",
            input:    value1,
            expected: result1,
        },
        // More cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Feature(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

**Test guidelines**:
- Use table-driven tests
- Test edge cases
- Use `t.Helper()` for test helpers
- Use `t.TempDir()` for temporary directories
- Aim for >70% coverage

### Documentation

- Add godoc comments for all exported functions
- Update README.md for user-facing changes
- Add examples in godoc comments
- Update relevant documentation in `docs/`

**Example**:
```go
// RequestVote handles incoming vote requests from candidates.
// It implements the RequestVote RPC as defined in the Raft paper.
//
// The function grants a vote if:
//  1. The candidate's term is >= current term
//  2. The node hasn't voted for another candidate this term
//  3. The candidate's log is at least as up-to-date
//
// Example:
//
//	args := &RequestVoteArgs{Term: 5, CandidateID: "node-1"}
//	reply := &RequestVoteReply{}
//	err := node.RequestVote(args, reply)
func (n *Node) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
    // Implementation...
}
```

## Pull Request Process

### Before Submitting

- [ ] All tests pass locally
- [ ] Code is formatted (`gofmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] Branch is up-to-date with main

### PR Checklist

Your PR should include:

- [ ] Clear description of changes
- [ ] Link to related issue(s)
- [ ] Tests for new functionality
- [ ] Documentation updates
- [ ] No breaking changes (or clearly documented)

### Review Process

1. **Automated Checks**: CI runs tests and linters
2. **Code Review**: Maintainer reviews code
3. **Revisions**: Address feedback
4. **Approval**: Maintainer approves
5. **Merge**: Squash and merge to main

## Areas for Contribution

### Good First Issues

Look for issues labeled `good-first-issue`:
- Documentation improvements
- Test coverage improvements
- Bug fixes with clear reproduction steps

### High Priority

- Performance optimizations
- Additional metrics and observability
- Integration tests
- E2E tests
- Client libraries (Python, JavaScript, etc.)

### Feature Requests

Before implementing a major feature:
1. Open an issue to discuss
2. Get maintainer approval
3. Create design document (if needed)
4. Implement with tests
5. Submit PR

## Release Process

Releases are handled by maintainers:

1. Update CHANGELOG.md
2. Tag version (`git tag v1.2.3`)
3. Push tag (`git push origin v1.2.3`)
4. GitHub Actions builds and publishes
5. Update documentation

## Getting Help

- **Questions**: Open a [GitHub Discussion](https://github.com/hugovillarreal/conflux/discussions)
- **Bugs**: Open a [GitHub Issue](https://github.com/hugovillarreal/conflux/issues)
- **Chat**: Join our community (link TBD)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors are recognized in:
- CHANGELOG.md
- GitHub contributors page
- Release notes

Thank you for contributing to Conflux! 🎉
