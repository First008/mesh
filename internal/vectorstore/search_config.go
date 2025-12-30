package vectorstore

import "fmt"

// SearchConfig holds configuration for intelligent file selection with
// adaptive scoring, token budget management, and hybrid ranking.
type SearchConfig struct {
	// Token budget management
	MaxTokenBudget     int // Maximum tokens for context (default: 80000)
	ReserveTokens      int // Buffer for system prompt (default: 5000)
	OversizeChunkLimit int // Top-K chunks for oversized files (default: 5)

	// Adaptive scoring (distribution-based)
	MinAbsoluteScore            float32 // Hard floor for scores (default: 0.15)
	ScoreDistributionPercentile float32 // Percentile for threshold (default: 0.90)
	MinFilesAfterThreshold      int     // Ensure min survivors (default: 5)

	// Search limits
	InitialChunkLimit int // Initial search limit (default: 50)
	MaxFilesLimit     int // Maximum files to return (default: 15)

	// Hybrid scoring weights (dynamically adjusted)
	// These weights should sum to 1.0 for proper normalization
	SemanticWeight  float32 // Vector similarity weight (default: 0.70)
	KeywordWeight   float32 // Keyword matching weight (default: 0.15)
	PathWeight      float32 // Path relevance weight (default: 0.05)
	AggregateWeight float32 // Multi-chunk depth weight (default: 0.10)
}

// DefaultSearchConfig returns a conservative configuration optimized for recall
// with safety guards against token overflow and overly strict gating.
func DefaultSearchConfig() *SearchConfig {
	return &SearchConfig{
		// Token budget: 80K tokens (~320KB text), leaving 35K for system prompt + cacheable context
		// Cacheable layer (CLAUDE.md + README + structure) typically uses ~20-30K tokens
		MaxTokenBudget:     80000,
		ReserveTokens:      35000, // Increased from 5K to account for cacheable layer overhead
		OversizeChunkLimit: 5,     // Include top-5 chunks from large files

		// Adaptive scoring: p90-based with 0.15 floor
		MinAbsoluteScore:            0.15,
		ScoreDistributionPercentile: 0.70, // p70 instead of p90 - include more files
		MinFilesAfterThreshold:      8,    // Increase min from 5 to 8 // Ensure at least 5 files survive

		// Search parameters
		InitialChunkLimit: 50, // 5x multiplier of target files
		MaxFilesLimit:     15,

		// Hybrid weights: semantic-heavy to avoid false keyword boosts
		SemanticWeight:  0.70, // Primary signal: vector similarity
		KeywordWeight:   0.15, // Secondary: exact keyword matches
		PathWeight:      0.05, // Tie-breaker: path relevance
		AggregateWeight: 0.10, // Depth signal: multi-chunk relevance
	}
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *SearchConfig) Validate() error {
	if c.MaxTokenBudget <= c.ReserveTokens {
		return fmt.Errorf("invalid config: MaxTokenBudget must be greater than ReserveTokens")
	}
	if c.MinAbsoluteScore < 0.0 || c.MinAbsoluteScore > 1.0 {
		return fmt.Errorf("invalid config: MinAbsoluteScore must be between 0.0 and 1.0")
	}
	if c.ScoreDistributionPercentile < 0.0 || c.ScoreDistributionPercentile > 1.0 {
		return fmt.Errorf("invalid config: ScoreDistributionPercentile must be between 0.0 and 1.0")
	}
	if c.MinFilesAfterThreshold < 1 {
		return fmt.Errorf("invalid config: MinFilesAfterThreshold must be at least 1")
	}
	if c.InitialChunkLimit < c.MaxFilesLimit {
		return fmt.Errorf("invalid config: InitialChunkLimit should be at least MaxFilesLimit")
	}

	// Validate weight sum (should be close to 1.0)
	weightSum := c.SemanticWeight + c.KeywordWeight + c.PathWeight + c.AggregateWeight
	if weightSum < 0.99 || weightSum > 1.01 {
		return fmt.Errorf("invalid config: Hybrid scoring weights must sum to 1.0 (got %.2f)", weightSum)
	}

	return nil
}

// EffectiveTokenBudget returns the available token budget after reserve.
func (c *SearchConfig) EffectiveTokenBudget() int {
	return c.MaxTokenBudget - c.ReserveTokens
}
