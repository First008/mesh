package vectorstore

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/rs/zerolog"
)

// OllamaEmbeddingProvider implements EmbeddingProvider using Ollama
// Runs embeddings locally - no data sent to external APIs
type OllamaEmbeddingProvider struct {
	client *api.Client
	model  string
	logger zerolog.Logger
}

const (
	// Default Ollama model for embeddings
	// Can be overridden via config - see configs/repos.yaml for options
	DefaultOllamaModel = "bge-m3"

	// Model dimensions (used for Qdrant collection creation)
	OllamaNomicDimension = 768  // nomic-embed-text: 2K context, fastest
	OllamaBGEM3Dimension = 1024 // bge-m3: 8K context, best quality (recommended)
	OllamaMxbaiDimension = 1024 // mxbai-embed-large: 512 token context, middle speed
)

// NewOllamaEmbeddingProvider creates a new Ollama embedding provider
func NewOllamaEmbeddingProvider(ollamaURL, model string, logger zerolog.Logger) (*OllamaEmbeddingProvider, error) {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434" // Default Ollama URL
	}

	if model == "" {
		model = DefaultOllamaModel
	}

	// Parse URL
	parsedURL, err := url.Parse(ollamaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid ollama URL: %w", err)
	}

	// Create Ollama client
	client := api.NewClient(parsedURL, http.DefaultClient)

	provider := &OllamaEmbeddingProvider{
		client: client,
		model:  model,
		logger: logger,
	}

	// Verify model is available
	if err := provider.verifyModel(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to verify ollama model: %w", err)
	}

	logger.Info().
		Str("model", model).
		Str("url", ollamaURL).
		Msg("Ollama embedding provider initialized")

	return provider, nil
}

// CreateEmbedding creates an embedding using Ollama
func (o *OllamaEmbeddingProvider) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	start := time.Now()

	req := &api.EmbedRequest{
		Model: o.model,
		Input: text,
	}

	resp, err := o.client.Embed(ctx, req)
	if err != nil {
		duration := time.Since(start)
		o.logger.Warn().
			Dur("duration_ms", duration).
			Int("text_len", len(text)).
			Err(err).
			Msg("Ollama embedding failed")
		return nil, fmt.Errorf("ollama embedding error: %w", err)
	}

	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned from ollama")
	}

	// Ollama returns [][]float64, we need []float32
	embedding64 := resp.Embeddings[0]
	embedding32 := make([]float32, len(embedding64))
	for i, v := range embedding64 {
		embedding32[i] = float32(v)
	}

	duration := time.Since(start)

	// Log slow embeddings
	if duration > 5*time.Second {
		o.logger.Warn().
			Dur("duration", duration).
			Int("text_len", len(text)).
			Int("dimension", len(embedding32)).
			Msg("Slow embedding detected")
	}

	return embedding32, nil
}

// GetDimensions returns the embedding dimension
// IMPORTANT: If you add a new model to configs/repos.yaml,
// you may need to add it here if dimensions differ from 1024
func (o *OllamaEmbeddingProvider) GetDimensions() int {
	switch o.model {
	case "bge-m3", "bge-m3:latest":
		return OllamaBGEM3Dimension // 1024
	case "mxbai-embed-large", "mxbai-embed-large:latest":
		return OllamaMxbaiDimension // 1024
	case "nomic-embed-text", "nomic-embed-text:latest":
		return OllamaNomicDimension // 768
	default:
		// Default to 1024 for modern embedding models
		// If you get dimension errors, add your model above
		o.logger.Warn().
			Str("model", o.model).
			Int("assumed_dimensions", 1024).
			Msg("Unknown model, assuming 1024 dimensions")
		return 1024
	}
}

// GetModelName returns the model name
func (o *OllamaEmbeddingProvider) GetModelName() string {
	return o.model
}

// verifyModel checks if the model is available in Ollama
func (o *OllamaEmbeddingProvider) verifyModel(ctx context.Context) error {
	// List available models
	listResp, err := o.client.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list ollama models: %w", err)
	}

	// Check if our model is available
	for _, model := range listResp.Models {
		if model.Name == o.model || model.Name == o.model+":latest" {
			o.logger.Debug().
				Str("model", o.model).
				Msg("Ollama model found and ready")
			return nil
		}
	}

	return fmt.Errorf("model %s not found in ollama. Run: ollama pull %s", o.model, o.model)
}
