package vectorstore

import (
	"context"
)

// VectorStore is the interface for vector database operations
// Implementations: QdrantStore, InMemoryStore (for testing), etc.
type VectorStore interface {
	// IndexFile indexes a single file with its content
	IndexFile(ctx context.Context, filePath, content string) error

	// Search performs semantic search for relevant code
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// SearchWithAggregation performs search with chunk aggregation to reconstruct complete files.
	// Implementations without aggregation support should fall back to Search().
	// This method is used by context builders to provide complete file context rather than fragments.
	SearchWithAggregation(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// DeleteFile removes a file from the index
	DeleteFile(ctx context.Context, filePath string) error

	// DeleteCollection removes an entire collection (repository)
	DeleteCollection(ctx context.Context) error

	// GetStats returns statistics about the vector store
	GetStats(ctx context.Context) (*Stats, error)

	// Close closes the connection to the vector store
	Close() error
}

// SearchResult represents a search result from the vector store
type SearchResult struct {
	// FilePath is the relative path to the file in the repository
	FilePath string

	// Content is the file content
	Content string

	// Score is the similarity score (0.0 to 1.0, higher is better)
	Score float32

	// Language is the programming language
	Language string

	// FileHash is the SHA256 hash of the file (for change detection)
	FileHash string
}

// Stats holds statistics about the vector store
type Stats struct {
	// TotalVectors is the total number of vectors in the collection
	TotalVectors int64

	// CollectionName is the name of the collection
	CollectionName string

	// IndexedFiles is the number of unique files indexed
	IndexedFiles int

	// LastIndexed is the timestamp of the last indexing operation
	LastIndexed string
}
