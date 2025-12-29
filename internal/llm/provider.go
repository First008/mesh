// Package llm provides LLM provider abstractions and implementations.
//
// The llm package defines the LLMProvider interface and implements support
// for multiple AI providers (Anthropic Claude, Ollama, etc.) with optional
// prompt caching capabilities.
package llm

import (
	"context"
)

// LLMProvider is the interface that all LLM providers must implement
// This adapter pattern allows easy swapping between different AI providers
// (Anthropic, OpenAI, Gemini, local models, etc.)
type LLMProvider interface {
	// Ask sends a question to the LLM and returns a response
	Ask(ctx context.Context, systemPrompt, userPrompt string) (*Response, error)

	// AskWithCache sends a question with cacheable context (Phase 3)
	// cacheableContext: content that rarely changes (CLAUDE.md, README, etc.)
	// regularContext: content that may vary (code search results)
	// question: the user's question
	AskWithCache(ctx context.Context, systemPrompt, cacheableContext, regularContext, question string) (*Response, error)

	// CountTokens estimates the number of tokens in a text
	CountTokens(text string) (int, error)

	// GetModel returns the model identifier being used
	GetModel() string

	// SupportsPromptCaching returns true if the provider supports prompt caching
	SupportsPromptCaching() bool
}

// Response contains the LLM's response along with usage statistics
type Response struct {
	// Content is the text response from the LLM
	Content string

	// InputTokens is the number of tokens in the input
	InputTokens int

	// OutputTokens is the number of tokens in the output
	OutputTokens int

	// CachedTokens is the number of tokens that were served from cache
	// Only applicable for providers that support prompt caching
	CachedTokens int

	// Model is the specific model that generated this response
	Model string
}
