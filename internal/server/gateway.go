package server

import (
	"fmt"

	"github.com/First008/mesh/internal/gateway"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// GatewayServer is the HTTP server for the gateway (multi-repo)
type GatewayServer struct {
	gateway *gateway.Gateway
	port    int
	logger  zerolog.Logger
	engine  *gin.Engine
}

// NewGateway creates a new HTTP server for the gateway
func NewGateway(gw *gateway.Gateway, port int, logger zerolog.Logger) *GatewayServer {
	// Set Gin mode based on log level
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	// Add logging middleware
	engine.Use(ginLogger(logger))

	// Add recovery middleware
	engine.Use(gin.Recovery())

	server := &GatewayServer{
		gateway: gw,
		port:    port,
		logger:  logger,
		engine:  engine,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all HTTP routes for the gateway
func (s *GatewayServer) setupRoutes() {
	// Health check
	s.engine.GET("/health", s.handleHealth)

	// Gateway info
	s.engine.GET("/info", s.handleGatewayInfo)

	// List all repositories
	s.engine.GET("/repos", s.handleListRepos)

	// Get specific repository info
	s.engine.GET("/repos/:repo", s.handleGetRepo)

	// Ask a specific repository
	s.engine.POST("/ask/:repo", s.handleAskRepo)

	// Ask all repositories
	s.engine.POST("/ask-all", s.handleAskAll)

	// Trigger re-indexing for a specific repository
	s.engine.POST("/repos/:repo/reindex", s.handleReindexRepo)

	// GitHub webhook for automatic re-indexing
	s.engine.POST("/webhooks/github", s.handleGitHubWebhook)
}

// Start starts the HTTP server
func (s *GatewayServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info().
		Str("addr", addr).
		Int("repos", len(s.gateway.ListRepos())).
		Msg("Starting Gateway HTTP server")

	return s.engine.Run(addr)
}
