// Package agent provides repository-specific AI agents with LLM integration.
//
// The agent package implements the core Agent type that manages LLM providers,
// builds context from repository code using vector search, tracks API costs,
// and supports prompt caching for cost optimization.
package agent

import (
	"context"
	"fmt"

	contextbuilder "github.com/First008/mesh/internal/context"
	"github.com/First008/mesh/internal/factory"
	"github.com/First008/mesh/internal/llm"
	"github.com/First008/mesh/internal/vectorstore"
	"github.com/First008/mesh/pkg/telemetry"
	"github.com/rs/zerolog"
)

// Agent represents a repository-specific AI agent
type Agent struct {
	config         *Config
	personality    *Personality
	llmProvider    llm.LLMProvider
	contextBuilder *contextbuilder.Builder
	costTracker    *telemetry.CostTracker
	logger         zerolog.Logger
}

// New creates a new Agent instance
func New(config *Config, logger zerolog.Logger) (*Agent, error) {
	// Create personality
	personality := NewPersonality(config.RepoName, config.Personality, config.FocusPaths)

	// Create LLM provider (Anthropic for Phase 1)
	llmProvider, err := llm.NewAnthropicProvider(
		config.AnthropicKey,
		config.LLMModel, // Use configured model (e.g. haiku, sonnet)
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Create context builder
	contextBuilder := contextbuilder.NewBuilder(
		config.RepoPath,
		config.RepoName,
		config.FocusPaths,
		logger,
	)

	// Set exclude patterns if configured
	if len(config.ExcludePatterns) > 0 {
		contextBuilder.SetExcludePatterns(config.ExcludePatterns)
	}

	// Initialize vector store if configured (Phase 2+)
	if config.QdrantURL != "" {
		// Create embedding provider using factory (eliminates duplication)
		embeddingProvider, err := factory.NewEmbeddingProvider(
			factory.EmbeddingConfig{
				Provider:    config.EmbeddingProvider,
				OpenAIKey:   config.OpenAIKey,
				OllamaURL:   config.OllamaURL,
				OllamaModel: config.OllamaModel,
			},
			logger,
		)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create embedding provider, vector search disabled")
		}

		// Create vector store if we have an embedding provider
		if embeddingProvider != nil {
			vectorStore, err := vectorstore.NewQdrantStore(
				config.QdrantURL,
				embeddingProvider,
				config.RepoName,
				logger,
			)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to initialize vector store, will use keyword search")
			} else {
				contextBuilder.SetVectorStore(vectorStore)
				logger.Info().
					Str("provider", embeddingProvider.GetModelName()).
					Int("dimensions", embeddingProvider.GetDimensions()).
					Msg("Vector store initialized - semantic search enabled")
			}
		}
	}

	// Create cost tracker
	costTracker := telemetry.NewCostTracker(
		config.CostLimits.DailyMaxUSD,
		config.CostLimits.AlertThresholdUSD,
		config.CostLimits.PerQueryMaxTokens,
		logger,
	)

	return &Agent{
		config:         config,
		personality:    personality,
		llmProvider:    llmProvider,
		contextBuilder: contextBuilder,
		costTracker:    costTracker,
		logger:         logger,
	}, nil
}

// Ask asks the agent a question about the repository
func (a *Agent) Ask(ctx context.Context, question string) (*llm.Response, error) {
	a.logger.Info().
		Str("repo", a.config.RepoName).
		Str("question", question).
		Msg("Received question")

	// 1. Build context in layers (cacheable vs regular)
	contextLayers, err := a.contextBuilder.BuildContextLayers(question)
	if err != nil {
		return nil, fmt.Errorf("failed to build context: %w", err)
	}

	// 2. Get system prompt from personality
	systemPrompt := a.personality.GetSystemPrompt()

	// 3. Call LLM provider with caching if supported
	var response *llm.Response
	if a.llmProvider.SupportsPromptCaching() && contextLayers.Cacheable != "" {
		// Use caching for static content
		response, err = a.llmProvider.AskWithCache(
			ctx,
			systemPrompt,
			contextLayers.Cacheable,
			contextLayers.Regular,
			question,
		)
	} else {
		// Fallback to regular Ask (no caching)
		combinedContext := contextLayers.Cacheable + contextLayers.Regular
		userPrompt := fmt.Sprintf(`Repository Context:
%s

---

Question: %s`, combinedContext, question)
		response, err = a.llmProvider.Ask(ctx, systemPrompt, userPrompt)
	}

	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// 5. Track costs
	cost, err := a.costTracker.RecordRequest(
		response.Model,
		response.InputTokens,
		response.OutputTokens,
		response.CachedTokens,
	)
	if err != nil {
		a.logger.Error().Err(err).Msg("Cost tracking failed")
		// Don't fail the request if cost tracking fails, just log it
	}

	a.logger.Info().
		Str("repo", a.config.RepoName).
		Int("input_tokens", response.InputTokens).
		Int("output_tokens", response.OutputTokens).
		Int("cached_tokens", response.CachedTokens).
		Float64("cost_usd", cost).
		Msg("Question answered")

	return response, nil
}

// GetRepoName returns the repository name
func (a *Agent) GetRepoName() string {
	return a.config.RepoName
}

// GetDailyStats returns daily cost statistics
func (a *Agent) GetDailyStats() telemetry.DailyStats {
	return a.costTracker.GetDailyStats()
}

// GetTotalStats returns total cost statistics
func (a *Agent) GetTotalStats() telemetry.TotalStats {
	return a.costTracker.GetTotalStats()
}

// GetModel returns the LLM model being used
func (a *Agent) GetModel() string {
	return a.llmProvider.GetModel()
}

// SetVectorStore updates the agent's vector store for semantic search
// Used by gateway to update the vector store after branch detection
func (a *Agent) SetVectorStore(store vectorstore.VectorStore) {
	a.contextBuilder.SetVectorStore(store)
}
