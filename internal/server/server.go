// Package server provides HTTP server for the agent API.
//
// The server package implements REST API endpoints using Gin framework,
// supporting both single-repository and gateway modes, with health checks,
// metrics, and GitHub webhook integration.
package server

import (
	"fmt"

	"github.com/First008/mesh/internal/agent"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Server is the HTTP server for the agent
type Server struct {
	agent  *agent.Agent
	port   int
	logger zerolog.Logger
	engine *gin.Engine
}

// New creates a new HTTP server
func New(agent *agent.Agent, port int, logger zerolog.Logger) *Server {
	// Set Gin mode based on log level
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	// Add logging middleware
	engine.Use(ginLogger(logger))

	// Add recovery middleware
	engine.Use(gin.Recovery())

	server := &Server{
		agent:  agent,
		port:   port,
		logger: logger,
		engine: engine,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check
	s.engine.GET("/health", s.handleHealth)

	// Agent info
	s.engine.GET("/info", s.handleInfo)

	// Metrics
	s.engine.GET("/metrics", s.handleMetrics)

	// Ask question
	s.engine.POST("/ask", s.handleAsk)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info().
		Str("addr", addr).
		Str("repo", s.agent.GetRepoName()).
		Msg("Starting HTTP server")

	return s.engine.Run(addr)
}

// ginLogger creates a Gin middleware that logs using zerolog
func ginLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log after processing
		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Str("client_ip", c.ClientIP()).
			Msg("HTTP request")
	}
}
