.PHONY: build test install clean lint fmt help

# Build variables
BINARY_NAME=proto-nil-linter
BUILD_DIR=bin
CMD_DIR=cmd/proto-nil-linter

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the linter binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: ## Install the linter binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install ./$(CMD_DIR)
	@echo "Installed to: $$(go env GOPATH)/bin/$(BINARY_NAME)"

test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter on the codebase
	@echo "Running linters..."
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download

verify: fmt vet test ## Run all verification steps

run-example: build ## Run linter on example code
	@echo "Running linter on example code..."
	./$(BUILD_DIR)/$(BINARY_NAME) ./testdata/src/example/...

# Development helpers
watch-test: ## Watch and run tests on file changes (requires entr)
	find . -name "*.go" | entr -c make test

quick: ## Quick build and test
	@echo "Quick check..."
	@go build ./$(CMD_DIR) && go test ./...
	@echo "✓ Quick check passed"

all: clean deps fmt vet test build ## Run all steps

.DEFAULT_GOAL := help