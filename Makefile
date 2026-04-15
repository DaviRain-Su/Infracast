.PHONY: build test lint clean install help release

# Build variables
BINARY_NAME := infracast
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Release targets
RELEASE_DIR := dist
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64

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

regression: ## Run local regression suite (fmt/vet/test/build/smoke)
	@echo "Running regression suite..."
	@./scripts/regression.sh

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

release-build: ## Build release binaries and package as tar.gz for all platforms
	@echo "Building release binaries ($(VERSION))..."
	@rm -rf $(RELEASE_DIR)
	@mkdir -p $(RELEASE_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		ARCHIVE_NAME=$(BINARY_NAME)_$(VERSION)_$${GOOS}_$${GOARCH}; \
		TMPDIR=$(RELEASE_DIR)/$$ARCHIVE_NAME; \
		echo "  Building $$GOOS/$$GOARCH..."; \
		mkdir -p $$TMPDIR; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$TMPDIR/$(BINARY_NAME) ./cmd/$(BINARY_NAME); \
		tar -czf $(RELEASE_DIR)/$$ARCHIVE_NAME.tar.gz -C $(RELEASE_DIR) $$ARCHIVE_NAME; \
		rm -rf $$TMPDIR; \
	done
	@echo ""
	@echo "Release archives built in $(RELEASE_DIR)/:"
	@ls -lh $(RELEASE_DIR)/*.tar.gz

release-checksums: release-build ## Generate SHA-256 checksums for release archives
	@echo "Generating checksums..."
	@cd $(RELEASE_DIR) && shasum -a 256 *.tar.gz > checksums.txt
	@echo "Checksums saved to $(RELEASE_DIR)/checksums.txt"
	@cat $(RELEASE_DIR)/checksums.txt

release: release-checksums ## Build release archives with checksums (full pipeline)
	@echo ""
	@echo "Release complete: $(VERSION)"
	@echo "  Archives: $(RELEASE_DIR)/*.tar.gz"
	@echo "  Checksums: $(RELEASE_DIR)/checksums.txt"
