package llm

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestNewAnthropicProvider_Success(t *testing.T) {
	provider, err := NewAnthropicProvider("test-key-123", "", testLogger())
	if err != nil {
		t.Fatalf("NewAnthropicProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider, got nil")
	}

	// Verify default model
	model := provider.GetModel()
	if model == "" {
		t.Error("Expected non-empty model")
	}
}

func TestNewAnthropicProvider_MissingKey(t *testing.T) {
	_, err := NewAnthropicProvider("", "", testLogger())
	if err == nil {
		t.Fatal("Expected error for missing API key, got nil")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestNewAnthropicProvider_CustomModel(t *testing.T) {
	customModel := "claude-opus-4-5-20251101"
	provider, err := NewAnthropicProvider("test-key-123", customModel, testLogger())
	if err != nil {
		t.Fatalf("NewAnthropicProvider with custom model failed: %v", err)
	}

	model := provider.GetModel()
	if model != customModel {
		t.Errorf("Expected model %s, got %s", customModel, model)
	}
}

func TestNewAnthropicProvider_DefaultModel(t *testing.T) {
	provider, err := NewAnthropicProvider("test-key-123", "", testLogger())
	if err != nil {
		t.Fatalf("NewAnthropicProvider failed: %v", err)
	}

	model := provider.GetModel()
	// Should use a default Sonnet model
	if model == "" {
		t.Error("Expected default model to be set")
	}

	// Verify it's a valid Claude model name
	if len(model) < 10 {
		t.Errorf("Model name seems invalid: %s", model)
	}
}

func TestSupportsPromptCaching(t *testing.T) {
	provider, err := NewAnthropicProvider("test-key-123", "", testLogger())
	if err != nil {
		t.Fatalf("NewAnthropicProvider failed: %v", err)
	}

	// Anthropic provider should support prompt caching
	if !provider.SupportsPromptCaching() {
		t.Error("Anthropic provider should support prompt caching")
	}
}

func TestCountTokens_ValidInput(t *testing.T) {
	provider, err := NewAnthropicProvider("test-key-123", "", testLogger())
	if err != nil {
		t.Fatalf("NewAnthropicProvider failed: %v", err)
	}

	testCases := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			text:     "",
			minCount: 0,
			maxCount: 1,
		},
		{
			name:     "short text",
			text:     "Hello, world!",
			minCount: 1,
			maxCount: 10,
		},
		{
			name: "medium text",
			text: "This is a longer piece of text that should have more tokens. " +
				"We're testing the token counting functionality.",
			minCount: 10,
			maxCount: 50,
		},
		{
			name: "long text",
			text: `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    for i := 0; i < 10; i++ {
        fmt.Printf("Iteration %d\n", i)
    }
}`,
			minCount: 20,
			maxCount: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count, err := provider.CountTokens(tc.text)
			if err != nil {
				t.Fatalf("CountTokens failed: %v", err)
			}

			if count < tc.minCount {
				t.Errorf("Token count %d is less than minimum %d", count, tc.minCount)
			}

			if count > tc.maxCount {
				t.Errorf("Token count %d exceeds maximum %d", count, tc.maxCount)
			}
		})
	}
}

func TestGetModel(t *testing.T) {
	tests := []struct {
		name          string
		providedModel string
		expectDefault bool
	}{
		{
			name:          "default model",
			providedModel: "",
			expectDefault: true,
		},
		{
			name:          "sonnet model",
			providedModel: "claude-sonnet-4-5-20250929",
			expectDefault: false,
		},
		{
			name:          "opus model",
			providedModel: "claude-opus-4-5-20251101",
			expectDefault: false,
		},
		{
			name:          "haiku model",
			providedModel: "claude-3-5-haiku-20241022",
			expectDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAnthropicProvider("test-key-123", tt.providedModel, testLogger())
			if err != nil {
				t.Fatalf("NewAnthropicProvider failed: %v", err)
			}

			model := provider.GetModel()

			if tt.expectDefault {
				if model == "" {
					t.Error("Expected default model to be set")
				}
			} else {
				if model != tt.providedModel {
					t.Errorf("Expected model %s, got %s", tt.providedModel, model)
				}
			}
		})
	}
}

// Note: Ask() and AskWithCache() methods require actual API calls
// and are tested through integration tests or with actual API keys.
// For unit tests, these would require mocking the HTTP client,
// which is beyond the scope of basic unit testing.
// The Agent tests will test these methods with mocked providers.
