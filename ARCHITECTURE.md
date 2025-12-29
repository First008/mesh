# MESH Architecture

**Multi-repository AI Code Assistant with Semantic Search**

This document provides a comprehensive architectural overview of MESH, including system design, component interactions, data flows, and design principles.

---

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Core Components](#core-components)
- [Data Flows](#data-flows)
- [Design Principles](#design-principles)
- [Technology Stack](#technology-stack)
- [Deployment Architecture](#deployment-architecture)

---

## Overview

MESH is an intelligent code understanding gateway that enables semantic search and AI-powered code comprehension across multiple repositories. It combines vector embeddings, LLM integration, and branch-aware indexing to provide accurate, context-based answers about codebases.

### Key Capabilities

- **Semantic Code Search**: Vector embeddings for intelligent code discovery
- **Multi-Repository Management**: Single gateway for all repositories
- **Branch-Aware Indexing**: Separate indices per git branch
- **Cost Optimization**: Prompt caching and incremental indexing
- **Universal Integration**: REST API, MCP Protocol, GitHub Webhooks

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     MESH Gateway (Single Container)              │
│                                                                   │
│  ┌────────────────┐    ┌──────────────────────┐                 │
│  │  HTTP Server   │    │  Branch Scanner      │                 │
│  │  Port 9000     │    │  Auto Re-index       │                 │
│  └────────┬───────┘    └──────────────────────┘                 │
│           │                                                       │
│  ┌────────▼──────────────────────────────────────────────────┐  │
│  │              Repository Agents (Per Repo)                 │  │
│  │  ┌──────────────────┐  ┌──────────────────┐              │  │
│  │  │  backend-api     │  │  frontend-app    │              │  │
│  │  │  ├─ LLM Client   │  │  ├─ LLM Client   │    ... more  │  │
│  │  │  ├─ Vector Store │  │  ├─ Vector Store │              │  │
│  │  │  └─ Context AI   │  │  └─ Context AI   │              │  │
│  │  └──────────────────┘  └──────────────────┘              │  │
│  └───────────────────────────────────────────────────────────┘  │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
              ┌─────────────────────────────────┐
              │  Qdrant Vector Database         │
              │  (Branch-Aware Collections)     │
              │                                 │
              │  ┌────────────────────────────┐ │
              │  │ mesh-backend-main-v1       │ │
              │  │ - 247 files, 1091 chunks   │ │
              │  ├────────────────────────────┤ │
              │  │ mesh-backend-staging-v1    │ │
              │  │ - 250 files, 1105 chunks   │ │
              │  ├────────────────────────────┤ │
              │  │ mesh-frontend-develop-v1   │ │
              │  │ - 189 files, 823 chunks    │ │
              │  └────────────────────────────┘ │
              └─────────────────────────────────┘
```

### Architecture

**Gateway Mode** (Multi-Repository):
- Manages multiple repositories with branch-aware indexing
- Configuration: `configs/repos.yaml`
- Single HTTP server routing to multiple agents
- Periodic branch scanning for automatic re-indexing
- REST API and MCP protocol support

---

## Core Components

### 1. Gateway (`internal/gateway`)

**Purpose**: Orchestrates multiple repository agents as a unified service.

**Key Responsibilities**:
- Initialize and manage repository agents
- Route queries to appropriate agents
- Aggregate responses from multiple repositories
- Monitor git branches for changes
- Trigger incremental re-indexing

**Key Components**:
- **Gateway**: Main coordinator managing agent lifecycle
- **BranchScanner**: Periodic branch monitoring (5-minute intervals)
- **Config**: Gateway configuration with repository definitions

**API Methods**:
```go
Ask(repoName, question) → Response
AskAll(question) → []Response
ListRepos() → []RepoInfo
ReindexRepo(repoName) → IndexStats
```

---

### 2. Agent (`internal/agent`)

**Purpose**: Represents a single repository AI agent that answers questions about one codebase.

**Key Responsibilities**:
- Manage LLM provider, context builder, cost tracking
- Orchestrate question answering workflow
- Build layered context (cacheable + dynamic)
- Call LLM with context
- Track API costs

**Key Components**:
- **Agent**: Main agent orchestrator
- **Config**: Repository-specific configuration
- **Personality**: Customizable agent behavior and expertise

**Context Layers**:
1. **Cacheable (Static)**: README.md, CLAUDE.md, repo structure
2. **Dynamic**: Semantic search results, relevant code files

---

### 3. Vector Store (`internal/vectorstore`)

**Purpose**: Semantic code search using vector embeddings with Qdrant.

**Key Responsibilities**:
- Index code files into vector database
- Perform semantic search for relevant code
- Chunk large files intelligently
- Detect file changes for incremental updates
- Support multiple embedding models

**Key Components**:

#### Indexer (`indexer.go`)
- Parallel indexing engine with worker pools (2-6 workers)
- Incremental indexing (only changed files)
- SHA256-based change detection
- Statistics tracking (indexed, skipped, errors)

#### Chunker (`chunker.go`)
- Token-aware code splitting (max 3500 tokens/chunk)
- Syntax-aware splitting for Go, TypeScript, JavaScript
- Overlap: 250 tokens for context continuity
- Whole file threshold: 3200 tokens

#### QdrantStore (`qdrant.go`)
- Branch-aware collection naming: `mesh-{repo}-{branch}-v1`
- Chunk aggregation to reconstruct complete files
- HNSW index configuration (M=16, EfConstruct=128)
- Cosine distance metric

#### Embedding Providers
- **OllamaEmbeddingProvider**: Local embeddings (bge-m3, nomic-embed-text)
- **OpenAIEmbeddingProvider**: Cloud embeddings (text-embedding-3-small/large)

---

### 4. Context Builder (`internal/context`)

**Purpose**: Build layered context from repository for LLM prompts.

**Key Responsibilities**:
- Load static context (README, CLAUDE.md, repo structure)
- Perform semantic search for relevant files
- Aggregate chunked results into complete files
- Build layered context for prompt caching

**Context Building Flow**:
1. Load cacheable context (README.md, CLAUDE.md)
2. Perform semantic search with query
3. Aggregate chunks into complete files (top 10)
4. Format context for LLM consumption

---

### 5. LLM Providers (`internal/llm`)

**Purpose**: Unified interface for multiple AI providers with prompt caching.

**Supported Providers**:
- **Anthropic**: Claude Sonnet 4.5, supports prompt caching (90% cost reduction)
- **OpenAI**: GPT-4 Turbo, no caching support
- **Ollama**: Local models (llama3.3:70b, deepseek-coder-v2, qwen2.5-coder)

**Interface**:
```go
type LLMProvider interface {
    Ask(systemPrompt, userPrompt) → Response
    AskWithCache(systemPrompt, cacheableContext, regularContext, question) → Response
    SupportsPromptCaching() → bool
    CountTokens(text) → int
}
```

---

### 6. HTTP Server (`internal/server`)

**Purpose**: REST API endpoints for querying repositories.

**Key Endpoints**:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Agent/Gateway information |
| `/metrics` | GET | Usage statistics |
| `/ask` | POST | Ask single-repo agent |
| `/ask/:repo` | POST | Ask specific repo in gateway |
| `/ask-all` | POST | Ask all repositories |
| `/repos` | GET | List all repositories |
| `/repos/:repo/reindex` | POST | Trigger re-indexing |
| `/webhooks/github` | POST | GitHub webhook receiver |

---

### 7. Cost Tracking (`pkg/telemetry`)

**Purpose**: API cost tracking with daily limits and budgets.

**Key Features**:
- Per-model pricing (Anthropic, OpenAI)
- 90% discount for cached tokens (Anthropic)
- Daily maximum USD spend limits
- Per-query token limit
- Daily tracking with auto-reset at midnight

**Anthropic Pricing** (as of Dec 2025):
- **Sonnet 4.5**: $3/1M input, $15/1M output
- **Opus 4.5**: $5/1M input, $25/1M output
- **Haiku 3.5**: $0.80/1M input, $4/1M output

---

### 8. MCP Server (`internal/mcp`)

**Purpose**: Model Context Protocol server for Claude Code integration.

**Key Features**:
- Registers tools for each repository
- Forwards MCP calls to HTTP agent
- Supports both single-repo and gateway modes
- Dynamic tool registration based on configured repositories

---

### 9. Factory (`internal/factory`)

**Purpose**: Centralized provider creation to eliminate duplication.

**Key Methods**:
```go
NewEmbeddingProvider(config) → EmbeddingProvider
```

**Benefits**:
- Eliminates code duplication
- Consistent provider initialization
- Easier to add new providers

---

### 10. File Types (`internal/filetypes`)

**Purpose**: Unified file type detection and classification.

**Supported Languages** (24+ file types):
- **Code**: Go, TypeScript, JavaScript, Python, Java, Rust, C/C++, C#, Ruby, PHP, Swift, Kotlin, Scala
- **Shell**: bash, sh, zsh
- **Data/Config**: Proto, SQL, YAML, JSON, TOML, XML
- **Documentation**: Markdown, RST

**Responsibilities**:
- File extension detection
- Skip unnecessary files/directories (node_modules, vendor, .git)
- Language identification for syntax-aware chunking

---

## Data Flows

### Query Processing Flow

```
User Question
    ↓
HTTP Server / MCP Server
    ↓
Gateway.Ask(repoName, question)
    ↓
Agent.Ask(context, question)
    ├─ contextBuilder.BuildContextLayers(question)
    │  ├─ Load README, CLAUDE.md (cacheable)
    │  └─ vectorStore.SearchWithAggregation(question, 10)
    │     ├─ Search for relevant files (limit: 50 results)
    │     ├─ Group by file path
    │     ├─ Fetch ALL chunks per file
    │     └─ Return complete files (no fragments)
    │
    ├─ Get system prompt (personality)
    │
    └─ llmProvider.AskWithCache() or Ask()
       ├─ Input: systemPrompt + cacheableContext + regularContext + question
       └─ Output: Response{Content, InputTokens, OutputTokens, CachedTokens, Model}

    ↓
costTracker.RecordRequest(model, tokens)
    ↓
Return JSON response with usage stats
```

### Indexing Flow

```
Repository Files
    ↓
Indexer.IndexIncremental()
    ├─ Walk repository tree
    ├─ Filter code files (filetypes.Extensions)
    ├─ Detect changes (SHA256 hashing)
    └─ For unchanged files: skip

    ↓ For changed files
    │
    ├─ chunker.ChunkFile(filePath, content, language)
    │  ├─ If tokens ≤ 3200: whole file chunk
    │  └─ Else: syntax-aware chunking
    │     ├─ Go: split at func, type, const, var
    │     ├─ TypeScript: split at export, class, function, interface
    │     └─ Other: line-based with overlap
    │
    ├─ Parallel worker pool (2-6 workers)
    │  └─ For each chunk: embeddingProvider.CreateEmbedding()
    │
    └─ vectorStore.IndexFile(filePath, content)
       └─ Upsert to Qdrant with metadata

    ↓
Save metadata (.mesh/{repo}/{branch}/metadata.json)
    └─ Track commit SHA, file hashes for incremental updates
```

### Branch Management Flow

```
Gateway initialization
    ↓
For each repository:
    ├─ Detect current branch (git)
    ├─ Create collection: mesh-{repo}-{branch}-v1
    └─ Index files incrementally

    ↓ Every 5 minutes (BranchScanner)
    │
    ├─ Check current branch
    ├─ If branch changed:
    │  ├─ Reindex new branch
    │  └─ Maintain separate collections
    │
    └─ If commits changed:
       └─ Incremental re-index
```

---

## Design Principles

MESH follows clean architecture principles with emphasis on simplicity and maintainability.

### From CLAUDE.md (Project Guidelines)

1. **ETC (Easier To Change)**: Design for flexibility by minimizing interdependencies
2. **Eliminate Side Effects**: Write "shy" code where modules expose only what's necessary
3. **Minimize Coupling**: Fewer interdependencies lead to easier, isolated changes
4. **Tell, Don't Ask**: Avoid chaining method calls to keep interactions clear
5. **Prefer Interfaces**: Express polymorphism without inheritance tax
6. **Three Strikes, Then Refactor**: Technical debt awareness

### Architecture Patterns

- **Factory Pattern**: `internal/factory` centralizes provider creation
- **Adapter Pattern**: `LLMProvider` abstracts multiple AI providers
- **Strategy Pattern**: Embedding providers (Ollama, OpenAI)
- **Composition Over Inheritance**: Agent composes providers (no base classes)
- **Interface-based Design**: All major components implement interfaces

### Code Organization

```
mesh/
├── cmd/                          # Executable entry points
├── internal/                     # Private business logic
│   ├── agent/                   # Domain: Repository agent
│   ├── gateway/                 # Domain: Multi-repo orchestration
│   ├── vectorstore/             # Domain: Vector search & indexing
│   ├── llm/                     # Adapter: LLM providers
│   ├── context/                 # Service: Context building
│   ├── server/                  # Interface: HTTP API
│   ├── mcp/                     # Interface: MCP Protocol
│   ├── factory/                 # Factory: Provider creation
│   └── filetypes/               # Utility: File type detection
├── pkg/                          # Public packages
│   └── telemetry/               # Public: Cost tracking
└── configs/                      # Configuration files
```

**Design Rationale**:
- **Domain-Driven Structure**: Each package represents a clear domain boundary
- **Separation of Concerns**: Interfaces (HTTP, MCP) separate from business logic
- **Testability**: Easy to mock interfaces for unit tests
- **Encapsulation**: Internal packages prevent unintended dependencies

---

## Technology Stack

### Core Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **anthropics/anthropic-sdk-go** | v1.19.0 | Claude API client |
| **gin-gonic/gin** | v1.11.0 | HTTP framework |
| **modelcontextprotocol/go-sdk** | v1.2.0 | MCP protocol implementation |
| **qdrant/go-client** | v1.16.2 | Vector database client |
| **ollama/ollama** | v0.13.5 | Local embeddings |
| **openai/openai-go** | v1.12.0 | OpenAI API client |
| **rs/zerolog** | v1.34.0 | Structured logging |

### Infrastructure

- **Qdrant**: High-performance vector database
- **Docker**: Containerization and deployment
- **Git**: Branch detection and change tracking

---

## Deployment Architecture

### Docker Compose Setup

```yaml
services:
  mesh-gateway:
    image: mesh-gateway:latest
    ports:
      - "9000:9000"
    volumes:
      - /repos:/repos:ro              # Read-only repository mount
      - ./configs/repos.yaml:/config.yaml:ro
      - ./.env:/app/.env:ro
      - mesh-metadata:/app/.mesh      # Incremental indexing metadata
    environment:
      - MODE=gateway
      - LOG_LEVEL=info
    depends_on:
      - mesh-qdrant

  mesh-qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"  # HTTP API
      - "6334:6334"  # gRPC API
    volumes:
      - qdrant-data:/qdrant/storage   # Vector database persistence
```

### Resource Requirements

**Minimum**:
- 2GB RAM (gateway + Qdrant)
- 2 CPU cores
- 10GB disk (for Qdrant storage)

**Recommended**:
- 4GB RAM
- 4 CPU cores
- 20GB SSD

**With Local Ollama LLM** (llama3.3:70b):
- 32GB+ RAM
- 8+ CPU cores
- GPU with 24GB+ VRAM (optional but recommended)

### Scaling Characteristics

**Tested Configuration**:
- 9 repositories (~2000 files total)
- Memory: 1.2GB (gateway 600MB + Qdrant 600MB)
- Indexing: ~20 minutes initial, <10s incremental
- Query latency: <3 seconds average

**Performance Metrics**:
- **Indexing**: 4-10x faster with parallel workers
- **Incremental Updates**: 1-5 seconds (only changed files)
- **Semantic Search**: 50-100ms
- **Total Query Latency**: 2-15 seconds

---

## Key Features & Capabilities

### Semantic Code Search
- Vector embeddings for code understanding
- Chunk aggregation prevents fragmented context
- Token-aware chunking respects code structure
- Syntax-aware splitting for major languages

### Multi-Repository Management
- Single gateway for all repositories
- Per-repository agents (code reuse)
- Branch-aware indexing (separate collections)
- Automatic branch detection & tracking

### Cost Optimization
- Prompt caching (90% cost reduction with Anthropic)
- Incremental indexing (only changed files)
- Local embeddings option (zero API cost)
- Per-model pricing with daily budgets

### Integration Options
- REST API (any client)
- MCP Protocol (Claude Code)
- GitHub Webhooks (auto re-index on push)
- Backward compatible HTTP server

### Quality Assurance
- Context-only answer constraints (zero hallucinations)
- File:line references (traceability)
- Per-query cost tracking
- Daily spend limits with alerts

---

## Security & Privacy

### Data Privacy
- Local embeddings (Ollama) - no data sent to external APIs
- Read-only repository mounts - MESH cannot modify code
- No query logs persistence (unless explicitly enabled)

### API Key Management
- Environment variable injection
- `.env` file support (not committed to git)
- Separate keys per provider

### Network Security
- Health check endpoints for monitoring
- GitHub webhook validation (optional)
- Future: Authentication and API key management

---

## Testing Strategy

### Test Coverage

- **Unit Tests**: All core components (agent, context, gateway, vectorstore)
- **Integration Tests**: End-to-end workflows
- **Mock Providers**: Test utilities in `internal/testing/`

### Key Test Files
- `agent_test.go`: Agent integration tests
- `config_test.go`: Configuration parsing
- `context_builder_test.go`: Context building
- `indexer_test.go`: Indexing pipeline
- `chunker_test.go`: Code chunking logic
- `cost_tracker_test.go`: Cost calculations

### Test Utilities
- `internal/testing/mocks.go`: Mock LLM and embedding providers
- `internal/testing/fixtures.go`: Test data and fixtures

---

## Known Limitations

1. **Embedding Context Limits**:
   - bge-m3: 8K tokens (~32KB code)
   - nomic-embed-text: 2K tokens (~8KB code)
   - Very large files (500KB+) automatically skipped

2. **Branch Detection**:
   - Uses git working directory branch
   - No automatic branch selection from query content (yet)

3. **No Multi-Tenancy**:
   - Single gateway serves all users
   - No per-user access control (planned)

4. **Embedding Model Fixed per Deployment**:
   - Changing models requires re-indexing all repositories
   - Collections tied to specific dimensions

---

## Future Enhancements

### Planned Features
- File system watchers for instant re-indexing
- Web UI dashboard for queries and monitoring
- Per-query branch selection API
- Authentication and API key management
- Rate limiting and usage quotas
- Hybrid search (semantic + keyword)
- Query result caching

### Research Areas
- Advanced chunking strategies (semantic boundaries)
- Multi-modal search (code + comments + documentation)
- Code diff analysis
- Pull request context generation

---

## References

- [README.md](README.md) - User documentation and setup guide
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [LICENSE](LICENSE) - GNU General Public License v3.0
- [CLAUDE.md](.claude/CLAUDE.md) - Project coding principles

---

**Built with ❤️ for developers who want AI that actually understands their code**
