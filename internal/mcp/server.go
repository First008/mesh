package mcpserver

import (
	"context"
	"fmt"

	"github.com/First008/mesh/internal/agent"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

// Server wraps the MCP server for the MESH agent
type Server struct {
	mcpServer *mcp.Server
	agent     *agent.Agent
	logger    zerolog.Logger
}

// AskToolArgs defines the arguments for the ask tool
type AskToolArgs struct {
	Question string `json:"question" jsonschema:"description:Question about the codebase"`
}

// New creates a new MCP server
func New(agt *agent.Agent, logger zerolog.Logger) (*Server, error) {
	s := &Server{
		agent:  agt,
		logger: logger,
	}

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    fmt.Sprintf("mesh-%s", agt.GetRepoName()),
		Version: "1.0.0",
	}

	mcpServer := mcp.NewServer(impl, nil)

	// Add tool for asking questions
	toolName := fmt.Sprintf("ask_%s", agt.GetRepoName())
	mcp.AddTool(
		mcpServer,
		&mcp.Tool{
			Name:        toolName,
			Description: fmt.Sprintf("Ask questions about the %s repository codebase. Use this to understand code structure, find functions, explain implementations, etc.", agt.GetRepoName()),
		},
		s.handleAskTool,
	)

	s.mcpServer = mcpServer

	logger.Info().
		Str("tool", toolName).
		Str("repo", agt.GetRepoName()).
		Msg("MCP server initialized")

	return s, nil
}

// ServeStdio starts the MCP server in stdio mode
func (s *Server) ServeStdio(ctx context.Context) error {
	s.logger.Info().Msg("Starting MCP server in stdio mode")

	transport := &mcp.StdioTransport{}

	return s.mcpServer.Run(ctx, transport)
}

// handleAskTool handles the ask tool invocation
func (s *Server) handleAskTool(ctx context.Context, request *mcp.CallToolRequest, args AskToolArgs) (*mcp.CallToolResult, any, error) {
	s.logger.Info().
		Str("question", args.Question).
		Msg("MCP tool invoked")

	// Ask the agent
	response, err := s.agent.Ask(ctx, args.Question)
	if err != nil {
		return nil, nil, fmt.Errorf("agent error: %w", err)
	}

	// Log metrics
	s.logger.Info().
		Int("input_tokens", response.InputTokens).
		Int("output_tokens", response.OutputTokens).
		Int("cached_tokens", response.CachedTokens).
		Msg("MCP tool completed")

	// Return response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: response.Content},
		},
	}, nil, nil
}
