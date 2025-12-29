package testing

import (
	"io"

	"github.com/First008/mesh/internal/agent"
	"github.com/First008/mesh/internal/gateway"
	"github.com/First008/mesh/pkg/telemetry"
	"github.com/rs/zerolog"
)

// Sample test code files
const (
	// SampleGoFile is a minimal Go source file
	SampleGoFile = `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`

	// SampleGoFileWithComments includes documentation
	SampleGoFileWithComments = `package calculator

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference between two integers
func Subtract(a, b int) int {
	return a - b
}
`

	// SampleTypeScriptFile is a TypeScript source file
	SampleTypeScriptFile = `export interface User {
	id: number;
	name: string;
	email: string;
}

export function formatUser(user: User): string {
	return ` + "`${user.name} <${user.email}>`" + `;
}
`

	// SampleREADME is a sample repository README
	SampleREADME = `# Test Repository

This is a test repository for unit testing.

## Features

- Feature 1
- Feature 2
- Feature 3

## Usage

` + "```go" + `
import "github.com/example/test"

func main() {
	test.Run()
}
` + "```" + `
`

	// SampleCLAUDEMD is a sample Claude instructions file
	SampleCLAUDEMD = `# Development Guidelines

## Code Style

- Use Go fmt for formatting
- Follow Effective Go principles
- Write tests for new features

## Architecture

This codebase follows clean architecture principles with:
- cmd/ for entry points
- internal/ for private code
- pkg/ for public libraries
`

	// SampleLargeFile is a larger file for testing chunking
	SampleLargeFile = `package service

import (
	"context"
	"database/sql"
	"errors"
)

// UserService handles user operations
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id int) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = $1", id).
		Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	var id int
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		name, email).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Name: name, Email: email}, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE users SET name = $1, email = $2 WHERE id = $3",
		user.Name, user.Email, user.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

// User represents a user entity
type User struct {
	ID    int
	Name  string
	Email string
}
`
)

// NewTestAgentConfig creates an agent.Config suitable for testing
func NewTestAgentConfig() *agent.Config {
	return &agent.Config{
		RepoPath:          "/tmp/test-repo",
		RepoName:          "test-repo",
		AnthropicKey:      "test-anthropic-key-12345",
		EmbeddingProvider: "ollama",
		OllamaURL:         "http://localhost:11434",
		OllamaModel:       "nomic-embed-text",
		QdrantURL:         "http://localhost:6333",
		CostLimits: agent.CostLimits{
			DailyMaxUSD:       10.0,
			AlertThresholdUSD: 8.0,
			PerQueryMaxTokens: 100000,
		},
	}
}

// NewTestGatewayConfig creates a gateway.Config suitable for testing
func NewTestGatewayConfig() *gateway.Config {
	return &gateway.Config{
		Port:              8080,
		QdrantURL:         "http://localhost:6333",
		EmbeddingProvider: "ollama",
		EmbeddingModel:    "nomic-embed-text",
		OllamaURL:         "http://localhost:11434",
		LLMProvider:       "anthropic",
		LLMModel:          "claude-sonnet-4-5-20250929",
		AnthropicKey:      "test-anthropic-key-12345",
		Repos: []gateway.RepoConfig{
			{
				Name: "test-repo-1",
				Path: "/tmp/test-repo-1",
			},
			{
				Name: "test-repo-2",
				Path: "/tmp/test-repo-2",
			},
		},
	}
}

// NewTestLogger creates a zerolog.Logger that discards output (for quiet tests)
func NewTestLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

// NewTestCostTracker creates a telemetry.CostTracker for testing
func NewTestCostTracker() *telemetry.CostTracker {
	logger := zerolog.New(io.Discard)
	return telemetry.NewCostTracker(10.0, 8.0, 100000, logger)
}

// TestPricingTable creates a custom pricing table for testing
func TestPricingTable() map[string]telemetry.PricingTable {
	return map[string]telemetry.PricingTable{
		"test-model-v1": {
			InputPricePerMToken:  1.0,
			OutputPricePerMToken: 5.0,
		},
		"expensive-model-v1": {
			InputPricePerMToken:  10.0,
			OutputPricePerMToken: 50.0,
		},
	}
}

// FileTree represents a test file structure
type FileTree struct {
	Files map[string]string // path -> content
}

// NewTestFileTree creates a sample file tree for testing
func NewTestFileTree() *FileTree {
	return &FileTree{
		Files: map[string]string{
			"README.md":              SampleREADME,
			"CLAUDE.md":              SampleCLAUDEMD,
			"main.go":                SampleGoFile,
			"pkg/calculator/calc.go": SampleGoFileWithComments,
			"pkg/service/user.go":    SampleLargeFile,
			"src/index.ts":           SampleTypeScriptFile,
		},
	}
}

// GetFilesByExtension returns files matching the given extension
func (ft *FileTree) GetFilesByExtension(ext string) map[string]string {
	result := make(map[string]string)
	for path, content := range ft.Files {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			result[path] = content
		}
	}
	return result
}
