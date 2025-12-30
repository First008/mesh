package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/First008/mesh/internal/agent"
	"github.com/First008/mesh/internal/gateway"
	mcpserver "github.com/First008/mesh/internal/mcp"
	"github.com/First008/mesh/internal/server"
	"github.com/rs/zerolog"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Setup logging
	logger := setupLogger()

	// Check mode (HTTP, MCP stdio, or Gateway)
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "http" // Default to HTTP
	}

	logger.Info().
		Str("config", *configPath).
		Str("mode", mode).
		Msg("Starting MESH")

	// Start in appropriate mode
	switch mode {
	case "gateway":
		// Gateway mode - single container managing multiple repos
		startGateway(*configPath, logger)

	case "mcp":
		// MCP stdio mode for Claude Code integration (single repo)
		startSingleRepoMCP(*configPath, logger)

	case "http":
		// HTTP API mode (single repo - backward compatible)
		startSingleRepoHTTP(*configPath, logger)

	default:
		logger.Fatal().Str("mode", mode).Msg("Unknown mode. Use 'gateway', 'http', or 'mcp'")
	}
}

// startGateway starts the gateway in multi-repo mode
func startGateway(configPath string, logger zerolog.Logger) {
	// Load gateway configuration
	config, err := gateway.LoadConfig(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load gateway configuration")
	}

	logger.Info().
		Int("repo_count", len(config.Repos)).
		Int("port", config.Port).
		Msg("Gateway configuration loaded")

	// Create gateway
	gw, err := gateway.New(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create gateway")
	}

	logger.Info().
		Int("repos", len(gw.ListRepos())).
		Msg("Gateway initialized successfully")

	// Start periodic branch scanner (every 10 seconds)
	ctx := context.Background()
	gw.StartScanner(ctx, 10*time.Second)

	// Start HTTP server for gateway
	srv := server.NewGateway(gw, config.Port, logger)
	if err := srv.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Gateway server failed")
	}
}

// startSingleRepoHTTP starts a single-repo agent in HTTP mode (backward compatible)
func startSingleRepoHTTP(configPath string, logger zerolog.Logger) {
	// Load single-repo configuration
	config, err := agent.LoadConfig(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	logger.Info().
		Str("repo", config.RepoName).
		Str("path", config.RepoPath).
		Int("port", config.Port).
		Msg("Configuration loaded")

	// Create agent
	agt, err := agent.New(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create agent")
	}

	logger.Info().
		Str("repo", agt.GetRepoName()).
		Str("model", agt.GetModel()).
		Msg("Agent created successfully")

	// Start HTTP server
	srv := server.New(agt, config.Port, logger)
	if err := srv.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}

// startSingleRepoMCP starts a single-repo agent in MCP stdio mode
func startSingleRepoMCP(configPath string, logger zerolog.Logger) {
	// Load single-repo configuration
	config, err := agent.LoadConfig(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	logger.Info().
		Str("repo", config.RepoName).
		Msg("Starting MCP server for single repository")

	// Create agent
	agt, err := agent.New(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create agent")
	}

	// Start MCP server
	mcpServer, err := mcpserver.New(agt, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create MCP server")
	}

	ctx := context.Background()
	if err := mcpServer.ServeStdio(ctx); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}

// setupLogger configures zerolog
func setupLogger() zerolog.Logger {
	// Pretty console output
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Logger()

	// Set log level from environment
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	return logger
}
