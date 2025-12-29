package vectorstore

import (
	"context"
)

// EmbeddingProvider is the interface for embedding providers
// Allows swapping between OpenAI, Ollama, local models, etc.
type EmbeddingProvider interface {
	// CreateEmbedding creates an embedding vector from text
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)

	// GetDimensions returns the dimensionality of the embedding vectors
	GetDimensions() int

	// GetModelName returns the name of the embedding model
	GetModelName() string
}
