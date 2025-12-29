// Package factory provides factory functions for creating providers and dependencies.
//
// The factory package centralizes provider creation logic, eliminating duplication
// and providing a single source of truth for dependency construction.
package factory

import (
	"fmt"

	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

// EmbeddingConfig holds configuration for creating an embedding provider
type EmbeddingConfig struct {
	Provider    string // "openai" | "ollama"
	OpenAIKey   string
	OllamaURL   string
	OllamaModel string
}

// NewEmbeddingProvider creates an embedding provider based on configuration.
// This is the single source of truth for embedding provider creation,
// eliminating duplication across agent.go and gateway.go.
func NewEmbeddingProvider(cfg EmbeddingConfig, logger zerolog.Logger) (vectorstore.EmbeddingProvider, error) {
	providerType := cfg.Provider

	// Auto-detect provider if not specified
	if providerType == "" {
		if cfg.OllamaURL != "" || cfg.OllamaModel != "" {
			providerType = "ollama"
		} else if cfg.OpenAIKey != "" {
			providerType = "openai"
		} else {
			return nil, fmt.Errorf("no embedding provider configured")
		}
	}

	switch providerType {
	case "ollama":
		return newOllamaProvider(cfg, logger)

	case "openai":
		return newOpenAIProvider(cfg, logger)

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s (supported: ollama, openai)", providerType)
	}
}

func newOllamaProvider(cfg EmbeddingConfig, logger zerolog.Logger) (vectorstore.EmbeddingProvider, error) {
	ollamaURL := cfg.OllamaURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434" // Default Ollama URL
	}

	provider, err := vectorstore.NewOllamaEmbeddingProvider(
		ollamaURL,
		cfg.OllamaModel,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create Ollama embedding provider: %w", err)
	}

	logger.Info().
		Str("provider", "ollama").
		Str("url", ollamaURL).
		Str("model", cfg.OllamaModel).
		Msg("Created Ollama embedding provider")

	return provider, nil
}

func newOpenAIProvider(cfg EmbeddingConfig, logger zerolog.Logger) (vectorstore.EmbeddingProvider, error) {
	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key required for openai embedding provider")
	}

	provider, err := vectorstore.NewOpenAIEmbeddingProvider(
		cfg.OpenAIKey,
		"", // Use default model
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI embedding provider: %w", err)
	}

	logger.Info().
		Str("provider", "openai").
		Msg("Created OpenAI embedding provider")

	return provider, nil
}
