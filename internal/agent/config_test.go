package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Valid config should not return error, got: %v", err)
	}
}

func TestValidate_MissingRepoPath(t *testing.T) {
	config := &Config{
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing repo_path")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestValidate_MissingRepoName(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		AnthropicKey: "sk-ant-test-key-123",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing repo_name")
	}
}

func TestValidate_MissingAnthropicKey(t *testing.T) {
	config := &Config{
		RepoPath: "/tmp/test-repo",
		RepoName: "test-repo",
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing anthropic_key")
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	config := &Config{
		// Missing RepoPath, RepoName, and AnthropicKey
		CostLimits: CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for multiple missing fields")
	}

	// Error should list multiple issues
	errMsg := err.Error()
	if !contains(errMsg, "repo_path") {
		t.Error("Error should mention missing repo_path")
	}

	if !contains(errMsg, "repo_name") {
		t.Error("Error should mention missing repo_name")
	}

	if !contains(errMsg, "anthropic_key") {
		t.Error("Error should mention missing anthropic_key")
	}
}

func TestValidate_SetsCostLimitDefaults(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123",
		// CostLimits left empty
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should set default DailyMaxUSD
	if config.CostLimits.DailyMaxUSD == 0 {
		t.Error("Expected default DailyMaxUSD to be set")
	}

	// Should set default PerQueryMaxTokens
	if config.CostLimits.PerQueryMaxTokens == 0 {
		t.Error("Expected default PerQueryMaxTokens to be set")
	}

	// Should set default AlertThresholdUSD (80% of DailyMaxUSD)
	if config.CostLimits.AlertThresholdUSD == 0 {
		t.Error("Expected default AlertThresholdUSD to be set")
	}

	expectedAlert := config.CostLimits.DailyMaxUSD * 0.8
	if config.CostLimits.AlertThresholdUSD != expectedAlert {
		t.Errorf("Expected AlertThresholdUSD %.2f, got %.2f", expectedAlert, config.CostLimits.AlertThresholdUSD)
	}
}

func TestValidate_CustomCostLimits(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123",
		CostLimits: CostLimits{
			DailyMaxUSD:       50.0,
			AlertThresholdUSD: 40.0,
			PerQueryMaxTokens: 200000,
		},
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Custom values should be preserved
	if config.CostLimits.DailyMaxUSD != 50.0 {
		t.Errorf("Expected DailyMaxUSD 50.0, got %.2f", config.CostLimits.DailyMaxUSD)
	}

	if config.CostLimits.AlertThresholdUSD != 40.0 {
		t.Errorf("Expected AlertThresholdUSD 40.0, got %.2f", config.CostLimits.AlertThresholdUSD)
	}

	if config.CostLimits.PerQueryMaxTokens != 200000 {
		t.Errorf("Expected PerQueryMaxTokens 200000, got %d", config.CostLimits.PerQueryMaxTokens)
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `repo_path: /tmp/test-repo
repo_name: test-repo
anthropic_key: sk-ant-test-key-123
cost_limits:
  daily_max_usd: 10.0
  alert_threshold_usd: 8.0
  per_query_max_tokens: 100000
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.RepoPath != "/tmp/test-repo" {
		t.Errorf("Expected RepoPath '/tmp/test-repo', got '%s'", config.RepoPath)
	}

	if config.RepoName != "test-repo" {
		t.Errorf("Expected RepoName 'test-repo', got '%s'", config.RepoName)
	}

	if config.AnthropicKey != "sk-ant-test-key-123" {
		t.Errorf("Expected AnthropicKey 'sk-ant-test-key-123', got '%s'", config.AnthropicKey)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `this is not
valid: yaml: content
  - with: bad: syntax
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfig_EnvironmentVariableExpansion(t *testing.T) {
	// Set environment variable
	os.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-from-env-123")
	defer os.Unsetenv("TEST_ANTHROPIC_KEY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `repo_path: /tmp/test-repo
repo_name: test-repo
anthropic_key: ${TEST_ANTHROPIC_KEY}
cost_limits:
  daily_max_usd: 10.0
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.AnthropicKey != "sk-ant-from-env-123" {
		t.Errorf("Expected env var expansion, got '%s'", config.AnthropicKey)
	}
}

func TestLoadConfig_WithQdrantAndEmbedding(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `repo_path: /tmp/test-repo
repo_name: test-repo
anthropic_key: sk-ant-test-key-123
qdrant_url: http://localhost:6333
embedding_provider: ollama
ollama_url: http://localhost:11434
ollama_model: nomic-embed-text
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.QdrantURL != "http://localhost:6333" {
		t.Errorf("Expected QdrantURL 'http://localhost:6333', got '%s'", config.QdrantURL)
	}

	if config.EmbeddingProvider != "ollama" {
		t.Errorf("Expected EmbeddingProvider 'ollama', got '%s'", config.EmbeddingProvider)
	}

	if config.OllamaURL != "http://localhost:11434" {
		t.Errorf("Expected OllamaURL 'http://localhost:11434', got '%s'", config.OllamaURL)
	}

	if config.OllamaModel != "nomic-embed-text" {
		t.Errorf("Expected OllamaModel 'nomic-embed-text', got '%s'", config.OllamaModel)
	}
}

func TestCostLimits_DefaultValues(t *testing.T) {
	config := &Config{
		RepoPath:     "/tmp/test-repo",
		RepoName:     "test-repo",
		AnthropicKey: "sk-ant-test-key-123",
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Check defaults are reasonable
	if config.CostLimits.DailyMaxUSD <= 0 {
		t.Error("Default DailyMaxUSD should be positive")
	}

	if config.CostLimits.PerQueryMaxTokens <= 0 {
		t.Error("Default PerQueryMaxTokens should be positive")
	}

	if config.CostLimits.AlertThresholdUSD <= 0 {
		t.Error("Default AlertThresholdUSD should be positive")
	}

	// Alert should be less than daily max
	if config.CostLimits.AlertThresholdUSD >= config.CostLimits.DailyMaxUSD {
		t.Error("AlertThresholdUSD should be less than DailyMaxUSD")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && anyIndex(s, substr))
}

func anyIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
