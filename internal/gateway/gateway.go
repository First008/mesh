// Package gateway provides multi-repository coordination and management.
//
// The gateway package implements the Gateway type that manages multiple repository
// agents as a single unified service, enabling efficient resource usage and
// centralized routing for queries across multiple codebases.
package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/First008/mesh/internal/agent"
	"github.com/First008/mesh/internal/factory"
	"github.com/First008/mesh/internal/llm"
	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

// Gateway orchestrates multiple repository agents
// Each repo gets its own Agent instance (reusing existing code!)
type Gateway struct {
	agents  map[string]*agent.Agent // repo name -> agent
	config  *Config
	scanner *BranchScanner // Periodic branch scanner
	mu      sync.RWMutex
	logger  zerolog.Logger
}

// New creates a new gateway with the given configuration
func New(config *Config, logger zerolog.Logger) (*Gateway, error) {
	gw := &Gateway{
		agents: make(map[string]*agent.Agent),
		config: config,
		logger: logger,
	}

	// Initialize agents for each repo
	for _, repoConfig := range config.Repos {
		if err := gw.addRepo(repoConfig); err != nil {
			return nil, fmt.Errorf("failed to add repo %s: %w", repoConfig.Name, err)
		}
	}

	logger.Info().
		Int("repo_count", len(config.Repos)).
		Msg("Gateway initialized with repositories")

	return gw, nil
}

// StartScanner starts the periodic branch scanner
func (gw *Gateway) StartScanner(ctx context.Context, interval time.Duration) {
	gw.scanner = NewBranchScanner(gw, interval, gw.logger)
	gw.scanner.Start(ctx)
}

// addRepo creates an agent for a repository
// Refactored into smaller methods for better maintainability
func (gw *Gateway) addRepo(repoConfig RepoConfig) error {
	repoLogger := gw.logger.With().Str("repo", repoConfig.Name).Logger()

	// Build agent config
	agentConfig := gw.buildAgentConfig(repoConfig)

	// Create agent
	agt, err := agent.New(agentConfig, repoLogger)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Detect branch
	branch := gw.detectBranch(repoConfig.Path)

	// Perform indexing if needed
	if gw.shouldIndex(repoConfig.Path) {
		if err := gw.performIndexing(repoConfig, branch, agt, repoLogger); err != nil {
			repoLogger.Warn().Err(err).Msg("Indexing failed, continuing without vector search")
		}
	}

	// Register agent
	gw.registerAgent(repoConfig.Name, agt)

	repoLogger.Info().
		Str("branch", branch).
		Msg("Repository agent initialized")

	return nil
}

// buildAgentConfig constructs agent.Config from gateway and repo config
func (gw *Gateway) buildAgentConfig(repoConfig RepoConfig) *agent.Config {
	return &agent.Config{
		RepoPath:          repoConfig.Path,
		RepoName:          repoConfig.Name,
		FocusPaths:        repoConfig.FocusPaths,
		Personality:       repoConfig.Personality,
		ExcludePatterns:   repoConfig.ExcludePatterns,
		Port:              gw.config.Port,
		AnthropicKey:      gw.config.AnthropicKey,
		OpenAIKey:         gw.config.OpenAIKey,
		QdrantURL:         gw.config.QdrantURL,
		EmbeddingProvider: gw.config.EmbeddingProvider,
		OllamaURL:         gw.config.OllamaURL,
		OllamaModel:       gw.config.EmbeddingModel,
		CostLimits: agent.CostLimits{
			DailyMaxUSD:       100.0,
			PerQueryMaxTokens: 100000,
			AlertThresholdUSD: 80.0,
		},
	}
}

// detectBranch detects the current git branch for a repository
func (gw *Gateway) detectBranch(repoPath string) string {
	branch := "main"
	if vectorstore.IsGitRepo(repoPath) {
		if b, err := vectorstore.GetCurrentBranch(repoPath); err == nil && b != "" {
			branch = b
		}
	}
	return branch
}

// shouldIndex checks if indexing should be performed
func (gw *Gateway) shouldIndex(repoPath string) bool {
	return gw.config.QdrantURL != "" && vectorstore.IsGitRepo(repoPath)
}

// performIndexing creates vector store and indexes the repository
func (gw *Gateway) performIndexing(repoConfig RepoConfig, branch string, agt *agent.Agent, logger zerolog.Logger) error {
	// Create embedding provider
	embeddingProvider, err := factory.NewEmbeddingProvider(
		factory.EmbeddingConfig{
			Provider:    gw.config.EmbeddingProvider,
			OpenAIKey:   gw.config.OpenAIKey,
			OllamaURL:   gw.config.OllamaURL,
			OllamaModel: gw.config.EmbeddingModel,
		},
		logger,
	)
	if err != nil {
		return fmt.Errorf("create embedding provider: %w", err)
	}

	// Create vector store
	store, err := vectorstore.NewQdrantStoreWithBranch(
		gw.config.QdrantURL,
		embeddingProvider,
		repoConfig.Name,
		branch,
		logger,
	)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}

	// Update agent to use branch-aware vector store
	agt.SetVectorStore(store)
	logger.Info().Str("branch", branch).Msg("Updated agent to use branch-aware vector store")

	// Create indexer
	indexer := vectorstore.NewIndexerWithBranch(
		store,
		repoConfig.Path,
		repoConfig.Name,
		branch,
		logger,
	)

	// Perform indexing
	logger.Info().Str("branch", branch).Msg("Starting incremental indexing")
	if err := indexer.IndexIncremental(context.Background()); err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	logger.Info().Str("branch", branch).Msg("Repository indexed successfully")
	return nil
}

// registerAgent stores the agent in the gateway's agent map (thread-safe)
func (gw *Gateway) registerAgent(repoName string, agt *agent.Agent) {
	gw.mu.Lock()
	gw.agents[repoName] = agt
	gw.mu.Unlock()
}

// Ask sends a question to a specific repository agent
func (gw *Gateway) Ask(ctx context.Context, repoName, question string) (*llm.Response, error) {
	gw.mu.RLock()
	agt, exists := gw.agents[repoName]
	gw.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository not found: %s", repoName)
	}

	return agt.Ask(ctx, question)
}

// AskAll sends a question to all repository agents and aggregates responses
func (gw *Gateway) AskAll(ctx context.Context, question string) (map[string]*llm.Response, error) {
	gw.mu.RLock()
	repos := make([]string, 0, len(gw.agents))
	for name := range gw.agents {
		repos = append(repos, name)
	}
	gw.mu.RUnlock()

	results := make(map[string]*llm.Response)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))

	for _, repoName := range repos {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			resp, err := gw.Ask(ctx, name, question)
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", name, err)
				return
			}

			mu.Lock()
			results[name] = resp
			mu.Unlock()
		}(repoName)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("errors from %d repos: %v", len(errs), errs)
	}

	return results, nil
}

// ListRepos returns the list of configured repositories
func (gw *Gateway) ListRepos() []string {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	repos := make([]string, 0, len(gw.agents))
	for name := range gw.agents {
		repos = append(repos, name)
	}
	return repos
}

// GetRepo returns information about a specific repository
func (gw *Gateway) GetRepo(name string) (*RepoInfo, error) {
	gw.mu.RLock()
	_, exists := gw.agents[name]
	gw.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository not found: %s", name)
	}

	// Find repo config
	var repoConfig *RepoConfig
	for i := range gw.config.Repos {
		if gw.config.Repos[i].Name == name {
			repoConfig = &gw.config.Repos[i]
			break
		}
	}

	if repoConfig == nil {
		return nil, fmt.Errorf("repository config not found: %s", name)
	}

	// Get branch info
	branch := "main"
	if vectorstore.IsGitRepo(repoConfig.Path) {
		if b, err := vectorstore.GetCurrentBranch(repoConfig.Path); err == nil {
			branch = b
		}
	}

	return &RepoInfo{
		Name:   repoConfig.Name,
		Path:   repoConfig.Path,
		Branch: branch,
	}, nil
}

// ReindexRepo triggers incremental re-indexing for a specific repository (current branch)
func (gw *Gateway) ReindexRepo(ctx context.Context, repoName string) error {
	// Find repo config
	var repoConfig *RepoConfig
	for i := range gw.config.Repos {
		if gw.config.Repos[i].Name == repoName {
			repoConfig = &gw.config.Repos[i]
			break
		}
	}

	if repoConfig == nil {
		return fmt.Errorf("repository config not found: %s", repoName)
	}

	// Detect current branch
	branch := "main"
	if vectorstore.IsGitRepo(repoConfig.Path) {
		if b, err := vectorstore.GetCurrentBranch(repoConfig.Path); err == nil && b != "" {
			branch = b
		}
	}

	return gw.ReindexBranch(ctx, repoName, branch)
}

// ReindexBranch triggers incremental re-indexing for a specific repository branch
func (gw *Gateway) ReindexBranch(ctx context.Context, repoName, branch string) error {
	// Find repo config
	var repoConfig *RepoConfig
	for i := range gw.config.Repos {
		if gw.config.Repos[i].Name == repoName {
			repoConfig = &gw.config.Repos[i]
			break
		}
	}

	if repoConfig == nil {
		return fmt.Errorf("repository config not found: %s", repoName)
	}

	repoLogger := gw.logger.With().
		Str("repo", repoName).
		Str("branch", branch).
		Logger()

	// Create embedding provider using factory (eliminates duplication)
	embeddingProvider, err := factory.NewEmbeddingProvider(
		factory.EmbeddingConfig{
			Provider:    gw.config.EmbeddingProvider,
			OpenAIKey:   gw.config.OpenAIKey,
			OllamaURL:   gw.config.OllamaURL,
			OllamaModel: gw.config.EmbeddingModel,
		},
		repoLogger,
	)
	if err != nil {
		return fmt.Errorf("create embedding provider: %w", err)
	}

	// Create vector store for this branch
	store, err := vectorstore.NewQdrantStoreWithBranch(
		gw.config.QdrantURL,
		embeddingProvider,
		repoConfig.Name,
		branch,
		repoLogger,
	)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}

	// Create indexer
	indexer := vectorstore.NewIndexerWithBranch(
		store,
		repoConfig.Path,
		repoConfig.Name,
		branch,
		repoLogger,
	)

	// Perform incremental indexing
	repoLogger.Info().Msg("Triggering incremental re-index")
	return indexer.IndexIncremental(ctx)
}

// Close closes all agents and releases resources
func (gw *Gateway) Close() error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	// Stop the scanner if running
	if gw.scanner != nil {
		gw.scanner.Stop()
	}

	gw.logger.Info().Msg("Gateway closing")

	return nil
}

// RepoInfo contains information about a repository
type RepoInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}
