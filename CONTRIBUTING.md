# Contributing to MESH

Thank you for your interest in contributing to MESH! This document provides guidelines for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Standards](#code-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Architecture Overview](#architecture-overview)

## Getting Started

MESH is a multi-repository AI code assistant with semantic search capabilities. Before contributing, please:

1. Read the [README.md](README.md) to understand the project
2. Review the [ARCHITECTURE.md](ARCHITECTURE.md) for architectural context
3. Check existing [issues](../../issues) and [pull requests](../../pulls)

## Development Setup

### Prerequisites

- **Go 1.24.1+** - [Install Go](https://golang.org/dl/)
- **Qdrant** - Vector database (Docker recommended)
- **Anthropic API Key** - For LLM functionality
- **Git** - Version control

### Quick Start

```bash
# Clone the repository
git clone https://github.com/First008/mesh.git
cd mesh

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./cmd/agent

# Run linter
golangci-lint run
```

### Running Qdrant Locally

```bash
docker run -p 6333:6333 qdrant/qdrant
```

## Code Standards

### Go Style Guide

We follow standard Go conventions:

- **Effective Go**: https://golang.org/doc/effective_go.html
- **Go Code Review Comments**: https://github.com/golang/go/wiki/CodeReviewComments
- **gofmt**: All code must be formatted with `go fmt`
- **golangci-lint**: Must pass linter checks

### Project-Specific Principles

From our [CLAUDE.md](/.claude/CLAUDE.md):

1. **ETC Principle**: Design for flexibility by minimizing interdependencies
2. **Eliminate Side Effects**: Write "shy" code that exposes only what's necessary
3. **Avoid Global Data**: Encapsulate data within dedicated APIs
4. **Minimize Coupling**: Fewer interdependencies = easier changes
5. **Tell, Don't Ask**: Avoid chaining method calls
6. **Prefer Interfaces**: Express polymorphism through interfaces, not inheritance
7. **Maximize Concurrency**: Use actors/goroutines, avoid shared state

### Code Quality

- **No duplication**: Follow DRY (Don't Repeat Yourself)
- **Single Responsibility**: Each function/type should have one clear purpose
- **Error handling**: Always wrap errors with context using `fmt.Errorf("%w", err)`
- **Logging**: Use structured logging (zerolog) consistently
- **Thread safety**: Protect shared state with mutexes

## Testing Guidelines

### Coverage Requirements

- **Minimum overall coverage**: 30%
- **New packages**: Should have 50%+ coverage
- **Critical paths**: 80%+ coverage (e.g., cost tracking, core agent logic)

### Writing Tests

```go
func TestFeatureName_Scenario(t *testing.T) {
    // Arrange
    input := setupTestData()

    // Act
    result, err := FeatureName(input)

    // Assert
    if err != nil {
        t.Fatalf("FeatureName failed: %v", err)
    }

    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Test Organization

- Test files: `*_test.go` in same directory as code
- Use table-driven tests for multiple scenarios
- Mock external dependencies (see `internal/testing/mocks.go`)
- Test both happy paths and error cases
- Include concurrency tests for thread-safe code

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...

# Run specific package
go test ./internal/agent/

# Run specific test
go test -run TestAgentAsk ./internal/agent/
```

## Pull Request Process

### Before Submitting

1. **Write tests**: Ensure your changes are tested
2. **Run tests**: `go test ./...` should pass
3. **Check coverage**: Ensure you meet minimum coverage
4. **Run linter**: `golangci-lint run`
5. **Format code**: `go fmt ./...`
6. **Update docs**: If changing APIs or behavior

### PR Guidelines

1. **Create a descriptive PR title**
   - Good: "Add support for Claude Opus 4.5 model"
   - Bad: "Update code"

2. **Write a clear description**
   - What problem does this solve?
   - What approach did you take?
   - Any breaking changes?
   - How to test?

3. **Keep PRs focused**
   - One feature/fix per PR
   - Split large changes into smaller PRs

4. **Link related issues**
   - Use "Fixes #123" or "Closes #123" in description

### Review Process

1. A maintainer will review your PR
2. Address feedback and update your PR
3. Once approved, a maintainer will merge

## Architecture Overview

### Package Structure

```
mesh/
├── cmd/               # Entry points (agent, indexer, mcp-bridge)
├── internal/          # Private application code
│   ├── agent/        # Repository-specific AI agents
│   ├── gateway/      # Multi-repo coordination
│   ├── context/      # Context building for LLM prompts
│   ├── llm/          # LLM provider abstractions
│   ├── vectorstore/  # Vector database & indexing
│   ├── server/       # HTTP API server
│   ├── factory/      # Provider factories
│   ├── filetypes/    # File type detection
│   ├── mcp/          # Model Context Protocol server
│   └── testing/      # Test utilities & mocks
├── pkg/               # Public library code
│   └── telemetry/    # Cost tracking
└── docs/              # Documentation
```

### Key Interfaces

- **LLMProvider**: Abstraction over AI providers (Anthropic, Ollama)
- **VectorStore**: Abstraction over vector databases (Qdrant)
- **EmbeddingProvider**: Abstraction over embedding models

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## Areas for Contribution

### High Priority

- **Increase test coverage**: Target 50%+ overall
- **Integration tests**: End-to-end API testing
- **Performance tests**: Benchmarks for indexing and search
- **Documentation**: Examples, guides, API docs

### Feature Ideas

- Support for additional LLM providers (OpenAI, Gemini)
- Alternative vector stores (Pinecone, Weaviate)
- Advanced chunking strategies
- Query caching
- Metrics and observability

### Bug Fixes

Check the [issue tracker](../../issues) for bugs labeled `good first issue`.

## Code of Conduct

Please be respectful and constructive in all interactions. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Questions?

- Open an [issue](../../issues/new) for questions
- Check [existing issues](../../issues) for similar questions

## License

By contributing, you agree that your contributions will be licensed under the GNU General Public License v3.0.
