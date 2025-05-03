# Makefile for Go Project

# ==============================================================================
# Variables
# ==============================================================================

# Binary name (default: name of the current directory)
BINARY_NAME = mbx

# Output directory for the binary
BUILD_OUTPUT_DIR = ./build/dist

# Go command
GO_CMD ?= go

# GolangCI-Lint command
# Assumes golangci-lint is in the PATH. If not, provide the full path.
GOLANGCI_LINT_CMD ?= golangci-lint

# Go build flags (e.g., -ldflags="-s -w" to strip symbols and debug info)
# Example: GO_BUILD_FLAGS = -ldflags="-X main.Version=1.0.0"
GO_BUILD_FLAGS ?=

# Go test flags (e.g., -v for verbose, -race for race detector)
GO_TEST_FLAGS ?= -v

# Packages to include in build/test etc. (default: all packages in the current module)
# Note: golangci-lint typically uses ./... by default, but we keep the variable for consistency
PACKAGES ?= ./...

# ==============================================================================
# Targets
# ==============================================================================

.PHONY: all build run test cover clean deps fmt lint help

# Default target: builds the project
all: build

# Build the Go application
# Creates the binary in the specified BUILD_OUTPUT_DIR
build: deps
	@echo "==> Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_OUTPUT_DIR)
	$(GO_CMD) build $(GO_BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BINARY_NAME) .

build-multi: deps
	@echo "==> Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_OUTPUT_DIR)
	GOOS=darwin GOARCH=arm64 $(GO_CMD) build $(GO_BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BINARY_NAME)_darwin_arm64 .
	GOOS=darwin GOARCH=amd64 $(GO_CMD) build $(GO_BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BINARY_NAME)_darwin_amd64 .
	GOOS=linux GOARCH=amd64 $(GO_CMD) build $(GO_BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BINARY_NAME)_linux_amd64 .
	GOOS=linux GOARCH=arm64 $(GO_CMD) build $(GO_BUILD_FLAGS) -o $(BUILD_OUTPUT_DIR)/$(BINARY_NAME)_linux_arm64 .

# Run the Go application
# Assumes the main package is in the current directory or specified by PACKAGES if it's a single package
run: build
	@echo "==> Running $(BINARY_NAME)..."
	$(BUILD_OUTPUT_DIR)/$(BINARY_NAME)

# Run tests
test: deps
	@echo "==> Running tests..."
	$(GO_CMD) test $(GO_TEST_FLAGS) $(PACKAGES)

# Run tests with coverage report
cover: deps
	@echo "==> Running tests with coverage..."
	$(GO_CMD) test $(GO_TEST_FLAGS) -coverprofile=coverage.out $(PACKAGES)
	@echo "==> Generating coverage report (coverage.html)..."
	$(GO_CMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts and coverage files
clean:
	@echo "==> Cleaning..."
	@rm -rf $(BUILD_OUTPUT_DIR)
	@rm -f coverage.out coverage.html

# Tidy dependencies (download new, remove unused)
deps:
	@echo "==> Tidying dependencies..."
	$(GO_CMD) mod tidy
	$(GO_CMD) mod download # Optional: ensures all deps are downloaded

# Format Go code
fmt:
	@echo "==> Formatting code..."
	$(GO_CMD) fmt $(PACKAGES)

# Run golangci-lint linter
# Assumes golangci-lint is installed (go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
# It will use the configuration file (.golangci.yml, .golangci.toml, or .golangci.json) if present.
lint: deps
	@echo "==> Running golangci-lint..."
	$(GOLANGCI_LINT_CMD) run $(PACKAGES)

# Display help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all        Build the application (default)"
	@echo "  build      Compile the application"
	@echo "  run        Compile and run the application"
	@echo "  test       Run tests"
	@echo "  cover      Run tests and generate HTML coverage report"
	@echo "  clean      Remove build artifacts and coverage files"
	@echo "  deps       Tidy and download dependencies"
	@echo "  fmt        Format Go source code"
	@echo "  lint       Run golangci-lint linter"
	@echo "  help       Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  GOLANGCI_LINT_CMD  Command to run golangci-lint (default: golangci-lint)"
	@echo "  GO_BUILD_FLAGS     Flags for 'go build' (e.g., -ldflags='...') "
	@echo "  GO_TEST_FLAGS      Flags for 'go test' (e.g., -v -race)"
	@echo "  PACKAGES           Packages to target (default: ./...)"
