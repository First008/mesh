package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/rs/zerolog"
)

// AnthropicProvider implements the LLMProvider interface for Anthropic's Claude API
type AnthropicProvider struct {
	client anthropic.Client
	model  string
	logger zerolog.Logger
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string, logger zerolog.Logger) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	if model == "" {
		model = string(anthropic.ModelClaudeSonnet4_5_20250929) // Default to latest Sonnet 4.5
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &AnthropicProvider{
		client: client,
		model:  model,
		logger: logger,
	}, nil
}

// Ask sends a question to Claude and returns the response
func (ap *AnthropicProvider) Ask(ctx context.Context, systemPrompt, userPrompt string) (*Response, error) {
	// Build the request
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(ap.model),
		MaxTokens: 8192, // Claude Sonnet 4.5 supports up to 8192 output tokens
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	}

	// Add system prompt if provided
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{
				Text: systemPrompt,
				Type: "text",
			},
		}
	}

	// Call the API
	message, err := ap.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Extract the response text
	if len(message.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	var responseText strings.Builder
	for _, block := range message.Content {
		// Check if this is a text block
		if block.Type == "text" {
			responseText.WriteString(block.Text)
		}
	}

	// Build response
	response := &Response{
		Content:      responseText.String(),
		InputTokens:  int(message.Usage.InputTokens),
		OutputTokens: int(message.Usage.OutputTokens),
		CachedTokens: 0, // Will be set by cache layer if applicable
		Model:        string(message.Model),
	}

	ap.logger.Debug().
		Str("model", string(message.Model)).
		Int("input_tokens", response.InputTokens).
		Int("output_tokens", response.OutputTokens).
		Str("stop_reason", string(message.StopReason)).
		Msg("Claude API request completed")

	return response, nil
}

// AskWithCache sends a question with prompt caching enabled
// Uses Claude's cache control to cache static context (90% cost savings)
func (ap *AnthropicProvider) AskWithCache(ctx context.Context, systemPrompt, cacheableContext, regularContext, question string) (*Response, error) {
	// Build user message with cacheable and regular content
	var userContent []anthropic.ContentBlockParamUnion

	// Add cacheable context first (will be cached)
	if cacheableContext != "" {
		userContent = append(userContent, anthropic.NewTextBlock(cacheableContext))
	}

	// Add regular context (not cached, may change)
	if regularContext != "" {
		userContent = append(userContent, anthropic.NewTextBlock(regularContext))
	}

	// Add the actual question
	userContent = append(userContent, anthropic.NewTextBlock(question))

	// Build request with cache control
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(ap.model),
		MaxTokens: 8192, // Claude Sonnet 4.5 supports up to 8192 output tokens
		Messages: []anthropic.MessageParam{
			{
				Role:    anthropic.MessageParamRoleUser,
				Content: userContent,
			},
		},
	}

	// Add system prompt with cache control if provided
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{
				Text: systemPrompt,
				Type: "text",
				// Mark system prompt as cacheable (5 minute TTL)
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		}
	}

	// Call the API
	message, err := ap.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Extract response text
	if len(message.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	var responseText strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText.WriteString(block.Text)
		}
	}

	// Extract cache tokens from usage
	cachedTokens := int(message.Usage.CacheReadInputTokens)

	// Build response
	response := &Response{
		Content:      responseText.String(),
		InputTokens:  int(message.Usage.InputTokens),
		OutputTokens: int(message.Usage.OutputTokens),
		CachedTokens: cachedTokens,
		Model:        string(message.Model),
	}

	ap.logger.Debug().
		Str("model", string(message.Model)).
		Int("input_tokens", response.InputTokens).
		Int("output_tokens", response.OutputTokens).
		Int("cached_tokens", response.CachedTokens).
		Str("stop_reason", string(message.StopReason)).
		Msg("Claude API request with caching completed")

	return response, nil
}

// CountTokens estimates the number of tokens in a text
// This is a rough estimate: ~4 characters per token for English text
func (ap *AnthropicProvider) CountTokens(text string) (int, error) {
	// Rough approximation: 1 token ≈ 4 characters for English
	// This is not exact but good enough for budget management
	return len(text) / 4, nil
}

// GetModel returns the model identifier
func (ap *AnthropicProvider) GetModel() string {
	return ap.model
}

// SupportsPromptCaching returns true since Anthropic supports prompt caching
func (ap *AnthropicProvider) SupportsPromptCaching() bool {
	return true
}
