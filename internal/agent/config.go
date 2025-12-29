package agent

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the configuration for a single agent instance
type Config struct {
	RepoPath          string     `yaml:"repo_path"`
	RepoName          string     `yaml:"repo_name"`
	FocusPaths        []string   `yaml:"focus_paths"`
	Personality       string     `yaml:"personality"`
	ExcludePatterns   []string   `yaml:"exclude_patterns"` // File patterns to exclude from search results
	Port              int        `yaml:"port"`
	AnthropicKey      string     `yaml:"anthropic_key"`
	OpenAIKey         string     `yaml:"openai_key"`
	QdrantURL         string     `yaml:"qdrant_url"`
	EmbeddingProvider string     `yaml:"embedding_provider"` // "openai" or "ollama"
	OllamaURL         string     `yaml:"ollama_url"`          // Ollama API endpoint
	OllamaModel       string     `yaml:"ollama_model"`        // Ollama embedding model
	CostLimits        CostLimits `yaml:"cost_limits"`
}

// CostLimits defines cost constraints for the agent
type CostLimits struct {
	DailyMaxUSD          float64 `yaml:"daily_max_usd"`
	PerQueryMaxTokens    int     `yaml:"per_query_max_tokens"`
	AlertThresholdUSD    float64 `yaml:"alert_threshold_usd"`
}

// LoadConfig loads agent configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config
	expanded := os.ExpandEnv(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	var errors []string

	if c.RepoPath == "" {
		errors = append(errors, "repo_path is required")
	}

	if c.RepoName == "" {
		errors = append(errors, "repo_name is required")
	}

	// AnthropicKey can be empty in YAML if it will come from env var
	// The actual validation happens when creating the provider
	if c.AnthropicKey == "" {
		// Try to get from environment
		c.AnthropicKey = os.Getenv("ANTHROPIC_API_KEY")
		if c.AnthropicKey == "" {
			errors = append(errors, "anthropic_key is required (set in config or ANTHROPIC_API_KEY env var)")
		}
	}

	// Same for OpenAI key (optional for Phase 1)
	if c.OpenAIKey == "" {
		c.OpenAIKey = os.Getenv("OPENAI_API_KEY")
	}

	if c.Port <= 0 {
		c.Port = 8080 // default port
	}

	// Set default cost limits if not specified
	if c.CostLimits.DailyMaxUSD == 0 {
		c.CostLimits.DailyMaxUSD = 10.0
	}

	if c.CostLimits.PerQueryMaxTokens == 0 {
		c.CostLimits.PerQueryMaxTokens = 100000
	}

	if c.CostLimits.AlertThresholdUSD == 0 {
		c.CostLimits.AlertThresholdUSD = c.CostLimits.DailyMaxUSD * 0.8
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}
