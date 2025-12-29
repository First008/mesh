# Vector Indexing Architecture

This document explains how MESH transforms source code into searchable vector embeddings.

## Overview

MESH uses **semantic vector search** to find relevant code. Instead of keyword matching, it understands the *meaning* of code through high-dimensional vector representations (embeddings).

```
Source Code → Chunking → Embedding → Vector DB → Semantic Search
```

---

## Phase 1: File Discovery & Filtering

### Directory Traversal

**Code**: `internal/vectorstore/indexer.go:100-158`

The indexer walks the repository tree and applies multiple filters:

```go
filepath.Walk(repoPath, func(path, info, err) {
    // 1. Skip unwanted directories
    if filetypes.ShouldSkipDirectory(info.Name()) {
        return filepath.SkipDir
    }

    // 2. Filter by file extension
    if !isCodeFile(path) {
        return nil
    }

    // 3. Read file content
    content := os.ReadFile(path)

    // 4. SHA256 hash for change detection
    currentHash := sha256(content)
    if previousHash == currentHash {
        return nil  // Skip unchanged files
    }

    // 5. Skip huge files
    if len(content) > 500_000 {  // 500KB limit
        return nil
    }

    // 6. Add to indexing queue
    filesToIndex = append(filesToIndex, IndexJob{
        RelPath: relPath,
        Content: string(content),
    })
})
```

### Skipped Directories

**Defined in**: `internal/filetypes/registry.go:143-176`

```
.git, .svn, .hg          # Version control
node_modules, vendor      # Dependencies
dist, build, target       # Build artifacts
__pycache__, .venv        # Python artifacts
.idea, .vscode            # IDE files
```

### Indexed File Types

**Defined in**: `internal/filetypes/registry.go:14-76`

```
Code:        .go, .ts, .js, .py, .java, .rs, .c, .cpp, .cs, .rb, .php, .swift
Schema:      .proto, .sql
Docs:        .md, .rst
Config:      (indexed but filtered at search time - see exclude_patterns)
```

---

## Phase 2: Intelligent Chunking

### Why Chunking?

Embedding models have context limits:
- **bge-m3**: 8192 tokens (~32KB)
- **nomic-embed-text**: 2048 tokens (~8KB)

Large files must be split into chunks that fit within these limits.

### Token Budget Calculation

**Code**: `internal/vectorstore/chunker.go:28-37`

```go
const (
    MaxTokensPerChunk    = 3500  // Safe chunk size
    OverlapTokens        = 250   // Overlap between chunks
    MaxTokensWholeFile   = 3200  // Embed whole if under this
    CharsPerTokenEstimate = 4.0  // ~4 chars per token
)

func estimateTokens(text string) int {
    return int(ceil(len(text) / 4.0))
}
```

### Chunking Decision Tree

**Code**: `internal/vectorstore/chunker.go:39-80`

```
File size?
│
├─ < 3200 tokens (12.8KB)
│   └─ Index as single chunk ✓
│
└─ >= 3200 tokens
    └─ Language-aware chunking:
        ├─ Go: chunkGoCode()
        │   └─ Split at function/type boundaries
        │
        ├─ TypeScript: chunkTSCode()
        │   └─ Split at class/export boundaries
        │
        └─ Other: chunkByLines()
            └─ Simple line-based splitting
```

### Example: Go Function Chunking

**Input file** (8000 tokens):
```go
package service

type Processor struct {
    workers int
    timeout time.Duration
}

func (p *Processor) Process(ctx context.Context) error {
    // ... 200 lines of code ...
}

func (p *Processor) cleanup() error {
    // ... 100 lines of code ...
}
```

**Output chunks**:
```
Chunk 0 (lines 1-150):
  type Processor struct { ... }
  func (p *Processor) Process(...) { ... }

Chunk 1 (lines 140-250):  ← Note: 10-line overlap (lines 140-150)
  func (p *Processor) Process(...) { ... }  ← Partial overlap
  func (p *Processor) cleanup() { ... }
```

### Chunk Metadata Structure

```go
type CodeChunk struct {
    Content    string  // Actual code
    ChunkIndex int     // Position in file (0, 1, 2, ...)
    StartLine  int     // Starting line number
    EndLine    int     // Ending line number
    Header     string  // "pkg/service/processor.go (lines 42-87)"
    ChunkID    string  // SHA256(path + startLine + endLine)
}
```

**Why overlap?**
- Prevents context loss at chunk boundaries
- A function spanning two chunks will appear (partially) in both
- Improves retrieval accuracy

---

## Phase 3: Parallel Processing

### Worker Pool Architecture

**Code**: `internal/vectorstore/indexer.go:175-237`

```
Job Queue (buffered channel)
     │
     ├──→ Worker 1 ──→ Chunk → Embed → Store
     ├──→ Worker 2 ──→ Chunk → Embed → Store
     ├──→ Worker 3 ──→ Chunk → Embed → Store
     ├──→ Worker 4 ──→ Chunk → Embed → Store
     └──→ Worker N ──→ Chunk → Embed → Store
           (N = runtime.NumCPU())
```

**Implementation**:
```go
workerCount := runtime.NumCPU()  // 12-16 on M3 Max
jobsChan := make(chan IndexJob, len(jobs))

// Start workers
for w := 0; w < workerCount; w++ {
    go func(workerID int) {
        for job := range jobsChan {
            chunks := ChunkFile(job.RelPath, job.Content)
            for _, chunk := range chunks {
                embedding := createEmbedding(chunk.Content)
                store.Upsert(chunkPath, embedding, chunk)
            }
        }
    }(w)
}

// Feed jobs to workers
for _, job := range jobs {
    jobsChan <- job
}
```

**Performance**: With 12 workers and 250 files:
- Sequential: ~25 minutes (6s/file × 250)
- Parallel: ~2 minutes (250 files / 12 workers × 6s)

---

## Phase 4: Vector Embedding Generation

### Ollama API Call

**Code**: `internal/vectorstore/ollama.go:72-118`

```go
func CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
    // Timeout protection
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // API request
    req := &api.EmbedRequest{
        Model: "bge-m3",
        Input: text,  // Code chunk (~14KB)
    }

    resp, err := client.Embed(ctx, req)

    // Convert float64 → float32
    embedding := make([]float32, len(resp.Embeddings[0]))
    for i, v := range resp.Embeddings[0] {
        embedding[i] = float32(v)
    }

    return embedding  // [1024 floats]
}
```

### What Happens Inside the Model?

```
Input: "func Execute(ctx context.Context) error { ... }"
   ↓
┌─────────────────────────────────────────┐
│ 1. Tokenization                         │
│    ["func", "Execute", "(", "ctx", ...] │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│ 2. Transformer Layers (12-24 layers)    │
│    - Self-attention                     │
│    - Feed-forward networks              │
│    - Layer normalization                │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│ 3. Pooling (mean/max/CLS token)        │
│    Aggregate token vectors → 1 vector   │
└─────────────┬───────────────────────────┘
              ↓
Output: [0.234, -0.891, 0.456, ..., 0.123]
        ↑
        1024-dimensional vector
```

### What Does Each Dimension Represent?

The 1024 dimensions capture **semantic features**:

- **Syntactic patterns**: function definitions, control flow, data structures
- **Naming conventions**: camelCase, snake_case, domain terms
- **Code structure**: nesting, indentation, brackets
- **Domain concepts**: authentication, database, API, processing, etc.
- **Language features**: async/await, generics, error handling

**Example**: Similar vectors for semantically similar code
```
Vector("func Execute(ctx) error { ... }")
    ≈ 0.89 similarity
Vector("func Run(context) error { ... }")

Vector("func Execute(ctx) error { ... }")
    ≈ 0.23 similarity
Vector("func calculateSum(a, b int) int { ... }")
```

### Model Performance

| Model | Context | Dimensions | Speed (M3 Max) | Best For |
|-------|---------|------------|----------------|----------|
| **bge-m3** | 8K tokens | 1024 | ~6s | Production (recommended) |
| nomic-embed-text | 2K tokens | 768 | ~2-3s | Quick testing |
| mxbai-embed-large | 512 tokens | 1024 | ~3-4s | Speed priority |

---

## Phase 5: Qdrant Vector Storage

### Collection Creation

**Code**: `internal/vectorstore/qdrant.go:81-121`

```go
client.CreateCollection(ctx, &qdrant.CreateCollection{
    CollectionName: "backend-service_main",  // {repo}_{branch}

    VectorsConfig: &qdrant.VectorParams{
        Size:     1024,                    // bge-m3 dimensions
        Distance: qdrant.Distance_Cosine,  // Similarity metric

        HnswConfig: &qdrant.HnswConfigDiff{
            M:           16,   // Graph connectivity
            EfConstruct: 128,  // Index quality
        },
    },
})
```

### HNSW Index Explained

**Hierarchical Navigable Small World Graph** enables O(log N) search instead of O(N).

```
Layer 2 (coarse, long jumps)
    ●───────●───────●

Layer 1 (medium jumps)
    ●───●───●───●───●───●

Layer 0 (fine, all vectors)
  ●─●─●─●─●─●─●─●─●─●─●─●─●─●

Search path: Start at top → Navigate down → Find neighbors
```

**Parameters**:
- **M=16**: Each node connects to 16 neighbors (balance between speed/accuracy)
- **EfConstruct=128**: Candidate list size during build (higher = better index)

### Distance Metric: Cosine Similarity

**Why Cosine instead of Euclidean?**

Cosine measures *angle* between vectors, not magnitude:

```
Vector A: [0.8, 0.6]
Vector B: [0.4, 0.3]  ← Half the magnitude, same direction

Euclidean distance:  0.50 (considers magnitude)
Cosine similarity:   0.97 (ignores magnitude) ✓

Good for text/code where magnitude doesn't matter
```

**Formula**:
```
similarity = dot(A, B) / (||A|| × ||B||)
           = cos(θ)
```

### Point Storage Structure

**Code**: `internal/vectorstore/qdrant.go:124-168`

```go
client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: "backend-service_main",
    Points: []*qdrant.PointStruct{
        {
            // Deterministic ID from file path
            Id: generatePointID("pkg/service/processor.go#chunk0"),

            // 1024-dim vector
            Vectors: [0.234, -0.891, 0.456, ...],

            // Metadata payload
            Payload: {
                "file_path":   "pkg/service/processor.go#chunk0",
                "base_path":   "pkg/service/processor.go",
                "content":     "func Process(ctx...) { ... }",
                "file_hash":   "a3f2b1c...",
                "language":    "go",
                "chunk_index": 0,
            },
        },
    },
})
```

### Deterministic Point IDs

**Code**: `internal/vectorstore/qdrant.go:807-817`

```go
func generatePointID(filePath string) uint64 {
    // Hash the file path
    hash := sha256([]byte(filePath))

    // Take first 8 bytes as uint64
    id := hash[0] | hash[1]<<8 | hash[2]<<16 | ...

    return id
}
```

**Why deterministic?**
- Same file path → Same ID
- **Upsert** replaces old version (not duplicate)
- Essential for incremental updates

---

## Phase 6: Incremental Indexing

### Metadata Tracking

**Path**: `.mesh/{repo}/{branch}/metadata.json`

```json
{
  "repo_name": "backend-service",
  "branch": "main",
  "commit_sha": "5c2d1e7a3b4f...",
  "indexed_at": "2025-12-29T13:45:00Z",
  "file_count": 248
}
```

### Incremental Logic

**Code**: `internal/vectorstore/indexer.go:284-356`

```go
// 1. Get current commit
currentCommit := git.GetCommit("main")  // "7a8b9c2..."

// 2. Load metadata
meta := LoadMetadata("backend-service", "main")

// 3. Check if re-indexing needed
if meta.CommitSHA == currentCommit {
    return  // No changes, skip indexing ✓
}

// 4. Get changed files since last indexed commit
changedFiles := git.Diff(meta.CommitSHA, currentCommit)
// Returns: ["pkg/service/processor.go", "web/handler/auth.go"]

// 5. Only index changed files (NOT all 248 files!)
for _, file := range changedFiles {
    chunks := ChunkFile(file)
    for _, chunk := range chunks {
        embedding := CreateEmbedding(chunk)
        store.Upsert(chunkPath, embedding, chunk)
    }
}

// 6. Update metadata
SaveMetadata(&BranchMetadata{
    CommitSHA:  currentCommit,
    IndexedAt:  time.Now(),
    FileCount:  248,
})
```

**Performance gain**:
- Full indexing: 250 files × 6s = 25 minutes
- Incremental (2 changed files): 2 files × 6s = 12 seconds
- **~125x faster** for typical commits

---

## Complete Flow Diagram

```
┌─────────────────────────────────────────┐
│ 1. File Discovery                       │
│    • Walk repo tree                     │
│    • Filter extensions (.go, .ts, ...)  │
│    • SHA256 hash check                  │
│    • Skip >500KB files                  │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 2. Intelligent Chunking                 │
│    • If file < 3200 tokens → 1 chunk    │
│    • Else: language-aware splitting     │
│      - Go: function boundaries          │
│      - TS: class boundaries             │
│      - Other: line-based                │
│    • 250-token overlap                  │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 3. Parallel Workers (N = NumCPU)       │
│    • Buffered job channel               │
│    • 12-16 workers on M3 Max            │
│    • ~12x speedup vs sequential         │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 4. Embedding Generation (per chunk)    │
│    • HTTP POST to Ollama (bge-m3)      │
│    • Input: ~3500 tokens max            │
│    • Output: 1024-dim vector            │
│    • Duration: ~6 seconds/chunk         │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 5. Qdrant Storage                       │
│    • Deterministic ID from SHA256(path) │
│    • Vector: [1024 floats]              │
│    • Payload: {content, language, ...}  │
│    • HNSW index for O(log N) search     │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 6. Metadata Update                      │
│    • Save commit SHA                    │
│    • Save timestamp                     │
│    • .mesh/{repo}/{branch}/metadata.json│
└─────────────────────────────────────────┘
```

---

## Search Time (Query Flow)

**Code**: `internal/vectorstore/qdrant.go:645-695`

```
User Question: "How does data processing work?"
    ↓
1. Embed question (bge-m3)
    → [0.123, 0.456, -0.789, ...]
    ↓
2. Qdrant similarity search
    → Top 50 chunks by cosine similarity
    ↓
3. Group chunks by base_path
    → Combine chunks from same file
    ↓
4. Hybrid scoring (4 signals):
    • Semantic:  70% (vector similarity)
    • Keyword:   15% (exact matches)
    • Path:       5% (focus_paths boost)
    • Aggregate: 10% (multi-chunk depth)
    ↓
5. File reconstruction
    → Sort chunks by index, combine content
    ↓
6. POST-FILTER: Apply exclude_patterns
    → Remove *.yaml, *.json, etc.
    ↓
7. Token budget selection
    → Top 15 files, max 80K tokens
    ↓
8. Return to LLM
    → Complete files as context
```

---

## Branch Isolation

### Why Separate Collections?

Each branch gets its own Qdrant collection:

```
Collections in Qdrant:
├── backend-service_main    (main branch)
├── backend-service_dev     (dev branch)
├── frontend-app_qa         (qa branch)
└── api-gateway_master      (master branch)
```

**Benefits**:
1. **No mixing**: Dev code doesn't pollute production search
2. **Easy cleanup**: Delete collection to remove branch
3. **Parallel indexing**: Index multiple branches simultaneously
4. **Version isolation**: Different API versions stay separate

**Cost**: Storage overhead (~1GB per branch for large repos)

---

## Performance Characteristics

### Indexing Speed

| Phase | Time per File | Parallelizable? |
|-------|---------------|-----------------|
| File reading | 1ms | ✓ Yes |
| Chunking | 5-10ms | ✓ Yes |
| Embedding (bge-m3) | 6000ms | ✓ Yes (GPU) |
| Qdrant upsert | 10ms | ✓ Yes |
| **Total** | **~6 seconds** | **✓ 12x with 12 workers** |

**Full repo indexing** (250 files):
- Sequential: 25 minutes
- Parallel (12 workers): 2 minutes

### Search Speed

| Operation | Time | Complexity |
|-----------|------|------------|
| Embed query | ~100ms | O(1) |
| HNSW search | ~5ms | O(log N) |
| Chunk grouping | ~2ms | O(K) |
| File reconstruction | ~5ms | O(K) |
| **Total** | **~112ms** | **O(log N)** |

Where:
- N = total vectors (10K-100K)
- K = retrieved chunks (50-100)

---

## Configuration

### repos.yaml

```yaml
embedding_model: "bge-m3"           # nomic-embed-text, mxbai-embed-large
ollama_url: "http://host.docker.internal:11434"

repos:
  - name: backend-service
    path: /repos/backend-service
    focus_paths:                     # Boost these in search
      - pkg/core/**
      - pkg/service/**
    exclude_patterns:                # Filter at search time
      - "*.yaml"
      - "*.json"
      - "docker-compose*"
```

### Tuning Parameters

**In code** (`internal/vectorstore/chunker.go`):
```go
MaxTokensPerChunk    = 3500  // Increase for larger chunks
OverlapTokens        = 250   // Increase for better context
MaxTokensWholeFile   = 3200  // Increase to embed more files whole
```

**In code** (`internal/vectorstore/search_config.go`):
```go
InitialChunkLimit: 50,    // Fetch more/fewer initial results
MaxFilesLimit:     15,    // Return more/fewer files to LLM
SemanticWeight:    0.70,  // Adjust hybrid scoring weights
```

---

## Troubleshooting

### Issue: Indexing Takes Forever

**Symptom**: Indexing a 500-file repo takes 40+ minutes

**Diagnosis**:
```bash
# Check if parallel workers are active
docker logs mesh-gateway | grep "Starting parallel indexing"
# Should show: "workers": 12
```

**Fix**: Ensure `runtime.NumCPU()` returns correct value in Docker

---

### Issue: Out of Memory During Indexing

**Symptom**: Docker container killed, OOMKilled in logs

**Diagnosis**: Too many large files being processed simultaneously

**Fix**: Reduce worker count:
```go
// internal/vectorstore/config.go
func GetWorkerCount() int {
    return min(runtime.NumCPU(), 8)  // Cap at 8 workers
}
```

---

### Issue: Search Returns Irrelevant Results

**Symptom**: Question about "authentication" returns database migration files

**Diagnosis**: Config files polluting search results

**Fix**: Add `exclude_patterns` to `repos.yaml`:
```yaml
exclude_patterns:
  - "*.yaml"
  - "*.json"
  - "*.sql"
  - "migrations/**"
```

---

### Issue: Embeddings Fail with Timeout

**Symptom**: `ollama embedding error: context deadline exceeded`

**Diagnosis**: Files exceed model context window

**Fix**: Reduce `MaxTokensPerChunk`:
```go
// internal/vectorstore/chunker.go
MaxTokensPerChunk = 2000  // Down from 3500
```

---

## Advanced Topics

### Custom Embedding Models

To add a new model to Ollama:

```bash
# 1. Pull model
ollama pull all-minilm

# 2. Update dimension mapping
# internal/vectorstore/ollama.go:123-139
func (o *OllamaEmbeddingProvider) GetDimensions() int {
    switch o.model {
    case "all-minilm":
        return 384  // Add your model dimensions
    default:
        return 1024
    }
}

# 3. Update config
# configs/repos.yaml
embedding_model: "all-minilm"
```

### Hybrid Search Tuning

Adjust weights in `internal/vectorstore/search_config.go`:

```go
// Semantic-heavy (default)
SemanticWeight:  0.70,
KeywordWeight:   0.15,
PathWeight:      0.05,
AggregateWeight: 0.10,

// Keyword-heavy (exact matches matter)
SemanticWeight:  0.50,
KeywordWeight:   0.35,
PathWeight:      0.05,
AggregateWeight: 0.10,

// Path-heavy (respect focus_paths)
SemanticWeight:  0.60,
KeywordWeight:   0.15,
PathWeight:      0.15,
AggregateWeight: 0.10,
```

### Re-indexing Strategy

**Full re-index** (force):
```bash
# Delete metadata
rm -rf .mesh/

# Restart gateway (will trigger full indexing)
docker compose restart mesh-gateway
```

**Incremental re-index** (automatic):
- Happens on every startup if commit SHA changed
- Typically indexes only 2-5 files per commit

---

## References

- Qdrant documentation: https://qdrant.tech/documentation/
- bge-m3 model: https://huggingface.co/BAAI/bge-m3
- HNSW algorithm: https://arxiv.org/abs/1603.09320
- Ollama API: https://github.com/ollama/ollama/blob/main/docs/api.md
