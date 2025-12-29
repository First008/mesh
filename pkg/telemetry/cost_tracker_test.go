package telemetry

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Helper to create a test logger that discards output
func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestNewCostTracker(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	if tracker == nil {
		t.Fatal("NewCostTracker returned nil")
	}

	stats := tracker.GetDailyStats()
	if stats.LimitUSD != 100.0 {
		t.Errorf("Expected daily limit 100.0, got %f", stats.LimitUSD)
	}

	if stats.SpendUSD != 0.0 {
		t.Errorf("Expected initial spend 0.0, got %f", stats.SpendUSD)
	}

	if stats.RemainingUSD != 100.0 {
		t.Errorf("Expected remaining 100.0, got %f", stats.RemainingUSD)
	}
}

func TestRecordRequest_CalculatesCosts_Sonnet(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	// Sonnet 4.5: $3.00/M input, $15.00/M output
	// 10,000 input tokens = $0.03, 5,000 output tokens = $0.075
	// Total = $0.105
	cost, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("RecordRequest failed: %v", err)
	}

	expectedCost := (10000.0/1000000)*3.00 + (5000.0/1000000)*15.00
	if cost != expectedCost {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, cost)
	}

	stats := tracker.GetDailyStats()
	if stats.SpendUSD != expectedCost {
		t.Errorf("Expected daily spend $%.6f, got $%.6f", expectedCost, stats.SpendUSD)
	}

	if stats.InputTokens != 10000 {
		t.Errorf("Expected 10000 input tokens, got %d", stats.InputTokens)
	}

	if stats.OutputTokens != 5000 {
		t.Errorf("Expected 5000 output tokens, got %d", stats.OutputTokens)
	}

	if stats.RequestCount != 1 {
		t.Errorf("Expected 1 request, got %d", stats.RequestCount)
	}
}

func TestRecordRequest_CalculatesCosts_Opus(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	// Opus 4.5: $5.00/M input, $25.00/M output
	// 10,000 input tokens = $0.05, 5,000 output tokens = $0.125
	// Total = $0.175
	cost, err := tracker.RecordRequest("claude-opus-4-5-20251101", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("RecordRequest failed: %v", err)
	}

	expectedCost := (10000.0/1000000)*5.00 + (5000.0/1000000)*25.00
	// Use tolerance for floating point comparison
	tolerance := 0.000001
	if cost < expectedCost-tolerance || cost > expectedCost+tolerance {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, cost)
	}
}

func TestRecordRequest_CalculatesCosts_Haiku(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	// Haiku 3.5: $0.80/M input, $4.00/M output
	// 10,000 input tokens = $0.008, 5,000 output tokens = $0.020
	// Total = $0.028
	cost, err := tracker.RecordRequest("claude-3-5-haiku-20241022", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("RecordRequest failed: %v", err)
	}

	expectedCost := (10000.0/1000000)*0.80 + (5000.0/1000000)*4.00
	if cost != expectedCost {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, cost)
	}
}

func TestRecordRequest_CachedTokenDiscount(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 200000, testLogger())

	// Sonnet 4.5: $3.00/M input
	// 100,000 cached tokens at 10% price = $0.03
	// 10,000 regular input tokens = $0.03
	// 5,000 output tokens = $0.075
	// Total = $0.135
	cost, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 100000)
	if err != nil {
		t.Fatalf("RecordRequest failed: %v", err)
	}

	cachedCost := (100000.0 / 1000000) * 3.00 * 0.1  // 90% discount
	inputCost := (10000.0 / 1000000) * 3.00
	outputCost := (5000.0 / 1000000) * 15.00
	expectedCost := cachedCost + inputCost + outputCost

	if cost != expectedCost {
		t.Errorf("Expected cost $%.6f, got $%.6f", expectedCost, cost)
	}

	stats := tracker.GetDailyStats()
	if stats.CachedTokens != 100000 {
		t.Errorf("Expected 100000 cached tokens, got %d", stats.CachedTokens)
	}

	// Verify cache savings: cached tokens at 10% vs 100% = 9x savings
	savings := cachedCost * 9
	if savings < 0.25 || savings > 0.35 {
		t.Errorf("Expected cache savings around $0.30, got $%.6f", savings)
	}
}

func TestRecordRequest_ExceedsDailyLimit(t *testing.T) {
	// Set a low limit: $0.10
	tracker := NewCostTracker(0.10, 0.08, 100000, testLogger())

	// First request: $0.105 (10k input + 5k output with Sonnet)
	// This should FAIL because it exceeds the $0.10 limit
	_, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err == nil {
		t.Fatal("Expected error for exceeding daily limit, got nil")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}

	// Stats should show no spend (request was rejected)
	stats := tracker.GetDailyStats()
	if stats.SpendUSD != 0.0 {
		t.Errorf("Expected spend 0.0 after rejected request, got $%.6f", stats.SpendUSD)
	}

	if stats.RequestCount != 0 {
		t.Errorf("Expected 0 requests after rejection, got %d", stats.RequestCount)
	}
}

func TestRecordRequest_MultipleRequestsApproachLimit(t *testing.T) {
	// Daily limit: $0.20, alert at $0.16
	tracker := NewCostTracker(0.20, 0.16, 100000, testLogger())

	// First request: ~$0.105 (Sonnet: 10k input + 5k output)
	cost1, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	stats := tracker.GetDailyStats()
	if stats.SpendUSD != cost1 {
		t.Errorf("After first request: expected spend $%.6f, got $%.6f", cost1, stats.SpendUSD)
	}

	// Second request: ~$0.105 again
	// Total would be ~$0.21, which exceeds limit of $0.20
	_, err = tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err == nil {
		t.Fatal("Expected error for second request exceeding limit, got nil")
	}

	// Spend should still be just the first request
	stats = tracker.GetDailyStats()
	if stats.SpendUSD != cost1 {
		t.Errorf("After rejected second request: expected spend $%.6f, got $%.6f", cost1, stats.SpendUSD)
	}

	if stats.RequestCount != 1 {
		t.Errorf("Expected 1 successful request, got %d", stats.RequestCount)
	}
}

func TestRecordRequest_PerQueryTokenLimit(t *testing.T) {
	// Token limit: 10,000 tokens
	tracker := NewCostTracker(100.0, 80.0, 10000, testLogger())

	// Try to record 15,000 tokens (exceeds limit)
	_, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err == nil {
		t.Fatal("Expected error for exceeding per-query token limit, got nil")
	}

	// Verify no tokens were recorded
	stats := tracker.GetDailyStats()
	if stats.InputTokens != 0 {
		t.Errorf("Expected 0 input tokens after rejection, got %d", stats.InputTokens)
	}

	// Try with tokens within limit (9,000 total)
	_, err = tracker.RecordRequest("claude-sonnet-4-5-20250929", 6000, 3000, 0)
	if err != nil {
		t.Fatalf("Request within token limit failed: %v", err)
	}

	stats = tracker.GetDailyStats()
	if stats.InputTokens != 6000 {
		t.Errorf("Expected 6000 input tokens, got %d", stats.InputTokens)
	}
}

func TestRecordRequest_UnknownModel(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	_, err := tracker.RecordRequest("unknown-model-xyz", 1000, 500, 0)
	if err == nil {
		t.Fatal("Expected error for unknown model, got nil")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}

	// Verify no cost was recorded
	stats := tracker.GetDailyStats()
	if stats.SpendUSD != 0.0 {
		t.Errorf("Expected spend 0.0 for unknown model, got $%.6f", stats.SpendUSD)
	}
}

func TestGetTotalStats_Aggregation(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	// Record multiple requests
	_, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("Request 1 failed: %v", err)
	}

	_, err = tracker.RecordRequest("claude-haiku-3.5", 20000, 10000, 0)
	if err != nil {
		t.Fatalf("Request 2 failed: %v", err)
	}

	_, err = tracker.RecordRequest("claude-opus-4.5", 5000, 2500, 1000)
	if err != nil {
		t.Fatalf("Request 3 failed: %v", err)
	}

	totalStats := tracker.GetTotalStats()

	// Verify aggregated totals
	expectedInputTokens := int64(10000 + 20000 + 5000)
	if totalStats.TotalInputTokens != expectedInputTokens {
		t.Errorf("Expected %d total input tokens, got %d", expectedInputTokens, totalStats.TotalInputTokens)
	}

	expectedOutputTokens := int64(5000 + 10000 + 2500)
	if totalStats.TotalOutputTokens != expectedOutputTokens {
		t.Errorf("Expected %d total output tokens, got %d", expectedOutputTokens, totalStats.TotalOutputTokens)
	}

	expectedCachedTokens := int64(1000)
	if totalStats.TotalCachedTokens != expectedCachedTokens {
		t.Errorf("Expected %d total cached tokens, got %d", expectedCachedTokens, totalStats.TotalCachedTokens)
	}

	if totalStats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", totalStats.TotalRequests)
	}

	if totalStats.TotalSpendUSD <= 0 {
		t.Errorf("Expected positive total spend, got $%.6f", totalStats.TotalSpendUSD)
	}
}

func TestAnthropicPricing_AllModels(t *testing.T) {
	// Verify all expected models are in the pricing table
	expectedModels := []string{
		"claude-opus-4.5",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4.5",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-3.5",
		"claude-3-5-haiku-20241022",
	}

	for _, model := range expectedModels {
		pricing, ok := AnthropicPricing[model]
		if !ok {
			t.Errorf("Model %s not found in AnthropicPricing", model)
			continue
		}

		if pricing.InputPricePerMToken <= 0 {
			t.Errorf("Model %s has invalid input price: %f", model, pricing.InputPricePerMToken)
		}

		if pricing.OutputPricePerMToken <= 0 {
			t.Errorf("Model %s has invalid output price: %f", model, pricing.OutputPricePerMToken)
		}

		// Output should be more expensive than input
		if pricing.OutputPricePerMToken <= pricing.InputPricePerMToken {
			t.Errorf("Model %s: output price (%f) should be higher than input price (%f)",
				model, pricing.OutputPricePerMToken, pricing.InputPricePerMToken)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker := NewCostTracker(1000.0, 800.0, 100000, testLogger())

	// Run 100 concurrent requests
	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use smaller token counts to stay under limits
			_, _ = tracker.RecordRequest("claude-haiku-3.5", 1000, 500, 0)
		}()
	}

	wg.Wait()

	// Verify all requests were recorded (or rejected consistently)
	totalStats := tracker.GetTotalStats()
	if totalStats.TotalRequests > concurrency {
		t.Errorf("Expected at most %d requests, got %d", concurrency, totalStats.TotalRequests)
	}

	// Verify no data races occurred (test would fail with -race flag if there were)
	dailyStats := tracker.GetDailyStats()
	if dailyStats.RequestCount != totalStats.TotalRequests {
		t.Errorf("Daily request count (%d) doesn't match total (%d)",
			dailyStats.RequestCount, totalStats.TotalRequests)
	}
}

func TestDailyReset(t *testing.T) {
	tracker := NewCostTracker(100.0, 80.0, 100000, testLogger())

	// Record a request
	cost1, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 10000, 5000, 0)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	dailyStats := tracker.GetDailyStats()
	if dailyStats.SpendUSD != cost1 {
		t.Errorf("Expected daily spend $%.6f, got $%.6f", cost1, dailyStats.SpendUSD)
	}

	// Manually trigger reset by changing the lastResetDate
	tracker.mu.Lock()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tracker.lastResetDate = yesterday
	tracker.mu.Unlock()

	// Get stats again - should trigger reset
	dailyStats = tracker.GetDailyStats()

	// Daily should be reset to 0
	if dailyStats.SpendUSD != 0.0 {
		t.Errorf("Expected daily spend $0.0 after reset, got $%.6f", dailyStats.SpendUSD)
	}

	if dailyStats.InputTokens != 0 {
		t.Errorf("Expected 0 input tokens after reset, got %d", dailyStats.InputTokens)
	}

	if dailyStats.RequestCount != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", dailyStats.RequestCount)
	}

	// Total should still have the original request
	totalStats := tracker.GetTotalStats()
	if totalStats.TotalSpendUSD != cost1 {
		t.Errorf("Expected total spend $%.6f, got $%.6f", cost1, totalStats.TotalSpendUSD)
	}

	if totalStats.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", totalStats.TotalRequests)
	}
}

func TestDailyStatsRemainingUSD(t *testing.T) {
	tracker := NewCostTracker(10.0, 8.0, 100000, testLogger())

	stats := tracker.GetDailyStats()
	if stats.RemainingUSD != 10.0 {
		t.Errorf("Expected remaining $10.0, got $%.2f", stats.RemainingUSD)
	}

	// Spend some money with Sonnet (50k input tokens = $0.15)
	cost, err := tracker.RecordRequest("claude-sonnet-4-5-20250929", 50000, 0, 0)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	stats = tracker.GetDailyStats()
	expectedRemaining := 10.0 - cost
	tolerance := 0.01 // $0.01 tolerance for floating point
	if stats.RemainingUSD < expectedRemaining-tolerance || stats.RemainingUSD > expectedRemaining+tolerance {
		t.Errorf("Expected remaining $%.2f, got $%.2f", expectedRemaining, stats.RemainingUSD)
	}
}
