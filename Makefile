.PHONY: build build-all build-mcp-bridge build-indexer run run-gateway run-mcp test test-verbose test-coverage \
        clean lint fmt docker-build docker-up docker-down docker-logs index verify deps help

# Variables
GO := go
GOFLAGS := -v
BINARY_NAME := mesh-agent
MCP_BRIDGE_NAME := mesh-mcp-bridge
INDEXER_NAME := mesh-indexer

# Build targets
build: ## Build the main agent binary
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/agent

build-mcp-bridge: ## Build the MCP bridge binary
	@echo "Building $(MCP_BRIDGE_NAME)..."
	@$(GO) build $(GOFLAGS) -o $(MCP_BRIDGE_NAME) ./cmd/mcp-bridge

build-indexer: ## Build the indexer binary
	@echo "Building $(INDEXER_NAME)..."
	@$(GO) build $(GOFLAGS) -o $(INDEXER_NAME) ./cmd/indexer

build-all: build build-mcp-bridge build-indexer ## Build all binaries

# Run targets
run: build ## Run the agent locally (HTTP mode)
	@echo "Running $(BINARY_NAME) in HTTP mode..."
	@./$(BINARY_NAME) -config configs/agent.yaml

run-gateway: build ## Run the gateway locally (Gateway mode)
	@echo "Running $(BINARY_NAME) in Gateway mode..."
	@MODE=gateway ./$(BINARY_NAME) -config configs/repos.yaml

run-mcp: build ## Run in MCP mode
	@echo "Running $(BINARY_NAME) in MCP mode..."
	@MODE=mcp ./$(BINARY_NAME) -config configs/agent.yaml

# Test targets
test: ## Run all tests
	@echo "Running tests..."
	@$(GO) test ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	@$(GO) test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	@$(GO) test -race ./...

# Code quality targets
lint: ## Run golangci-lint
	@echo "Running linters..."
	@golangci-lint run ./...

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -s -w .
	@$(GO) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@$(GO) vet ./...

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@$(GO) mod download

deps-tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	@$(GO) mod tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	@$(GO) mod verify

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t mesh-gateway:latest -f deployments/docker/Dockerfile.agent .

docker-up: ## Start services with Docker Compose
	@echo "Starting with Docker Compose..."
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs) && cd deployments/docker && docker compose up --build -d; else cd deployments/docker && docker compose up --build -d; fi
	@echo "Services started. Use 'make docker-logs' to view logs."

docker-down: ## Stop Docker Compose services
	@echo "Stopping Docker Compose..."
	@cd deployments/docker && docker compose down

docker-logs: ## Follow Docker Compose logs
	@cd deployments/docker && docker compose logs -f

docker-restart: docker-down docker-up ## Restart Docker Compose services

docker-clean: docker-down ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	@docker system prune -f
	@docker volume prune -f

# Indexing targets
index: build-indexer ## Index a repository (REPO=name PATH=path QDRANT=url)
	@echo "Indexing repository..."
	@./scripts/index-repo.sh $(REPO) $(PATH) $(QDRANT)

reindex: ## Trigger re-indexing via API (REPO=name)
	@echo "Triggering re-index for repository: $(REPO)"
	@curl -X POST http://localhost:9000/repos/$(REPO)/reindex

# Utility targets
clean: ## Clean build artifacts and temporary files
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME) $(MCP_BRIDGE_NAME) $(INDEXER_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf bin/
	@$(GO) clean -cache -testcache

verify: test lint vet ## Run all verification checks (test, lint, vet)

install: build-all ## Install binaries to $GOPATH/bin
	@echo "Installing binaries..."
	@$(GO) install ./cmd/agent
	@$(GO) install ./cmd/mcp-bridge
	@$(GO) install ./cmd/indexer

# Health check targets
health: ## Check service health
	@echo "Checking service health..."
	@curl -s http://localhost:9000/health | jq

repos: ## List indexed repositories
	@echo "Listing indexed repositories..."
	@curl -s http://localhost:9000/repos | jq

metrics: ## Show usage metrics
	@echo "Fetching metrics..."
	@curl -s http://localhost:9000/metrics | jq

# Development helpers
dev-setup: deps ## Setup development environment
	@echo "Setting up development environment..."
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development environment ready!"

watch: ## Watch for changes and rebuild (requires entr)
	@echo "Watching for changes..."
	@find . -name '*.go' | entr -r make build

# Help target
help: ## Show this help message
	@echo "MESH - Intelligent Code Understanding Gateway"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@grep -E '^build.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Run Targets:"
	@grep -E '^run.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Test Targets:"
	@grep -E '^test.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Docker Targets:"
	@grep -E '^docker.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Other Targets:"
	@grep -E '^(clean|lint|fmt|vet|deps|install|health|repos|metrics|verify|dev-setup|index|reindex|watch|help):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make build              # Build main binary"
	@echo "  make run-gateway        # Run in gateway mode"
	@echo "  make test-coverage      # Run tests with coverage"
	@echo "  make docker-up          # Start with Docker Compose"
	@echo "  make health             # Check service health"
	@echo ""
	@echo "For detailed documentation, see:"
	@echo "  README.md         - User guide and setup"
	@echo "  ARCHITECTURE.md   - Technical architecture"
	@echo "  CONTRIBUTING.md   - Contribution guidelines"
