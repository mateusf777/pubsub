.PHONY: help build test clean lint fmt vet staticcheck install-tools docker-build docker-test

# Default target
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build all binaries
	@echo "Building binaries..."
	@mkdir -p build
	@cd server && go build -o ../build/ps-server ./cmd/pubsub
	@cd example && go build -o ../build/queue ./queue
	@cd example && go build -o ../build/subscriber ./subscriber
	@cd example && go build -o ../build/publisher ./publisher
	@cd example && go build -o ../build/request ./request
	@cd example && go build -o ../build/subscriber_tls ./subscriber_tls
	@cd example && go build -o ../build/publisher_tls ./publisher_tls
	@echo "Build complete. Binaries in ./build/"

build-server: ## Build only the server
	@mkdir -p build
	@cd server && go build -o ../build/ps-server ./cmd/pubsub

# Test targets
test: ## Run all tests
	@echo "Running core tests..."
	@cd core && go test -race -cover
	@echo "Running server tests..."
	@cd server && go test -race -cover
	@echo "Running client tests..."
	@cd client && go test -race -cover

test-verbose: ## Run tests with verbose output
	@cd core && go test -v -race -cover
	@cd server && go test -v -race -cover
	@cd client && go test -v -race -cover

test-coverage: ## Run tests with coverage report
	@mkdir -p coverage
	@cd core && go test -race -coverprofile=../coverage/core.out -covermode=atomic
	@cd server && go test -race -coverprofile=../coverage/server.out -covermode=atomic
	@cd client && go test -race -coverprofile=../coverage/client.out -covermode=atomic
	@go tool cover -html=coverage/core.out -o coverage/core.html
	@go tool cover -html=coverage/server.out -o coverage/server.html
	@go tool cover -html=coverage/client.out -o coverage/client.html
	@echo "Coverage reports generated in ./coverage/"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	@cd core && go test -bench=. -benchmem
	@cd server && go test -bench=. -benchmem
	@cd client && go test -bench=. -benchmem

# Linting and formatting targets
fmt: ## Format Go code
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "Code formatted."

vet: ## Run go vet
	@echo "Running go vet..."
	@cd core && go vet ./...
	@cd server && go vet ./...
	@cd client && go vet ./...
	@cd example && go vet ./...

staticcheck: ## Run staticcheck (requires staticcheck to be installed)
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null || (echo "staticcheck not found. Run 'make install-tools' first." && exit 1)
	@cd core && staticcheck ./...
	@cd server && staticcheck ./...
	@cd client && staticcheck ./...
	@cd example && staticcheck ./...

golangci-lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-tools' first." && exit 1)
	@golangci-lint run ./...

lint: fmt vet ## Run all linting (fmt and vet)

# Security targets
govulncheck: ## Run govulncheck for vulnerability scanning
	@echo "Running vulnerability check..."
	@which govulncheck > /dev/null || (echo "govulncheck not found. Run 'make install-tools' first." && exit 1)
	@cd core && govulncheck ./...
	@cd server && govulncheck ./...
	@cd client && govulncheck ./...
	@cd example && govulncheck ./...

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t pubsub:latest -f server/Dockerfile .
	@echo "Docker image built: pubsub:latest"

docker-test: docker-build ## Run integration tests with Docker
	@echo "Running integration tests..."
	@cd example/integration && go test -race -run '^(TestPublish|TestQueue|TestRequest)$$' .

# Utility targets
clean: ## Clean build artifacts and test caches
	@echo "Cleaning..."
	@rm -rf build/
	@rm -rf coverage/
	@go clean -cache -testcache
	@echo "Clean complete."

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/vektra/mockery/v2@v2.52.1
	@echo "Tools installed."

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@cd core && go mod download
	@cd server && go mod download
	@cd client && go mod download
	@cd example && go mod download
	@echo "Dependencies downloaded."

tidy: ## Tidy go.mod files
	@echo "Tidying go.mod files..."
	@cd core && go mod tidy
	@cd server && go mod tidy
	@cd client && go mod tidy
	@cd example && go mod tidy
	@echo "go.mod files tidied."

# Run targets
run-server: build-server ## Run the server
	@echo "Starting server..."
	@./build/ps-server

run-subscriber: build ## Run a subscriber example
	@echo "Starting subscriber..."
	@./build/subscriber

run-publisher: build ## Run a publisher example
	@echo "Starting publisher..."
	@./build/publisher

# CI targets
ci: lint vet test ## Run CI checks (lint, vet, test)
	@echo "CI checks passed."

ci-full: install-tools lint vet staticcheck test test-coverage ## Run full CI checks with coverage
	@echo "Full CI checks passed."

# Pre-commit checks
pre-commit: fmt vet test ## Run pre-commit checks (format, vet, test)
	@echo "Pre-commit checks passed."
