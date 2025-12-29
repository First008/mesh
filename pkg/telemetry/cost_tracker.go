// Package telemetry provides API cost tracking and budget limits.
//
// The telemetry package implements cost tracking for LLM API usage with
// daily limits, alert thresholds, and automatic resets. It supports
// prompt caching cost reductions and per-model pricing.
package telemetry

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// PricingTable holds pricing information for different AI providers and models
type PricingTable struct {
	InputPricePerMToken  float64
	OutputPricePerMToken float64
}

// Anthropic pricing (as of December 2025)
// Supports both versioned models (claude-sonnet-4-5-20250929) and base names (claude-sonnet-4.5)
var AnthropicPricing = map[string]PricingTable{
	// Opus 4.5
	"claude-opus-4.5":          {InputPricePerMToken: 5.00, OutputPricePerMToken: 25.00},
	"claude-opus-4-5-20251101": {InputPricePerMToken: 5.00, OutputPricePerMToken: 25.00},

	// Sonnet 4.5
	"claude-sonnet-4.5":          {InputPricePerMToken: 3.00, OutputPricePerMToken: 15.00},
	"claude-sonnet-4-5-20250929": {InputPricePerMToken: 3.00, OutputPricePerMToken: 15.00},

	// Haiku 3.5
	"claude-haiku-3.5":          {InputPricePerMToken: 0.80, OutputPricePerMToken: 4.00},
	"claude-3-5-haiku-20241022": {InputPricePerMToken: 0.80, OutputPricePerMToken: 4.00},
}

// CostTracker tracks API costs and enforces limits
type CostTracker struct {
	mu sync.RWMutex

	dailyMaxUSD       float64
	alertThresholdUSD float64
	perQueryMaxTokens int

	// Daily tracking (resets at midnight)
	dailySpend        float64
	dailyInputTokens  int64
	dailyOutputTokens int64
	dailyCachedTokens int64
	dailyRequestCount int
	lastResetDate     string

	// Overall tracking
	totalSpend        float64
	totalInputTokens  int64
	totalOutputTokens int64
	totalCachedTokens int64
	totalRequestCount int

	logger zerolog.Logger
}

// NewCostTracker creates a new cost tracker with specified limits
func NewCostTracker(dailyMaxUSD, alertThresholdUSD float64, perQueryMaxTokens int, logger zerolog.Logger) *CostTracker {
	return &CostTracker{
		dailyMaxUSD:       dailyMaxUSD,
		alertThresholdUSD: alertThresholdUSD,
		perQueryMaxTokens: perQueryMaxTokens,
		lastResetDate:     time.Now().Format("2006-01-02"),
		logger:            logger,
	}
}

// RecordRequest records a request and its costs
func (ct *CostTracker) RecordRequest(model string, inputTokens, outputTokens, cachedTokens int) (float64, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Check daily reset
	ct.checkDailyReset()

	// Get pricing for model
	pricing, ok := AnthropicPricing[model]
	if !ok {
		return 0, fmt.Errorf("unknown model: %s", model)
	}

	// Calculate cost
	// Cached tokens cost 90% less (10% of regular price)
	cachedCost := float64(cachedTokens) / 1_000_000 * pricing.InputPricePerMToken * 0.1
	inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPricePerMToken
	outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPricePerMToken
	totalCost := cachedCost + inputCost + outputCost

	// Check if this would exceed daily limit
	if ct.dailySpend+totalCost > ct.dailyMaxUSD {
		return 0, fmt.Errorf("daily cost limit exceeded: current=$%.2f, limit=$%.2f, this request=$%.2f",
			ct.dailySpend, ct.dailyMaxUSD, totalCost)
	}

	// Check per-query token limit
	totalTokens := inputTokens + outputTokens + cachedTokens
	if totalTokens > ct.perQueryMaxTokens {
		return 0, fmt.Errorf("per-query token limit exceeded: tokens=%d, limit=%d",
			totalTokens, ct.perQueryMaxTokens)
	}

	// Update tracking
	ct.dailySpend += totalCost
	ct.dailyInputTokens += int64(inputTokens)
	ct.dailyOutputTokens += int64(outputTokens)
	ct.dailyCachedTokens += int64(cachedTokens)
	ct.dailyRequestCount++

	ct.totalSpend += totalCost
	ct.totalInputTokens += int64(inputTokens)
	ct.totalOutputTokens += int64(outputTokens)
	ct.totalCachedTokens += int64(cachedTokens)
	ct.totalRequestCount++

	// Log cost information
	ct.logger.Info().
		Str("model", model).
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Int("cached_tokens", cachedTokens).
		Float64("cost_usd", totalCost).
		Float64("daily_spend_usd", ct.dailySpend).
		Float64("cache_savings_usd", cachedCost*9). // Savings vs non-cached
		Msg("API request cost recorded")

	// Check alert threshold
	if ct.dailySpend >= ct.alertThresholdUSD && ct.dailySpend-totalCost < ct.alertThresholdUSD {
		ct.logger.Warn().
			Float64("daily_spend_usd", ct.dailySpend).
			Float64("alert_threshold_usd", ct.alertThresholdUSD).
			Float64("daily_max_usd", ct.dailyMaxUSD).
			Msg("Daily cost alert threshold reached")
	}

	return totalCost, nil
}

// checkDailyReset resets daily counters if the date has changed
func (ct *CostTracker) checkDailyReset() {
	today := time.Now().Format("2006-01-02")
	if today != ct.lastResetDate {
		ct.logger.Info().
			Float64("previous_daily_spend_usd", ct.dailySpend).
			Int64("previous_daily_requests", int64(ct.dailyRequestCount)).
			Msg("Daily cost tracking reset")

		ct.dailySpend = 0
		ct.dailyInputTokens = 0
		ct.dailyOutputTokens = 0
		ct.dailyCachedTokens = 0
		ct.dailyRequestCount = 0
		ct.lastResetDate = today
	}
}

// GetDailyStats returns current daily statistics
func (ct *CostTracker) GetDailyStats() DailyStats {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.checkDailyReset()

	return DailyStats{
		SpendUSD:     ct.dailySpend,
		InputTokens:  ct.dailyInputTokens,
		OutputTokens: ct.dailyOutputTokens,
		CachedTokens: ct.dailyCachedTokens,
		RequestCount: ct.dailyRequestCount,
		LimitUSD:     ct.dailyMaxUSD,
		RemainingUSD: ct.dailyMaxUSD - ct.dailySpend,
	}
}

// GetTotalStats returns overall statistics
func (ct *CostTracker) GetTotalStats() TotalStats {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	return TotalStats{
		TotalSpendUSD:     ct.totalSpend,
		TotalInputTokens:  ct.totalInputTokens,
		TotalOutputTokens: ct.totalOutputTokens,
		TotalCachedTokens: ct.totalCachedTokens,
		TotalRequests:     ct.totalRequestCount,
	}
}

// DailyStats holds daily cost statistics
type DailyStats struct {
	SpendUSD     float64
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	RequestCount int
	LimitUSD     float64
	RemainingUSD float64
}

// TotalStats holds overall cost statistics
type TotalStats struct {
	TotalSpendUSD     float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCachedTokens int64
	TotalRequests     int
}
