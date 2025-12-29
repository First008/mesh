# MESH 🕸️

Multi-repository AI Code Assistant with Semantic Search. Ask questions about your codebase, get accurate answers based on actual code with file:line references.

[Quick Start](#quick-start) • [Architecture](#architecture) • [Configuration](#configuration) • [API](#api-reference) • [Development](#development)

---

## What is MESH?

MESH is a semantic search gateway for codebases. It indexes code into vector embeddings, performs intelligent search, and uses LLMs to answer questions with zero hallucinations.

### Problem vs Solution

| Without MESH | With MESH |
|--------------|-----------|
| AI hallucinates non-existent code | Answers strictly from indexed code |
| Manual grep/find through thousands of files | Semantic search finds relevant code automatically |
| Context scattered across repos | Single gateway for all repositories |
| Separate AI setup per project | One service, unlimited repos |

### Key Features

- **Semantic Search**: Vector embeddings with Qdrant, chunk aggregation for complete files
- **Multi-Repository**: Single gateway manages unlimited repos with branch-aware indexing
- **Cost Optimized**: 90% cost reduction via Anthropic prompt caching, incremental indexing
- **Universal Integration**: REST API, MCP protocol for Claude Code, GitHub webhooks
- **Zero Hallucinations**: Strict context-only constraints, file:line references for all claims

---

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Ollama (free local embeddings) OR OpenAI API key
- Anthropic API key (recommended) OR OpenAI/Ollama for LLM

### 1. Install Ollama (Recommended)

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh
or 
brew install ollama

# Start service
ollama serve

# Pull embedding model (1.3GB)
ollama pull bge-m3
```

### 2. Configure

```bash
git clone https://github.com/First008/mesh.git
cd mesh

# Create .env in mesh root directory
cat > .env << EOF
ANTHROPIC_API_KEY=sk-ant-your-key-here
LOG_LEVEL=info
# Optional: Set if mesh is not in same directory as other repos
# PLATFORM_DIR=/path/to/parent/directory
EOF
```

**Directory Structure:**
```
/your/projects/
  ├── mesh/              (this repository)
  ├── repo1/             (your codebases)
  ├── repo2/
  └── repo3/
```

If your repos are siblings to mesh (as above), you don't need `PLATFORM_DIR` - the default works.
If mesh is elsewhere, set `PLATFORM_DIR=/your/projects` in `.env`.

**Create `configs/repos.yaml`:**
Prepare repositories with proper name, path (relative to `/repos` mount), focus paths, exclude patterns and personality for best results.

### 3. Start

```bash
# Start services
cd deployments/docker
docker compose up -d

# Watch logs
docker compose logs -f mesh-gateway

# Wait for: "INF Gateway initialized successfully"
```

### 4. Query

```bash
# Health check
curl http://localhost:9000/health

# Ask a question
curl -X POST http://localhost:9000/ask/your-repo \
  -H 'Content-Type: application/json' \
  -d '{"question":"How does authentication work?"}'
```

**Response includes**:
- Answer with code snippets and file:line references
- Token usage (input/output/cached)
- Cost tracking per query

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  MESH Gateway (Single Container)            │
│  ┌────────────┐              ┌──────────────────────┐       │
│  │ HTTP Server│              │  Branch Scanner      │       │
│  │ Port 9000  │              │  Auto Re-index       │       │
│  └─────┬──────┘              └──────────────────────┘       │
│        │                                                     │
│  ┌─────▼──────────────────────────────────────────────┐     │
│  │      Repository Agents (Per Repo)                  │     │
│  │  ┌──────────────┐  ┌──────────────┐               │     │
│  │  │ Repo 1 Agent │  │ Repo 2 Agent │    ... more   │     │
│  │  │ ├─ LLM       │  │ ├─ LLM       │               │     │
│  │  │ ├─ Vector    │  │ ├─ Vector    │               │     │
│  │  │ └─ Context   │  │ └─ Context   │               │     │
│  │  └──────────────┘  └──────────────┘               │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
        ┌─────────────────────────────────┐
        │   Qdrant Vector Database        │
        │   (Branch-Aware Collections)    │
        │                                 │
        │  mesh-repo1-main-v1             │
        │  mesh-repo1-staging-v1          │
        │  mesh-repo2-develop-v1          │
        └─────────────────────────────────┘
```

### Core Components

| Component | Location | Purpose |
|-----------|----------|---------|
| Gateway | `internal/gateway` | Multi-repo orchestration, branch scanning |
| Agent | `internal/agent` | Per-repo LLM integration, context building |
| Vector Store | `internal/vectorstore` | Indexing, semantic search, chunking |
| Context Builder | `internal/context` | File selection, context layering |
| LLM Providers | `internal/llm` | Anthropic, OpenAI, Ollama adapters |
| HTTP Server | `internal/server` | REST API endpoints |
| MCP Server | `internal/mcp` | Model Context Protocol for Claude Code |

**Execution Modes** (single binary):
- **Gateway**: Multi-repo (recommended)
- **HTTP**: Single-repo backward compatibility
- **MCP**: Claude Code integration

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design.

---

## Configuration

### Repository Configuration (`configs/repos.yaml`)

```yaml
port: 9000
qdrant_url: "qdrant:6334"

# Embedding config
embedding_provider: "ollama"          # ollama | openai
embedding_model: "bge-m3"             # bge-m3 (8K context) | nomic-embed-text (2K)
ollama_url: "http://host.docker.internal:11434"

# LLM config
llm_provider: "anthropic"             # anthropic | openai | ollama
llm_model: "claude-sonnet-4-5-20250929"

# Repositories
repos:
  - name: my-backend
    path: /repos/my-backend
    focus_paths:                      # Optional: prioritize important code
      - internal/**
      - pkg/**
    personality: |                     # Optional: customize AI expertise
      Expert in Go microservices, gRPC, distributed systems.
```

### Docker Compose

Uses `deployments/docker/docker-compose.yml` for the gateway (single file for multi-repo setup).

**Volume Mount Configuration:**
The compose file automatically mounts your repositories using `PLATFORM_DIR` from `.env`:

```yaml
volumes:
  - ${PLATFORM_DIR:-../../..}:/repos:ro  # Auto-discovers sibling repos
```

**No configuration needed** if your directory structure is:
```
/your/projects/
  ├── mesh/
  ├── repo1/
  └── repo2/
```

**Custom location?** Set in `.env`:
```bash
PLATFORM_DIR=/path/to/your/projects
```

### Embedding Models

| Model | Context | Quality | Speed | Use Case |
|-------|---------|---------|-------|----------|
| bge-m3 | 8K tokens | Highest | ~6s | Production (recommended) |
| nomic-embed-text | 2K tokens | Good | ~2s | Fast iteration, small files only (large files will fail) |

### LLM Providers

**Anthropic Claude** (Recommended):
- 90% cost reduction via prompt caching
- Best code understanding quality
- ~$0.03-0.10 per query

**Ollama** (Free):
- Zero cost, runs locally
- Requires 32GB+ RAM, GPU recommended
- Models: `llama3.3:70b`, `deepseek-coder-v2`

**OpenAI**:
- Good quality, no caching support
- ~$0.05-0.30 per query

---

## API Reference

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service information (mode, model, etc) |
| `/metrics` | GET | Usage statistics (gateway only) |
| `/repos` | GET | List indexed repositories with branch info (gateway only) |
| `/repos/:repo` | GET | Get specific repository info (gateway only) |
| `/ask` | POST | Query repository (single-repo mode) |
| `/ask/:repo` | POST | Query specific repository (gateway mode) |
| `/ask-all` | POST | Query all repositories (gateway mode) |
| `/repos/:repo/reindex` | POST | Trigger incremental re-indexing (gateway only) |
| `/webhooks/github` | POST | GitHub webhook receiver (gateway only) |

### Query Example

**Request**:
```bash
curl -X POST http://localhost:9000/ask/my-backend \
  -H 'Content-Type: application/json' \
  -d '{"question":"How does retry logic work?"}'
```

**Response**:
```json
{
  "answer": "Based on [internal/retry/retry.go:45]:\n\n```go\nfunc RetryWithBackoff(fn func() error, maxRetries int) error {\n    for i := 0; i < maxRetries; i++ {\n        if err := fn(); err == nil {\n            return nil\n        }\n        time.Sleep(backoff(i))\n    }\n    return ErrMaxRetriesExceeded\n}\n```\n\nThe system uses exponential backoff...",
  "repo": "my-backend",
  "model": "claude-sonnet-4-5-20250929",
  "usage": {
    "input_tokens": 15420,
    "output_tokens": 1893,
    "cached_tokens": 13200
  }
}
```

### List Repositories

**Request**:
```bash
curl http://localhost:9000/repos | jq
```

**Response**:
```json
{
  "repos": [
    {
      "name": "my-backend",
      "path": "/repos/my-backend",
      "branch": "main",
      "indexed_at": "2025-12-28T10:30:00Z",
      "file_count": 247,
      "commit_sha": "abc123..."
    }
  ],
  "count": 1
}
```

---

## Development

### Build

```bash
# Install dependencies
go mod download

# Build all binaries
make build-all

# Build specific binaries
make build                # mesh-agent (gateway/HTTP/MCP)
make build-mcp-bridge     # MCP bridge for Claude Code
make build-indexer        # Standalone indexer
```

### Run Locally

```bash
# Gateway mode (multi-repo)
make run-gateway

# HTTP mode (single repo)
make run

# MCP mode (Claude Code integration)
make run-mcp
```

### Test

```bash
# All tests
make test

# With coverage
make test-coverage

# With race detection
make test-race

# Specific package
go test ./internal/vectorstore/...
```

### Docker

```bash
# Build and start
make docker-up

# View logs
make docker-logs

# Stop
make docker-down

# Rebuild
make docker-build
```

### Extend

**Add new embedding provider**:
Implement `EmbeddingProvider` interface in `internal/vectorstore/`:

```go
type EmbeddingProvider interface {
    CreateEmbedding(ctx context.Context, text string) ([]float32, error)
    GetModelName() string
    GetDimensions() int
}
```

**Add new LLM provider**:
Implement `LLMProvider` interface in `internal/llm/`:

```go
type LLMProvider interface {
    Ask(systemPrompt, userPrompt string) (Response, error)
    AskWithCache(systemPrompt, cacheableContext, regularContext, question string) (Response, error)
    SupportsPromptCaching() bool
    CountTokens(text string) int
}
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Branch Management

MESH automatically tracks the current git branch for each repository:

```bash
# Automatically indexes current branch on startup
docker compose up -d

# Switch branch and re-index
cd /repos/my-backend
git checkout develop
curl -X POST http://localhost:9000/repos/my-backend/reindex

# Auto re-indexing every 5 minutes via BranchScanner
# Or trigger via GitHub webhook on push
```

Collections are branch-specific: `mesh-{repo}-{branch}-v1`

---

## Performance

**Indexing** (typical 250-file repo):
- Initial: 2-6 minutes
- Incremental: 1-5 seconds (only changed files)
- Workers: 2-6 parallel (adaptive)

**Query**:
- Semantic search: 50-100ms
- Total latency: 2-15 seconds (including LLM)

**Resource Usage** (tested: 9 repos, 2000 files):
- Memory: 1.2GB (gateway 600MB + Qdrant 600MB)
- Disk: ~5MB per 1000 files

**Requirements**:
- Minimum: 2GB RAM, 2 CPU cores, 10GB disk
- Recommended: 4GB RAM, 4 CPU cores, 20GB SSD
- With local Ollama LLM: 32GB+ RAM, GPU with 24GB+ VRAM

---

## Technology Stack

- **Go**: 1.24.1
- **Qdrant**: Vector database (HNSW indexing)
- **Anthropic SDK**: Claude API with prompt caching
- **Ollama**: Local embeddings and LLM
- **OpenAI SDK**: Cloud embeddings and LLM
- **Gin**: HTTP framework
- **MCP SDK**: Model Context Protocol
- **Zerolog**: Structured logging

See [go.mod](go.mod) for full dependency list.

---

## MCP Integration (Claude Code)

```bash
# Build bridge
make build-mcp-bridge

# Add to Claude Code
~/.local/bin/claude mcp add mesh-gateway \
  --transport stdio \
  -- /path/to/mesh/mesh-mcp-bridge \
     --agent-url=http://localhost:9000 \
     --gateway

# Query from Claude Code
@ask_my_backend How does authentication work?
```

---

## GitHub Webhook Auto Re-indexing

1. GitHub Settings → Webhooks → Add webhook
   - Payload URL: `http://your-server:9000/webhooks/github`
   - Content type: `application/json`
   - Events: Just the push event

2. Ensure repo names match `repos.yaml` configuration

3. Push code → automatic re-indexing

---

## Security

- **Local embeddings**: Ollama keeps data on-premise
- **Read-only mounts**: MESH cannot modify code
- **API keys**: Environment variables, not committed
- **Network**: Use reverse proxy (nginx/Caddy) for production TLS

---

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE)

---

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - Technical architecture and design
- [doc/INDEXING.md](doc/INDEXING.md) - Vector indexing deep dive (chunking, embeddings, HNSW)
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines

---

**MESH**: Accurate code understanding for AI-assisted development.
