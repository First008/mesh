package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealth returns the health status of the gateway
func (s *GatewayServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"mode":   "gateway",
	})
}

// handleGatewayInfo returns information about the gateway
func (s *GatewayServer) handleGatewayInfo(c *gin.Context) {
	repos := s.gateway.ListRepos()

	c.JSON(http.StatusOK, gin.H{
		"mode":       "gateway",
		"repo_count": len(repos),
		"repos":      repos,
	})
}

// handleListRepos returns the list of all configured repositories
func (s *GatewayServer) handleListRepos(c *gin.Context) {
	repos := s.gateway.ListRepos()

	// Get detailed info for each repo
	repoInfos := make([]interface{}, 0, len(repos))
	for _, repoName := range repos {
		info, err := s.gateway.GetRepo(repoName)
		if err != nil {
			s.logger.Warn().Err(err).Str("repo", repoName).Msg("Failed to get repo info")
			continue
		}
		repoInfos = append(repoInfos, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"repos": repoInfos,
		"count": len(repoInfos),
	})
}

// handleGetRepo returns information about a specific repository
func (s *GatewayServer) handleGetRepo(c *gin.Context) {
	repoName := c.Param("repo")

	info, err := s.gateway.GetRepo(repoName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleAskRepo handles questions to a specific repository
func (s *GatewayServer) handleAskRepo(c *gin.Context) {
	repoName := c.Param("repo")

	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	s.logger.Info().
		Str("repo", repoName).
		Str("question", req.Question).
		Msg("Processing question for repository")

	// Ask the gateway
	response, err := s.gateway.Ask(c.Request.Context(), repoName, req.Question)
	if err != nil {
		s.logger.Error().Err(err).Str("repo", repoName).Msg("Failed to process question")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"repo":     repoName,
		"question": req.Question,
		"answer":   response.Content,
		"usage": gin.H{
			"input_tokens":  response.InputTokens,
			"output_tokens": response.OutputTokens,
			"cached_tokens": response.CachedTokens,
		},
		"model": response.Model,
	})
}

// handleAskAll handles questions to all repositories
func (s *GatewayServer) handleAskAll(c *gin.Context) {
	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	s.logger.Info().
		Str("question", req.Question).
		Msg("Processing question for all repositories")

	// Ask all repositories
	responses, err := s.gateway.AskAll(c.Request.Context(), req.Question)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Some repositories failed to respond")
		// Continue even if some repos failed
	}

	// Format responses
	results := make(map[string]interface{})
	totalInputTokens := 0
	totalOutputTokens := 0
	totalCachedTokens := 0

	for repoName, response := range responses {
		results[repoName] = gin.H{
			"answer": response.Content,
			"usage": gin.H{
				"input_tokens":  response.InputTokens,
				"output_tokens": response.OutputTokens,
				"cached_tokens": response.CachedTokens,
			},
			"model": response.Model,
		}
		totalInputTokens += response.InputTokens
		totalOutputTokens += response.OutputTokens
		totalCachedTokens += response.CachedTokens
	}

	c.JSON(http.StatusOK, gin.H{
		"question":  req.Question,
		"responses": results,
		"total_usage": gin.H{
			"input_tokens":  totalInputTokens,
			"output_tokens": totalOutputTokens,
			"cached_tokens": totalCachedTokens,
		},
	})
}

// handleReindexRepo triggers re-indexing for a specific repository
func (s *GatewayServer) handleReindexRepo(c *gin.Context) {
	repoName := c.Param("repo")

	s.logger.Info().
		Str("repo", repoName).
		Msg("Triggering re-indexing")

	if err := s.gateway.ReindexRepo(c.Request.Context(), repoName); err != nil {
		s.logger.Error().Err(err).Str("repo", repoName).Msg("Failed to re-index repository")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"repo":   repoName,
		"message": "Repository re-indexed successfully",
	})
}
