package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		EmbeddingModel:    "nomic-embed-text",
		LLMProvider:       "anthropic",
		LLMModel:          "claude-sonnet-4-5-20250929",
		AnthropicKey:      "sk-ant-test-key-123",
		Repos: []RepoConfig{
			{Name: "repo1", Path: "/tmp/repo1"},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Valid config should not return error, got: %v", err)
	}
}

func TestValidate_MissingPort(t *testing.T) {
	config := &Config{
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos:             []RepoConfig{{Name: "repo1", Path: "/tmp/repo1"}},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing port")
	}
}

func TestValidate_MissingQdrantURL(t *testing.T) {
	config := &Config{
		Port:              8080,
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos:             []RepoConfig{{Name: "repo1", Path: "/tmp/repo1"}},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing qdrant_url")
	}
}

func TestValidate_MissingEmbeddingProvider(t *testing.T) {
	config := &Config{
		Port:        8080,
		QdrantURL:   "http://localhost:6333",
		LLMProvider: "anthropic",
		Repos:       []RepoConfig{{Name: "repo1", Path: "/tmp/repo1"}},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing embedding_provider")
	}
}

func TestValidate_MissingLLMProvider(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		Repos:             []RepoConfig{{Name: "repo1", Path: "/tmp/repo1"}},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for missing llm_provider")
	}
}

func TestValidate_NoRepos(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos:             []RepoConfig{},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for empty repos")
	}

	if !contains(err.Error(), "repository") {
		t.Error("Error should mention repository requirement")
	}
}

func TestValidate_RepoMissingName(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos: []RepoConfig{
			{Path: "/tmp/repo1"}, // Missing Name
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for repo missing name")
	}

	if !contains(err.Error(), "name") {
		t.Error("Error should mention missing name")
	}
}

func TestValidate_RepoMissingPath(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos: []RepoConfig{
			{Name: "repo1"}, // Missing Path
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for repo missing path")
	}

	if !contains(err.Error(), "path") {
		t.Error("Error should mention missing path")
	}
}

func TestValidate_MultipleRepos(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos: []RepoConfig{
			{Name: "repo1", Path: "/tmp/repo1"},
			{Name: "repo2", Path: "/tmp/repo2"},
			{Name: "repo3", Path: "/tmp/repo3"},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Multiple repos should be valid, got error: %v", err)
	}
}

func TestValidate_RepoWithFocusPaths(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos: []RepoConfig{
			{
				Name:       "repo1",
				Path:       "/tmp/repo1",
				FocusPaths: []string{"internal/", "pkg/"},
			},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Config with focus paths should be valid, got error: %v", err)
	}
}

func TestValidate_RepoWithPersonality(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		LLMProvider:       "anthropic",
		Repos: []RepoConfig{
			{
				Name:        "repo1",
				Path:        "/tmp/repo1",
				Personality: "You are a Go expert.",
			},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Config with personality should be valid, got error: %v", err)
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/gateway-config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gateway.yaml")

	configYAML := `port: 8080
qdrant_url: http://localhost:6333
embedding_provider: ollama
embedding_model: nomic-embed-text
ollama_url: http://localhost:11434
llm_provider: anthropic
llm_model: claude-sonnet-4-5-20250929
anthropic_key: sk-ant-test-key-123
repos:
  - name: repo1
    path: /tmp/repo1
  - name: repo2
    path: /tmp/repo2
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Port != 8080 {
		t.Errorf("Expected Port 8080, got %d", config.Port)
	}

	if config.QdrantURL != "http://localhost:6333" {
		t.Errorf("Expected QdrantURL 'http://localhost:6333', got '%s'", config.QdrantURL)
	}

	if len(config.Repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(config.Repos))
	}

	if config.Repos[0].Name != "repo1" {
		t.Errorf("Expected first repo name 'repo1', got '%s'", config.Repos[0].Name)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `this is: not: valid: yaml
  - structure
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

func TestLoadConfig_EnvironmentVariableOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-from-env-456")
	os.Setenv("OPENAI_API_KEY", "sk-openai-from-env-789")
	defer func() {
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gateway.yaml")

	// Config without API keys (will use env vars)
	configYAML := `port: 8080
qdrant_url: http://localhost:6333
embedding_provider: openai
llm_provider: anthropic
repos:
  - name: repo1
    path: /tmp/repo1
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.AnthropicKey != "sk-ant-from-env-456" {
		t.Errorf("Expected Anthropic key from env, got '%s'", config.AnthropicKey)
	}

	if config.OpenAIKey != "sk-openai-from-env-789" {
		t.Errorf("Expected OpenAI key from env, got '%s'", config.OpenAIKey)
	}
}

func TestRepoConfig_Fields(t *testing.T) {
	repo := RepoConfig{
		Name:        "test-repo",
		Path:        "/tmp/test-repo",
		FocusPaths:  []string{"src/", "lib/"},
		Personality: "Custom personality text",
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected Name 'test-repo', got '%s'", repo.Name)
	}

	if repo.Path != "/tmp/test-repo" {
		t.Errorf("Expected Path '/tmp/test-repo', got '%s'", repo.Path)
	}

	if len(repo.FocusPaths) != 2 {
		t.Errorf("Expected 2 focus paths, got %d", len(repo.FocusPaths))
	}

	if repo.Personality != "Custom personality text" {
		t.Errorf("Expected custom personality, got '%s'", repo.Personality)
	}
}

// Helper function
func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
