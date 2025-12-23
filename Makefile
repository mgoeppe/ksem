.PHONY: help build run clean test docker-build docker-run docker-stop docker-clean proto deps install dev

# Variables
BINARY_NAME=ksem
DOCKER_IMAGE=ksem:latest
DOCKER_CONTAINER=ksem-monitor
GO=go
PROTOC=protoc

# Default target - show help
help: ## Show this help message
	@echo "KSEM Meter Scraper - Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

# Build targets
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build -o $(BINARY_NAME) .
	@echo "✓ Build complete: ./$(BINARY_NAME)"

build-static: ## Build static binary (CGO disabled)
	@echo "Building static $(BINARY_NAME)..."
	@CGO_ENABLED=0 $(GO) build -a -installsuffix cgo -ldflags="-w -s" -o $(BINARY_NAME) .
	@echo "✓ Static build complete: ./$(BINARY_NAME)"

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@$(GO) install
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Run targets
run: ## Run the application (TUI mode)
	@$(GO) run .

run-json: ## Run the application in JSON mode (stdout)
	@$(GO) run . -f json

run-sqlite: ## Run the application in SQLite mode
	@$(GO) run . -f sqlite -o ksem.db -i 10s

dev: ## Run in development mode with debug enabled
	@$(GO) run . --debug

# Docker targets
docker-build: ## Build the Docker image
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "✓ Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run the Docker container (requires config.yaml)
	@if [ ! -f config.yaml ]; then \
		echo "Error: config.yaml not found. Copy config.yaml.example and configure it first."; \
		exit 1; \
	fi
	@mkdir -p data
	@echo "Starting Docker container $(DOCKER_CONTAINER)..."
	@docker run -d \
		--name $(DOCKER_CONTAINER) \
		-v $(PWD)/config.yaml:/app/config/config.yaml:ro \
		-v $(PWD)/data:/app/data \
		--restart unless-stopped \
		$(DOCKER_IMAGE)
	@echo "✓ Container started: $(DOCKER_CONTAINER)"
	@echo "  View logs: make docker-logs"

docker-stop: ## Stop the Docker container
	@echo "Stopping $(DOCKER_CONTAINER)..."
	@docker stop $(DOCKER_CONTAINER) 2>/dev/null || true
	@docker rm $(DOCKER_CONTAINER) 2>/dev/null || true
	@echo "✓ Container stopped and removed"

docker-logs: ## Show Docker container logs
	@docker logs -f $(DOCKER_CONTAINER)

docker-clean: docker-stop ## Stop container and remove Docker image
	@echo "Removing Docker image $(DOCKER_IMAGE)..."
	@docker rmi $(DOCKER_IMAGE) 2>/dev/null || true
	@echo "✓ Docker cleanup complete"

# Development targets
proto: ## Regenerate Protocol Buffer code
	@echo "Generating Protocol Buffer code..."
	@$(GO) generate ./pkg/proto
	@echo "✓ Protocol Buffer code generated"

deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod verify
	@echo "✓ Dependencies ready"

tidy: ## Tidy and verify go.mod
	@echo "Tidying go.mod..."
	@$(GO) mod tidy
	@echo "✓ go.mod tidied"

# Testing targets
test: ## Run tests
	@echo "Running tests..."
	@$(GO) test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@$(GO) test -v -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Code quality targets
fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	@$(GO) vet ./...
	@echo "✓ Vet check passed"

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running golangci-lint..."
	@golangci-lint run || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"

# Cleanup targets
clean: ## Remove built binaries and artifacts
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -f ksem.db
	@rm -f coverage.out coverage.html
	@echo "✓ Cleanup complete"

clean-all: clean docker-clean ## Remove all built artifacts including Docker images
	@echo "✓ Full cleanup complete"

# Setup targets
setup-config: ## Create config.yaml from example
	@if [ -f config.yaml ]; then \
		echo "config.yaml already exists. Not overwriting."; \
	else \
		cp config.yaml.example config.yaml; \
		echo "✓ Created config.yaml from example. Please edit with your KSEM credentials."; \
	fi

setup: setup-config deps ## Initial setup (config + dependencies)
	@echo "✓ Setup complete. Edit config.yaml and run 'make run' to start."

# Quick targets
all: clean build ## Clean and build

docker: docker-build docker-run ## Build and run Docker container
