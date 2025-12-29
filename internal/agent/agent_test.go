package agent

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestNew_Success(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent, got nil")
	}

	// Verify agent was initialized properly
	if agent.config == nil {
		t.Error("Agent config is nil")
	}

	if agent.llmProvider == nil {
		t.Error("Agent LLM provider is nil")
	}

	if agent.contextBuilder == nil {
		t.Error("Agent context builder is nil")
	}

	if agent.costTracker == nil {
		t.Error("Agent cost tracker is nil")
	}

	if agent.personality == nil {
		t.Error("Agent personality is nil")
	}
}

func TestNew_MissingAnthropicKey(t *testing.T) {
	config := &Config{
		RepoPath: "/tmp/test-repo",
		RepoName: "test-repo",
		// Missing AnthropicKey
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	_, err := New(config, testLogger())
	if err == nil {
		t.Fatal("Expected error for missing Anthropic key, got nil")
	}

	// Should mention LLM provider or API key
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestGetRepoName(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "my-awesome-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	repoName := agent.GetRepoName()
	if repoName != "my-awesome-repo" {
		t.Errorf("Expected repo name 'my-awesome-repo', got '%s'", repoName)
	}
}

func TestGetModel(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	model := agent.GetModel()
	if model == "" {
		t.Error("Expected non-empty model name")
	}

	// Should be a Claude model
	if len(model) < 10 {
		t.Errorf("Model name seems invalid: %s", model)
	}
}

func TestGetDailyStats_InitialState(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		CostLimits: CostLimits{
			DailyMaxUSD:       100.0,
			AlertThresholdUSD: 80.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	stats := agent.GetDailyStats()

	// Initial state should have no spend
	if stats.SpendUSD != 0.0 {
		t.Errorf("Expected initial spend 0.0, got %.2f", stats.SpendUSD)
	}

	if stats.InputTokens != 0 {
		t.Errorf("Expected 0 input tokens, got %d", stats.InputTokens)
	}

	if stats.OutputTokens != 0 {
		t.Errorf("Expected 0 output tokens, got %d", stats.OutputTokens)
	}

	if stats.RequestCount != 0 {
		t.Errorf("Expected 0 requests, got %d", stats.RequestCount)
	}

	if stats.LimitUSD != 100.0 {
		t.Errorf("Expected limit 100.0, got %.2f", stats.LimitUSD)
	}

	if stats.RemainingUSD != 100.0 {
		t.Errorf("Expected remaining 100.0, got %.2f", stats.RemainingUSD)
	}
}

func TestGetTotalStats_InitialState(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		CostLimits: CostLimits{
			DailyMaxUSD:       100.0,
			AlertThresholdUSD: 80.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	stats := agent.GetTotalStats()

	// Initial state should have no activity
	if stats.TotalSpendUSD != 0.0 {
		t.Errorf("Expected total spend 0.0, got %.2f", stats.TotalSpendUSD)
	}

	if stats.TotalInputTokens != 0 {
		t.Errorf("Expected 0 total input tokens, got %d", stats.TotalInputTokens)
	}

	if stats.TotalOutputTokens != 0 {
		t.Errorf("Expected 0 total output tokens, got %d", stats.TotalOutputTokens)
	}

	if stats.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests, got %d", stats.TotalRequests)
	}
}

func TestNew_WithCustomPersonality(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		Personality:  "You are a helpful Go programming expert.",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if agent.personality == nil {
		t.Fatal("Personality is nil")
	}

	// Personality should be set
	systemPrompt := agent.personality.GetSystemPrompt()
	if systemPrompt == "" {
		t.Error("Expected non-empty system prompt")
	}
}

func TestNew_WithFocusPaths(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123456789",
		FocusPaths:   []string{"internal/", "pkg/"},
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	agent, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if agent.personality == nil {
		t.Fatal("Personality is nil")
	}

	// Focus paths should be incorporated into personality
	systemPrompt := agent.personality.GetSystemPrompt()
	if systemPrompt == "" {
		t.Error("Expected non-empty system prompt with focus paths")
	}
}

func TestNew_WithDifferentCostLimits(t *testing.T) {
	testCases := []struct {
		name              string
		dailyMax          float64
		alertThreshold    float64
		perQueryMaxTokens int
	}{
		{
			name:              "low limits",
			dailyMax:          1.0,
			alertThreshold:    0.8,
			perQueryMaxTokens: 10000,
		},
		{
			name:              "medium limits",
			dailyMax:          50.0,
			alertThreshold:    40.0,
			perQueryMaxTokens: 100000,
		},
		{
			name:              "high limits",
			dailyMax:          500.0,
			alertThreshold:    400.0,
			perQueryMaxTokens: 500000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				RepoPath:     "/tmp/test-repo",
				RepoName:     "test-repo",
				AnthropicKey: "sk-ant-test-key-123456789",
				CostLimits: CostLimits{
					DailyMaxUSD:       tc.dailyMax,
					AlertThresholdUSD: tc.alertThreshold,
					PerQueryMaxTokens: tc.perQueryMaxTokens,
				},
			}

			agent, err := New(config, testLogger())
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			stats := agent.GetDailyStats()
			if stats.LimitUSD != tc.dailyMax {
				t.Errorf("Expected daily limit %.2f, got %.2f", tc.dailyMax, stats.LimitUSD)
			}
		})
	}
}

// Note: The Ask() method requires actual API calls and is difficult to unit test
// without dependency injection. After Phase 3 refactoring (adding dependency injection),
// we can add tests that use MockLLMProvider from internal/testing/mocks.go
// to test the Ask() method without making real API calls.

// Example of what Ask() tests will look like after Phase 3 refactoring:
/*
func TestAsk_WithMockLLM(t *testing.T) {
	// After Phase 3, Agent will accept injected dependencies
	mockLLM := &testing.MockLLMProvider{
		Model: "claude-sonnet-4-5-20250929",
		SupportsCaching: true,
	}

	mockVectorStore := testing.NewMockVectorStore()
	mockCostTracker := testing.NewTestCostTracker()

	agent := NewWithDependencies(config, AgentDependencies{
		LLMProvider: mockLLM,
		VectorStore: mockVectorStore,
		CostTracker: mockCostTracker,
	}, testLogger())

	response, err := agent.Ask(context.Background(), "What is this repo about?")
	if err != nil {
		t.Fatalf("Ask() failed: %v", err)
	}

	if response.Content == "" {
		t.Error("Expected non-empty response content")
	}

	if mockLLM.CallCount != 1 {
		t.Errorf("Expected 1 LLM call, got %d", mockLLM.CallCount)
	}
}
*/
