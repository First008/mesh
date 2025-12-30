package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AskRequest is the request body for the /ask endpoint
type AskRequest struct {
	Question string `json:"question" binding:"required"`
}

// AskResponse is the response body for the /ask endpoint
type AskResponse struct {
	Answer       string  `json:"answer"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CachedTokens int     `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// ErrorResponse is the response body for errors
type ErrorResponse struct {
	Error string `json:"error"`
}

// handleAsk handles POST /ask requests
func (s *Server) handleAsk(c *gin.Context) {
	var req AskRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}

	// Ask the agent
	response, err := s.agent.Ask(c.Request.Context(), req.Question)
	if err != nil {
		s.logger.Error().Err(err).Msg("Agent.Ask failed")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to process question: " + err.Error(),
		})
		return
	}

	// Return response
	c.JSON(http.StatusOK, AskResponse{
		Answer:       response.Content,
		Model:        response.Model,
		InputTokens:  response.InputTokens,
		OutputTokens: response.OutputTokens,
		CachedTokens: response.CachedTokens,
	})
}

// HealthResponse is the response body for /health
type HealthResponse struct {
	Status string `json:"status"`
	Repo   string `json:"repo"`
	Model  string `json:"model"`
}

// handleHealth handles GET /health requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "healthy",
		Repo:   s.agent.GetRepoName(),
		Model:  s.agent.GetModel(),
	})
}

// InfoResponse is the response body for /info
type InfoResponse struct {
	RepoName string `json:"repo_name"`
	Model    string `json:"model"`
}

// handleInfo handles GET /info requests
func (s *Server) handleInfo(c *gin.Context) {
	c.JSON(http.StatusOK, InfoResponse{
		RepoName: s.agent.GetRepoName(),
		Model:    s.agent.GetModel(),
	})
}

// MetricsResponse is the response body for /metrics
type MetricsResponse struct {
	Daily DailyMetrics `json:"daily"`
	Total TotalMetrics `json:"total"`
}

// DailyMetrics holds daily statistics
type DailyMetrics struct {
	SpendUSD     float64 `json:"spend_usd"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	RequestCount int     `json:"request_count"`
	LimitUSD     float64 `json:"limit_usd"`
	RemainingUSD float64 `json:"remaining_usd"`
}

// TotalMetrics holds total statistics
type TotalMetrics struct {
	TotalSpendUSD     float64 `json:"total_spend_usd"`
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalCachedTokens int64   `json:"total_cached_tokens"`
	TotalRequests     int     `json:"total_requests"`
}

// handleMetrics handles GET /metrics requests
func (s *Server) handleMetrics(c *gin.Context) {
	dailyStats := s.agent.GetDailyStats()
	totalStats := s.agent.GetTotalStats()

	c.JSON(http.StatusOK, MetricsResponse{
		Daily: DailyMetrics{
			SpendUSD:     dailyStats.SpendUSD,
			InputTokens:  dailyStats.InputTokens,
			OutputTokens: dailyStats.OutputTokens,
			CachedTokens: dailyStats.CachedTokens,
			RequestCount: dailyStats.RequestCount,
			LimitUSD:     dailyStats.LimitUSD,
			RemainingUSD: dailyStats.RemainingUSD,
		},
		Total: TotalMetrics{
			TotalSpendUSD:     totalStats.TotalSpendUSD,
			TotalInputTokens:  totalStats.TotalInputTokens,
			TotalOutputTokens: totalStats.TotalOutputTokens,
			TotalCachedTokens: totalStats.TotalCachedTokens,
			TotalRequests:     totalStats.TotalRequests,
		},
	})
}
