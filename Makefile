.PHONY: build test lint clean install help

# Build variables
BINARY_NAME := infracast
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI binary
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)
	@echo "Built: bin/$(BINARY_NAME)"

test: ## Run all tests
	@echo "Running tests..."
	@go test -v -race ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@golangci-lint run ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/

install: build ## Install the binary to GOPATH/bin
	@echo "Installing to $(GOPATH)/bin..."
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/

deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	@go mod tidy

fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
