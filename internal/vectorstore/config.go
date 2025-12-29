package vectorstore

import (
	"runtime"
)

// IndexingConfig holds configuration for indexing performance
type IndexingConfig struct {
	MaxWorkers      int  // Concurrent file indexing workers
	ChunkingEnabled bool // Enable chunking for large files
	MaxChunkSize    int  // Maximum chunk size in bytes
	OverlapSize     int  // Overlap between chunks in bytes
}

// DefaultIndexingConfig returns sensible defaults based on available resources
func DefaultIndexingConfig() *IndexingConfig {
	// Detect available CPU cores
	numCPU := runtime.NumCPU()

	workers := numCPU / 2
	if workers < 3 {
		workers = 3
	}
	if workers > 8 {
		workers = 8
	}

	return &IndexingConfig{
		MaxWorkers:      workers,
		ChunkingEnabled: true,
		MaxChunkSize:    6000, // ~1,500 tokens for 8K context models
		OverlapSize:     400,  // ~100 tokens overlap
	}
}

// GetWorkerCount returns optimal worker count for current system
func GetWorkerCount() int {
	return DefaultIndexingConfig().MaxWorkers
}
