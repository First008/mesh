package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

// HTTPAgent represents the connection to the HTTP agent (single repo or gateway)
type HTTPAgent struct {
	baseURL   string
	repoName  string
	isGateway bool
	logger    zerolog.Logger
}

// AskToolArgs defines the arguments for the ask tool (single repo)
type AskToolArgs struct {
	Question string `json:"question" jsonschema:"description:Question about the codebase"`
}

// AskRepoToolArgs defines the arguments for asking a specific repo in gateway mode
type AskRepoToolArgs struct {
	Repository string `json:"repository" jsonschema:"description:Repository name to query"`
	Question   string `json:"question" jsonschema:"description:Question about the codebase"`
}

// RepoInfo represents repository information from the gateway
type RepoInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// AskRequest matches the HTTP API request format
type AskRequest struct {
	Question string `json:"question"`
}

// AskResponse matches the HTTP API response format
type AskResponse struct {
	Answer       string  `json:"answer"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CachedTokens int     `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

func main() {
	// Parse flags
	agentURL := flag.String("agent-url", "http://localhost:9000", "URL of the HTTP agent or gateway")
	repoName := flag.String("repo", "", "Repository name (for single-repo mode)")
	gatewayMode := flag.Bool("gateway", false, "Gateway mode - register tools for all repos")
	flag.Parse()

	// Setup logger
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Logger()

	isGateway := *gatewayMode || *repoName == ""

	logger.Info().
		Str("agent_url", *agentURL).
		Str("repo", *repoName).
		Bool("gateway_mode", isGateway).
		Msg("Starting MCP-to-HTTP bridge")

	// Create HTTP agent client
	agent := &HTTPAgent{
		baseURL:   *agentURL,
		repoName:  *repoName,
		isGateway: isGateway,
		logger:    logger,
	}

	// Create MCP server
	var impl *mcp.Implementation
	if isGateway {
		impl = &mcp.Implementation{
			Name:    "mesh-gateway-bridge",
			Version: "1.0.0",
		}
	} else {
		impl = &mcp.Implementation{
			Name:    fmt.Sprintf("mesh-%s-bridge", *repoName),
			Version: "1.0.0",
		}
	}

	mcpServer := mcp.NewServer(impl, nil)

	if isGateway {
		// Gateway mode - fetch repos and register tool for each
		if err := agent.registerGatewayTools(mcpServer); err != nil {
			logger.Fatal().Err(err).Msg("Failed to register gateway tools")
		}
	} else {
		// Single-repo mode (backward compatible)
		toolName := fmt.Sprintf("ask_%s", *repoName)
		mcp.AddTool(
			mcpServer,
			&mcp.Tool{
				Name:        toolName,
				Description: fmt.Sprintf("Ask questions about the %s repository. The agent has deep knowledge of the codebase and uses semantic search to find relevant code.", *repoName),
			},
			agent.handleAsk,
		)

		logger.Info().
			Str("tool", toolName).
			Msg("MCP bridge initialized (single-repo mode)")
	}

	// Start MCP server on stdio
	transport := &mcp.StdioTransport{}
	ctx := context.Background()

	if err := mcpServer.Run(ctx, transport); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}

// registerGatewayTools fetches repos from gateway and registers a tool for each
func (h *HTTPAgent) registerGatewayTools(mcpServer *mcp.Server) error {
	// Fetch repositories from gateway
	resp, err := http.Get(h.baseURL + "/repos")
	if err != nil {
		return fmt.Errorf("failed to fetch repos from gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway error (status %d): %s", resp.StatusCode, string(body))
	}

	var reposResp struct {
		Repos []RepoInfo `json:"repos"`
		Count int        `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&reposResp); err != nil {
		return fmt.Errorf("failed to decode repos response: %w", err)
	}

	h.logger.Info().
		Int("repo_count", reposResp.Count).
		Msg("Fetched repositories from gateway")

	// Register a tool for each repository
	for _, repo := range reposResp.Repos {
		repoName := repo.Name
		repoBranch := repo.Branch

		toolName := fmt.Sprintf("ask_%s", repoName)
		toolDesc := fmt.Sprintf("Ask questions about the %s repository (branch: %s). The agent has deep knowledge of the codebase and uses semantic search to find relevant code.", repoName, repoBranch)

		// Create a closure that captures repoName
		handler := func(ctx context.Context, request *mcp.CallToolRequest, args AskToolArgs) (*mcp.CallToolResult, any, error) {
			return h.handleAskRepo(ctx, request, repoName, args)
		}

		mcp.AddTool(
			mcpServer,
			&mcp.Tool{
				Name:        toolName,
				Description: toolDesc,
			},
			handler,
		)

		h.logger.Info().
			Str("tool", toolName).
			Str("repo", repoName).
			Str("branch", repoBranch).
			Msg("Registered MCP tool for repository")
	}

	return nil
}

// handleAsk forwards the question to the HTTP agent (single-repo mode)
func (h *HTTPAgent) handleAsk(ctx context.Context, request *mcp.CallToolRequest, args AskToolArgs) (*mcp.CallToolResult, any, error) {
	h.logger.Info().
		Str("question", args.Question).
		Msg("MCP tool invoked, forwarding to HTTP agent")

	// Build request
	reqBody := AskRequest{
		Question: args.Question,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call HTTP agent
	resp, err := http.Post(
		h.baseURL+"/ask",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call HTTP agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("HTTP agent error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var askResp AskResponse
	if err := json.NewDecoder(resp.Body).Decode(&askResp); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	h.logger.Info().
		Int("input_tokens", askResp.InputTokens).
		Int("output_tokens", askResp.OutputTokens).
		Int("cached_tokens", askResp.CachedTokens).
		Msg("HTTP agent responded")

	// Return MCP response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: askResp.Answer},
		},
	}, nil, nil
}

// handleAskRepo forwards the question to a specific repo in gateway mode
func (h *HTTPAgent) handleAskRepo(ctx context.Context, request *mcp.CallToolRequest, repoName string, args AskToolArgs) (*mcp.CallToolResult, any, error) {
	h.logger.Info().
		Str("repo", repoName).
		Str("question", args.Question).
		Msg("MCP tool invoked for repository, forwarding to gateway")

	// Build request
	reqBody := AskRequest{
		Question: args.Question,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call gateway for specific repo
	url := fmt.Sprintf("%s/ask/%s", h.baseURL, repoName)
	resp, err := http.Post(
		url,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("gateway error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response (gateway returns slightly different format)
	var gatewayResp struct {
		Repo     string `json:"repo"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Usage    struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CachedTokens int `json:"cached_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gatewayResp); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	h.logger.Info().
		Str("repo", repoName).
		Int("input_tokens", gatewayResp.Usage.InputTokens).
		Int("output_tokens", gatewayResp.Usage.OutputTokens).
		Int("cached_tokens", gatewayResp.Usage.CachedTokens).
		Msg("Gateway responded")

	// Return MCP response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: gatewayResp.Answer},
		},
	}, nil, nil
}
