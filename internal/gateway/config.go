package gateway

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the gateway configuration for multi-repo setup
type Config struct {
	Port              int          `yaml:"port"`
	QdrantURL         string       `yaml:"qdrant_url"`
	EmbeddingProvider string       `yaml:"embedding_provider"` // "ollama" or "openai"
	EmbeddingModel    string       `yaml:"embedding_model"`
	OllamaURL         string       `yaml:"ollama_url,omitempty"`
	OpenAIKey         string       `yaml:"openai_key,omitempty"`
	LLMProvider       string       `yaml:"llm_provider"` // "anthropic", "ollama", "openai"
	LLMModel          string       `yaml:"llm_model"`
	AnthropicKey      string       `yaml:"anthropic_key,omitempty"`
	Repos             []RepoConfig `yaml:"repos"`
}

// RepoConfig represents configuration for a single repository
type RepoConfig struct {
	Name            string   `yaml:"name"`
	Path            string   `yaml:"path"`
	FocusPaths      []string `yaml:"focus_paths,omitempty"`
	Personality     string   `yaml:"personality,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"` // File patterns to exclude from search results
}

// LoadConfig loads gateway configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Override with environment variables if not set in YAML
	if config.AnthropicKey == "" {
		config.AnthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if config.OpenAIKey == "" {
		config.OpenAIKey = os.Getenv("OPENAI_API_KEY")
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port <= 0 {
		return fmt.Errorf("port must be greater than 0")
	}

	if c.QdrantURL == "" {
		return fmt.Errorf("qdrant_url is required")
	}

	if c.EmbeddingProvider == "" {
		return fmt.Errorf("embedding_provider is required")
	}

	if c.LLMProvider == "" {
		return fmt.Errorf("llm_provider is required")
	}

	if len(c.Repos) == 0 {
		return fmt.Errorf("at least one repository must be configured")
	}

	// Validate each repo
	for i, repo := range c.Repos {
		if repo.Name == "" {
			return fmt.Errorf("repo[%d]: name is required", i)
		}
		if repo.Path == "" {
			return fmt.Errorf("repo[%d]: path is required", i)
		}
	}

	return nil
}
