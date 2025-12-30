package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GitHubPushEvent represents a GitHub push webhook payload (simplified)
type GitHubPushEvent struct {
	Ref        string `json:"ref"`    // refs/heads/main
	Before     string `json:"before"` // Previous commit SHA
	After      string `json:"after"`  // New commit SHA
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"` // owner/repo
	} `json:"repository"`
}

// handleGitHubWebhook handles GitHub push webhooks for automatic re-indexing
func (s *GatewayServer) handleGitHubWebhook(c *gin.Context) {
	var event GitHubPushEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid webhook payload",
		})
		return
	}

	// Extract branch from ref (refs/heads/main -> main)
	branch := ""
	if len(event.Ref) > 11 && event.Ref[:11] == "refs/heads/" {
		branch = event.Ref[11:]
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "not a branch push event",
		})
		return
	}

	s.logger.Info().
		Str("repo", event.Repository.Name).
		Str("branch", branch).
		Str("before", event.Before[:8]).
		Str("after", event.After[:8]).
		Msg("Received GitHub webhook")

	// Find matching repo in gateway by repository name
	// GitHub sends "repo-name", we need to match it to our configured repos
	repoName := event.Repository.Name

	// Trigger re-indexing for this branch
	if err := s.gateway.ReindexBranch(c.Request.Context(), repoName, branch); err != nil {
		s.logger.Error().
			Err(err).
			Str("repo", repoName).
			Str("branch", branch).
			Msg("Failed to trigger webhook re-index")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to trigger re-index",
		})
		return
	}

	s.logger.Info().
		Str("repo", repoName).
		Str("branch", branch).
		Msg("Webhook re-index triggered successfully")

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"repo":    repoName,
		"branch":  branch,
		"message": "Re-index triggered",
	})
}
