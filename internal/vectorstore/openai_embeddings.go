package vectorstore

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog"
)

// OpenAIEmbeddingProvider implements EmbeddingProvider using OpenAI API
type OpenAIEmbeddingProvider struct {
	client openai.Client
	model  string
	logger zerolog.Logger
}

const (
	// OpenAI embedding models
	OpenAIModelTextEmbedding3Small = "text-embedding-3-small"
	OpenAIModelTextEmbedding3Large = "text-embedding-3-large"

	// Dimensions
	OpenAIDimensionSmall = 1536
	OpenAIDimensionLarge = 3072
)

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider
func NewOpenAIEmbeddingProvider(apiKey, model string, logger zerolog.Logger) (*OpenAIEmbeddingProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}

	if model == "" {
		model = OpenAIModelTextEmbedding3Small
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	logger.Info().
		Str("model", model).
		Msg("OpenAI embedding provider initialized")

	return &OpenAIEmbeddingProvider{
		client: client,
		model:  model,
		logger: logger,
	}, nil
}

// CreateEmbedding creates an embedding using OpenAI API
func (o *OpenAIEmbeddingProvider) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := o.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{text},
		},
		Model: openai.EmbeddingModelTextEmbedding3Small,
	})

	if err != nil {
		return nil, fmt.Errorf("openai embedding error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	// Convert float64 to float32
	embedding64 := resp.Data[0].Embedding
	embedding32 := make([]float32, len(embedding64))
	for i, v := range embedding64 {
		embedding32[i] = float32(v)
	}

	return embedding32, nil
}

// GetDimensions returns the embedding dimension
func (o *OpenAIEmbeddingProvider) GetDimensions() int {
	if o.model == OpenAIModelTextEmbedding3Large {
		return OpenAIDimensionLarge
	}
	return OpenAIDimensionSmall
}

// GetModelName returns the model name
func (o *OpenAIEmbeddingProvider) GetModelName() string {
	return o.model
}
