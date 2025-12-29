// Package vectorstore provides code indexing and vector storage operations.
//
// The vectorstore package implements semantic code search using vector embeddings,
// including parallel indexing, incremental updates, chunk-based document storage,
// and integration with Qdrant vector database.
package vectorstore

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/First008/mesh/internal/filetypes"
	"github.com/rs/zerolog"
)

// Indexer handles indexing of repository files into the vector store
type Indexer struct {
	store      VectorStore
	repoPath   string
	repoName   string
	branch     string
	fileHashes map[string]string // file path -> SHA256 hash
	mu         sync.RWMutex
	logger     zerolog.Logger
}

// IndexJob represents a file indexing job for the worker pool
type IndexJob struct {
	RelPath string
	Content string
}

// IndexStats tracks indexing statistics (thread-safe)
type IndexStats struct {
	Indexed int
	Skipped int
	Errors  int
	mu      sync.Mutex
}

func (s *IndexStats) incIndexed() {
	s.mu.Lock()
	s.Indexed++
	s.mu.Unlock()
}

func (s *IndexStats) incSkipped() {
	s.mu.Lock()
	s.Skipped++
	s.mu.Unlock()
}

func (s *IndexStats) incErrors() {
	s.mu.Lock()
	s.Errors++
	s.mu.Unlock()
}

func (s *IndexStats) get() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Indexed, s.Skipped, s.Errors
}

// NewIndexer creates a new indexer
func NewIndexer(store VectorStore, repoPath string, logger zerolog.Logger) *Indexer {
	return &Indexer{
		store:      store,
		repoPath:   repoPath,
		fileHashes: make(map[string]string),
		logger:     logger,
	}
}

// NewIndexerWithBranch creates a new indexer with branch awareness
func NewIndexerWithBranch(store VectorStore, repoPath, repoName, branch string, logger zerolog.Logger) *Indexer {
	return &Indexer{
		store:      store,
		repoPath:   repoPath,
		repoName:   repoName,
		branch:     branch,
		fileHashes: make(map[string]string),
		logger:     logger,
	}
}

// IndexRepository indexes all code files in the repository
// Uses incremental indexing - only re-indexes files that have changed
// Now with parallel workers and chunking support
func (idx *Indexer) IndexRepository(ctx context.Context) error {
	idx.logger.Info().Str("repo_path", idx.repoPath).Msg("Starting repository indexing")

	// Collect all files first
	var filesToIndex []IndexJob
	err := filepath.Walk(idx.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip common directories (delegates to filetypes)
			if filetypes.ShouldSkipDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !isCodeFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			idx.logger.Warn().Err(err).Str("path", path).Msg("Failed to read file")
			return nil
		}

		currentHash := computeFileHash(content)
		relPath, err := filepath.Rel(idx.repoPath, path)
		if err != nil {
			relPath = path
		}

		// Check if file has changed
		idx.mu.RLock()
		previousHash, exists := idx.fileHashes[relPath]
		idx.mu.RUnlock()

		if exists && previousHash == currentHash {
			return nil
		}

		// Skip extremely large files (>500KB)
		if len(content) > 500000 {
			idx.logger.Debug().
				Str("path", relPath).
				Int("size", len(content)).
				Msg("File too large, skipping (>500KB)")
			return nil
		}

		filesToIndex = append(filesToIndex, IndexJob{
			RelPath: relPath,
			Content: string(content),
		})

		// Update hash
		idx.mu.Lock()
		idx.fileHashes[relPath] = currentHash
		idx.mu.Unlock()

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk error: %w", err)
	}

	// Index files in parallel
	stats := idx.indexFilesParallel(ctx, filesToIndex)

	idx.logger.Info().
		Int("indexed", stats.Indexed).
		Int("skipped", stats.Skipped).
		Int("errors", stats.Errors).
		Msg("Repository indexing completed")

	return nil
}

// indexFilesParallel indexes files using a worker pool
func (idx *Indexer) indexFilesParallel(ctx context.Context, jobs []IndexJob) *IndexStats {
	stats := &IndexStats{}

	if len(jobs) == 0 {
		return stats
	}

	// Create job channel
	jobsChan := make(chan IndexJob, len(jobs))
	workerCount := GetWorkerCount()

	idx.logger.Info().
		Int("files", len(jobs)).
		Int("workers", workerCount).
		Msg("Starting parallel indexing")

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			idx.indexWorker(ctx, workerID, jobsChan, stats)
		}(w)
	}

	// Feed jobs to workers
	for _, job := range jobs {
		jobsChan <- job
	}
	close(jobsChan)

	// Wait for all workers to complete
	wg.Wait()

	return stats
}

// indexWorker processes files from the job queue
func (idx *Indexer) indexWorker(ctx context.Context, workerID int, jobs <-chan IndexJob, stats *IndexStats) {
	for job := range jobs {
		if err := idx.indexFileOrChunks(ctx, job.RelPath, job.Content); err != nil {
			idx.logger.Error().
				Err(err).
				Int("worker", workerID).
				Str("path", job.RelPath).
				Msg("Failed to index file")
			stats.incErrors()
		} else {
			stats.incIndexed()

			// Log progress every 10 files
			indexed, _, _ := stats.get()
			if indexed%10 == 0 {
				idx.logger.Debug().
					Int("indexed", indexed).
					Int("worker", workerID).
					Msg("Indexing progress")
			}
		}
	}
}

// indexFileOrChunks indexes a file using token-aware chunking
// Always chunks files that might exceed model token limits
func (idx *Indexer) indexFileOrChunks(ctx context.Context, relPath, content string) error {
	// Use token-aware chunking - ChunkFile decides whether to chunk based on token budget
	language := detectLanguage(relPath)
	chunks := ChunkFile(relPath, content, language)

	if len(chunks) > 1 {
		idx.logger.Debug().
			Str("path", relPath).
			Int("size", len(content)).
			Int("chunks", len(chunks)).
			Msg("File chunked for token budget")
	}

	for _, chunk := range chunks {
		// Build chunk path: "path/file.go#chunk0", "path/file.go#chunk1", etc.
		chunkPath := relPath
		if len(chunks) > 1 {
			chunkPath = fmt.Sprintf("%s#chunk%d", relPath, chunk.ChunkIndex)
		}

		// Index chunk content directly WITHOUT header
		// The header would confuse LLMs by appearing as code
		// Chunk path already provides context via file_path field
		if err := idx.store.IndexFile(ctx, chunkPath, chunk.Content); err != nil {
			return fmt.Errorf("chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	return nil
}

// isCodeFile checks if a file should be indexed
// Delegates to filetypes package (single source of truth)
func isCodeFile(path string) bool {
	return filetypes.IsCodeFile(path)
}

// computeFileHash computes SHA256 hash of file content
func computeFileHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}

// IndexIncremental performs incremental indexing based on git changes
// Only re-indexes files that have changed since the last indexed commit
func (idx *Indexer) IndexIncremental(ctx context.Context) error {
	if idx.repoName == "" || idx.branch == "" {
		return fmt.Errorf("incremental indexing requires repoName and branch to be set")
	}

	idx.logger.Info().
		Str("repo", idx.repoName).
		Str("branch", idx.branch).
		Str("path", idx.repoPath).
		Msg("Starting incremental indexing")

	// Check if we need re-indexing
	needsReindex, currentCommit, err := NeedsReindexing(idx.repoPath, idx.repoName, idx.branch)
	if err != nil {
		return fmt.Errorf("check reindexing: %w", err)
	}

	if !needsReindex {
		idx.logger.Info().Msg("No changes detected, skipping indexing")
		return nil
	}

	// Load previous metadata
	meta, err := LoadMetadata(idx.repoName, idx.branch)
	if err != nil {
		return fmt.Errorf("load metadata: %w", err)
	}

	var indexed, errors int

	if meta == nil {
		// First time indexing this branch - index everything
		idx.logger.Info().Msg("First time indexing this branch, indexing all files")
		return idx.indexAllFiles(ctx, currentCommit)
	}

	// Get changed files since last commit
	changedFiles, err := GetChangedFilesSince(idx.repoPath, meta.CommitSHA)
	if err != nil {
		return fmt.Errorf("get changed files: %w", err)
	}

	idx.logger.Info().
		Int("changed_files", len(changedFiles)).
		Str("from_commit", meta.CommitSHA[:8]).
		Str("to_commit", currentCommit[:8]).
		Msg("Detected changed files")

	// Collect files and content for parallel indexing
	var jobsToIndex []IndexJob
	for _, file := range changedFiles {
		fullPath := filepath.Join(idx.repoPath, file)
		if !isCodeFile(fullPath) {
			continue
		}

		// Check if file still exists (might have been deleted)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted, remove from index (deletes all chunks)
				if err := idx.store.DeleteFile(ctx, file); err != nil {
					idx.logger.Error().Err(err).Str("path", file).Msg("Failed to delete file from index")
					errors++
				} else {
					idx.logger.Debug().Str("path", file).Msg("File deleted from index")
				}
				continue
			}
			idx.logger.Warn().Err(err).Str("path", file).Msg("Failed to read file")
			errors++
			continue
		}

		// Skip extremely large files (>500KB)
		if len(content) > 500000 {
			idx.logger.Debug().
				Str("path", file).
				Int("size", len(content)).
				Msg("File too large, skipping (>500KB)")
			continue
		}

		// Before re-indexing, delete any existing chunks for this file
		// This prevents stale chunks from lingering
		if err := idx.store.DeleteFile(ctx, file); err != nil {
			idx.logger.Warn().Err(err).Str("path", file).Msg("Failed to delete old chunks before re-indexing")
		}

		jobsToIndex = append(jobsToIndex, IndexJob{
			RelPath: file,
			Content: string(content),
		})
	}

	// Index files in parallel with chunking support
	if len(jobsToIndex) > 0 {
		stats := idx.indexFilesParallel(ctx, jobsToIndex)
		indexed = stats.Indexed
		errors += stats.Errors
	}

	// Update metadata
	indexedAt := time.Now()
	if ctxTime, ok := ctx.Value("indexed_at").(time.Time); ok && !ctxTime.IsZero() {
		indexedAt = ctxTime
	}

	newMeta := &BranchMetadata{
		RepoName:  idx.repoName,
		Branch:    idx.branch,
		CommitSHA: currentCommit,
		IndexedAt: indexedAt,
		FileCount: indexed,
	}

	if err := SaveMetadata(newMeta); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}

	idx.logger.Info().
		Int("indexed", indexed).
		Int("errors", errors).
		Str("commit", currentCommit[:8]).
		Msg("Incremental indexing completed")

	return nil
}

// indexAllFiles indexes all files in the repository (used for first-time indexing)
func (idx *Indexer) indexAllFiles(ctx context.Context, currentCommit string) error {
	// Collect all files first
	var filesToIndex []IndexJob

	err := filepath.Walk(idx.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip common directories (delegates to filetypes)
			if filetypes.ShouldSkipDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !isCodeFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			idx.logger.Warn().Err(err).Str("path", path).Msg("Failed to read file")
			return nil
		}

		relPath, err := filepath.Rel(idx.repoPath, path)
		if err != nil {
			relPath = path
		}

		// Skip extremely large files (>500KB - likely generated, minified, or binary)
		if len(content) > 500000 {
			idx.logger.Debug().
				Str("path", relPath).
				Int("size", len(content)).
				Msg("File too large, skipping (>500KB)")
			return nil
		}

		filesToIndex = append(filesToIndex, IndexJob{
			RelPath: relPath,
			Content: string(content),
		})

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk error: %w", err)
	}

	// Index files in parallel
	stats := idx.indexFilesParallel(ctx, filesToIndex)

	// Save metadata
	meta := &BranchMetadata{
		RepoName:  idx.repoName,
		Branch:    idx.branch,
		CommitSHA: currentCommit,
		IndexedAt: time.Now(),
		FileCount: stats.Indexed,
	}

	if err := SaveMetadata(meta); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}

	idx.logger.Info().
		Int("indexed", stats.Indexed).
		Int("skipped", stats.Skipped).
		Int("errors", stats.Errors).
		Msg("Initial indexing completed")

	return nil
}
