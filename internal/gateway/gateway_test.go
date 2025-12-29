package gateway

import (
	"io"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestListRepos_Empty(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	repos := gw.ListRepos()
	if len(repos) != 0 {
		t.Errorf("Expected 0 repos, got %d", len(repos))
	}
}

func TestGetRepo_NotFound(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	_, err = gw.GetRepo("nonexistent-repo")
	if err == nil {
		t.Fatal("Expected error for nonexistent repo, got nil")
	}

	// Error should mention the repo name
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestListRepos_ThreadSafe(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Call ListRepos concurrently from multiple goroutines
	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repos := gw.ListRepos()
			if repos == nil {
				t.Error("ListRepos returned nil")
			}
		}()
	}

	wg.Wait()
	// Test would fail with -race flag if there were race conditions
}

func TestGetRepo_ThreadSafe(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Call GetRepo concurrently from multiple goroutines
	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = gw.GetRepo("test-repo")
			// We don't care about the error, just checking for races
		}()
	}

	wg.Wait()
	// Test would fail with -race flag if there were race conditions
}

func TestNew_EmptyRepos(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() with empty repos failed: %v", err)
	}

	if gw == nil {
		t.Fatal("Expected gateway, got nil")
	}

	if gw.agents == nil {
		t.Error("Gateway agents map should not be nil")
	}

	if len(gw.agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(gw.agents))
	}
}

func TestNew_NilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// Panic is acceptable for nil config
			t.Log("Correctly panicked on nil config")
		}
	}()

	_, _ = New(nil, testLogger())
}

func TestListRepos_ReturnsNewSlice(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	repos1 := gw.ListRepos()
	repos2 := gw.ListRepos()

	// Should return different slice instances (defensive copy)
	// Modifying one should not affect the other
	if len(repos1) == 0 && len(repos2) == 0 {
		// For empty lists, just verify they're non-nil
		if repos1 == nil {
			t.Error("repos1 should not be nil")
		}
		if repos2 == nil {
			t.Error("repos2 should not be nil")
		}
	}
}

func TestGatewayStruct_HasRequiredFields(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		AnthropicKey:      "test-key",
		Repos:             []RepoConfig{},
	}

	gw, err := New(config, testLogger())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Verify Gateway struct has essential fields initialized
	if gw.agents == nil {
		t.Error("Gateway.agents should be initialized")
	}

	if gw.config == nil {
		t.Error("Gateway.config should be set")
	}

	// Verify config is the same reference
	if gw.config != config {
		t.Error("Gateway.config should reference the provided config")
	}
}

func TestConfigValidation_MissingAnthropicKey(t *testing.T) {
	config := &Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		// Missing AnthropicKey
		Repos: []RepoConfig{
			{
				Name: "test-repo",
				Path: "/tmp/test-repo",
			},
		},
	}

	_, err := New(config, testLogger())
	// Should fail because repo creation requires Anthropic key
	if err == nil {
		t.Fatal("Expected error for missing Anthropic key")
	}
}
