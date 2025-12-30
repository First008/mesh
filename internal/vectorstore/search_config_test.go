package vectorstore

import (
	"testing"
)

func TestDefaultSearchConfig(t *testing.T) {
	config := DefaultSearchConfig()

	// Test that defaults are set correctly
	if config.MaxTokenBudget != 60000 {
		t.Errorf("Expected MaxTokenBudget=60000, got %d", config.MaxTokenBudget)
	}

	if config.ReserveTokens != 25000 {
		t.Errorf("Expected ReserveTokens=25000, got %d", config.ReserveTokens)
	}

	if config.MinAbsoluteScore != 0.15 {
		t.Errorf("Expected MinAbsoluteScore=0.15, got %f", config.MinAbsoluteScore)
	}

	if config.ScoreDistributionPercentile != 0.75 {
		t.Errorf("Expected ScoreDistributionPercentile=0.75, got %f", config.ScoreDistributionPercentile)
	}

	if config.MinFilesAfterThreshold != 6 {
		t.Errorf("Expected MinFilesAfterThreshold=6, got %d", config.MinFilesAfterThreshold)
	}

	// Test weight sum
	weightSum := config.SemanticWeight + config.KeywordWeight + config.PathWeight + config.AggregateWeight
	if weightSum < 0.99 || weightSum > 1.01 {
		t.Errorf("Expected weight sum ≈ 1.0, got %f", weightSum)
	}
}

func TestSearchConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  SearchConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  *DefaultSearchConfig(),
			wantErr: false,
		},
		{
			name: "invalid: token budget <= reserve",
			config: SearchConfig{
				MaxTokenBudget: 5000,
				ReserveTokens:  5000,
			},
			wantErr: true,
		},
		{
			name: "invalid: score below 0",
			config: SearchConfig{
				MaxTokenBudget:   80000,
				ReserveTokens:    5000,
				MinAbsoluteScore: -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid: score above 1",
			config: SearchConfig{
				MaxTokenBudget:   80000,
				ReserveTokens:    5000,
				MinAbsoluteScore: 1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid: weights don't sum to 1.0",
			config: SearchConfig{
				MaxTokenBudget:              80000,
				ReserveTokens:               5000,
				MinAbsoluteScore:            0.15,
				ScoreDistributionPercentile: 0.90,
				MinFilesAfterThreshold:      5,
				InitialChunkLimit:           50,
				MaxFilesLimit:               15,
				SemanticWeight:              0.50, // Sum = 0.80 (invalid)
				KeywordWeight:               0.20,
				PathWeight:                  0.05,
				AggregateWeight:             0.05,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveTokenBudget(t *testing.T) {
	config := DefaultSearchConfig()
	expected := config.MaxTokenBudget - config.ReserveTokens
	if config.EffectiveTokenBudget() != expected {
		t.Errorf("Expected EffectiveTokenBudget=%d, got %d", expected, config.EffectiveTokenBudget())
	}
}
