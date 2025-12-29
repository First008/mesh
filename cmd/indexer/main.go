package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

func main() {
	// Parse flags
	repoPath := flag.String("repo", "", "Path to repository to index")
	repoName := flag.String("name", "", "Repository name")
	qdrantURL := flag.String("qdrant", "localhost:6334", "Qdrant URL")
	provider := flag.String("provider", "ollama", "Embedding provider: ollama or openai")
	openaiKey := flag.String("openai-key", "", "OpenAI API key (for openai provider)")
	ollamaURL := flag.String("ollama-url", "http://localhost:11434", "Ollama API URL")
	ollamaModel := flag.String("ollama-model", "nomic-embed-text", "Ollama model name")
	flag.Parse()

	// Setup logger
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger()

	// Validate required flags
	if *repoPath == "" {
		logger.Fatal().Msg("--repo flag is required")
	}

	if *repoName == "" {
		logger.Fatal().Msg("--name flag is required")
	}

	logger.Info().
		Str("repo_path", *repoPath).
		Str("repo_name", *repoName).
		Str("qdrant_url", *qdrantURL).
		Str("provider", *provider).
		Msg("Starting repository indexing")

	// Create embedding provider
	var embeddingProvider vectorstore.EmbeddingProvider
	var err error

	switch *provider {
	case "ollama":
		embeddingProvider, err = vectorstore.NewOllamaEmbeddingProvider(*ollamaURL, *ollamaModel, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create Ollama embedding provider")
		}

	case "openai":
		apiKey := *openaiKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			logger.Fatal().Msg("OpenAI API key required (--openai-key or OPENAI_API_KEY env var)")
		}

		embeddingProvider, err = vectorstore.NewOpenAIEmbeddingProvider(apiKey, "", logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create OpenAI embedding provider")
		}

	default:
		logger.Fatal().Str("provider", *provider).Msg("Unknown provider. Use 'ollama' or 'openai'")
	}

	logger.Info().
		Str("model", embeddingProvider.GetModelName()).
		Int("dimensions", embeddingProvider.GetDimensions()).
		Msg("Embedding provider initialized")

	// Create Qdrant store
	store, err := vectorstore.NewQdrantStore(*qdrantURL, embeddingProvider, *repoName, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Qdrant store")
	}
	defer store.Close()

	// Create indexer
	indexer := vectorstore.NewIndexer(store, *repoPath, logger)

	// Index the repository
	ctx := context.Background()
	startTime := time.Now()

	if err := indexer.IndexRepository(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Indexing failed")
	}

	duration := time.Since(startTime)

	// Get stats
	stats, err := store.GetStats(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get stats")
	} else {
		logger.Info().
			Int64("total_vectors", stats.TotalVectors).
			Int("indexed_files", stats.IndexedFiles).
			Str("collection", stats.CollectionName).
			Dur("duration", duration).
			Msg("Indexing completed successfully")

		fmt.Printf("\n✅ Indexing complete!\n")
		fmt.Printf("   Collection: %s\n", stats.CollectionName)
		fmt.Printf("   Files indexed: %d\n", stats.IndexedFiles)
		fmt.Printf("   Total vectors: %d\n", stats.TotalVectors)
		fmt.Printf("   Duration: %s\n", duration.Round(time.Second))
	}
}
