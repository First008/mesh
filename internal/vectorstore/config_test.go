package vectorstore

import (
	"runtime"
	"testing"
)

func TestDefaultIndexingConfig(t *testing.T) {
	config := DefaultIndexingConfig()

	if config == nil {
		t.Fatal("DefaultIndexingConfig returned nil")
	}

	// Worker count should be reasonable
	if config.MaxWorkers < 3 {
		t.Errorf("Expected MaxWorkers >= 3, got %d", config.MaxWorkers)
	}

	if config.MaxWorkers > 8 {
		t.Errorf("Expected MaxWorkers <= 8, got %d", config.MaxWorkers)
	}

	// Chunking should be enabled by default
	if !config.ChunkingEnabled {
		t.Error("ChunkingEnabled should be true by default")
	}

	// Chunk sizes should be positive
	if config.MaxChunkSize <= 0 {
		t.Errorf("MaxChunkSize should be positive, got %d", config.MaxChunkSize)
	}

	if config.OverlapSize < 0 {
		t.Errorf("OverlapSize should be non-negative, got %d", config.OverlapSize)
	}

	// Overlap should be smaller than chunk size
	if config.OverlapSize >= config.MaxChunkSize {
		t.Errorf("OverlapSize (%d) should be less than MaxChunkSize (%d)",
			config.OverlapSize, config.MaxChunkSize)
	}
}

func TestDefaultIndexingConfig_WorkerCalculation(t *testing.T) {
	config := DefaultIndexingConfig()

	numCPU := runtime.NumCPU()
	expectedWorkers := numCPU / 2

	if expectedWorkers < 3 {
		expectedWorkers = 3
	}
	if expectedWorkers > 8 {
		expectedWorkers = 8
	}

	if config.MaxWorkers != expectedWorkers {
		t.Errorf("Expected MaxWorkers %d (for %d CPUs), got %d",
			expectedWorkers, numCPU, config.MaxWorkers)
	}
}

func TestGetWorkerCount(t *testing.T) {
	count := GetWorkerCount()

	if count < 3 {
		t.Errorf("GetWorkerCount should return at least 3, got %d", count)
	}

	if count > 8 {
		t.Errorf("GetWorkerCount should return at most 8, got %d", count)
	}

	// Should match DefaultIndexingConfig
	config := DefaultIndexingConfig()
	if count != config.MaxWorkers {
		t.Errorf("GetWorkerCount (%d) should match DefaultIndexingConfig.MaxWorkers (%d)",
			count, config.MaxWorkers)
	}
}

func TestIndexingConfig_ChunkSizes(t *testing.T) {
	config := DefaultIndexingConfig()

	// Chunk size should be reasonable for LLM context windows
	// Typically 3000-8000 characters for ~750-2000 tokens
	if config.MaxChunkSize < 1000 {
		t.Errorf("MaxChunkSize seems too small: %d", config.MaxChunkSize)
	}

	if config.MaxChunkSize > 20000 {
		t.Errorf("MaxChunkSize seems too large: %d", config.MaxChunkSize)
	}

	// Overlap should be 5-20% of chunk size
	minOverlap := config.MaxChunkSize / 20
	maxOverlap := config.MaxChunkSize / 5

	if config.OverlapSize < minOverlap {
		t.Errorf("OverlapSize (%d) seems too small relative to MaxChunkSize (%d)",
			config.OverlapSize, config.MaxChunkSize)
	}

	if config.OverlapSize > maxOverlap {
		t.Errorf("OverlapSize (%d) seems too large relative to MaxChunkSize (%d)",
			config.OverlapSize, config.MaxChunkSize)
	}
}

func TestIndexingConfig_Fields(t *testing.T) {
	config := &IndexingConfig{
		MaxWorkers:      5,
		ChunkingEnabled: false,
		MaxChunkSize:    10000,
		OverlapSize:     500,
	}

	if config.MaxWorkers != 5 {
		t.Errorf("Expected MaxWorkers 5, got %d", config.MaxWorkers)
	}

	if config.ChunkingEnabled {
		t.Error("Expected ChunkingEnabled false")
	}

	if config.MaxChunkSize != 10000 {
		t.Errorf("Expected MaxChunkSize 10000, got %d", config.MaxChunkSize)
	}

	if config.OverlapSize != 500 {
		t.Errorf("Expected OverlapSize 500, got %d", config.OverlapSize)
	}
}
