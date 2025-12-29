package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// OllamaLLMProvider implements the LLMProvider interface for Ollama's local LLM API
// This allows running models like llama3.3:70b, deepseek-coder-v2, qwen2.5-coder locally
type OllamaLLMProvider struct {
	baseURL string
	model   string
	client  *http.Client
	logger  zerolog.Logger
}

// NewOllamaLLMProvider creates a new Ollama LLM provider
func NewOllamaLLMProvider(baseURL, model string, logger zerolog.Logger) (*OllamaLLMProvider, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Default Ollama URL
	}

	if model == "" {
		model = "llama3.3:70b" // Default to Llama 3.3 70B
	}

	return &OllamaLLMProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		client:  &http.Client{},
		logger:  logger,
	}, nil
}

// ollamaRequest represents the request body for Ollama API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

// ollamaResponse represents the response from Ollama API
type ollamaResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	Context   []int  `json:"context,omitempty"`
	TotalDuration int64 `json:"total_duration,omitempty"`
	LoadDuration  int64 `json:"load_duration,omitempty"`
	PromptEvalCount int `json:"prompt_eval_count,omitempty"`
	EvalCount     int `json:"eval_count,omitempty"`
}

// Ask sends a question to Ollama and returns the response
func (op *OllamaLLMProvider) Ask(ctx context.Context, systemPrompt, userPrompt string) (*Response, error) {
	// Build request
	reqBody := ollamaRequest{
		Model:  op.model,
		Prompt: userPrompt,
		System: systemPrompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", op.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := op.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Build response
	response := &Response{
		Content:      ollamaResp.Response,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
		CachedTokens: 0, // Ollama doesn't support caching
		Model:        ollamaResp.Model,
	}

	op.logger.Debug().
		Str("model", ollamaResp.Model).
		Int("input_tokens", response.InputTokens).
		Int("output_tokens", response.OutputTokens).
		Int64("duration_ms", ollamaResp.TotalDuration/1000000).
		Msg("Ollama LLM request completed")

	return response, nil
}

// AskWithCache sends a question (caching not supported by Ollama, falls back to regular Ask)
func (op *OllamaLLMProvider) AskWithCache(ctx context.Context, systemPrompt, cacheableContext, regularContext, question string) (*Response, error) {
	// Ollama doesn't support prompt caching, so we combine everything into one prompt
	var combinedPrompt strings.Builder

	if cacheableContext != "" {
		combinedPrompt.WriteString(cacheableContext)
		combinedPrompt.WriteString("\n\n")
	}

	if regularContext != "" {
		combinedPrompt.WriteString(regularContext)
		combinedPrompt.WriteString("\n\n")
	}

	combinedPrompt.WriteString(question)

	return op.Ask(ctx, systemPrompt, combinedPrompt.String())
}

// CountTokens estimates the number of tokens in a text
// This is a rough estimate: ~4 characters per token for English text
func (op *OllamaLLMProvider) CountTokens(text string) (int, error) {
	// Rough approximation: 1 token ≈ 4 characters for English
	return len(text) / 4, nil
}

// GetModel returns the model identifier
func (op *OllamaLLMProvider) GetModel() string {
	return op.model
}

// SupportsPromptCaching returns false since Ollama doesn't support prompt caching
func (op *OllamaLLMProvider) SupportsPromptCaching() bool {
	return false
}
