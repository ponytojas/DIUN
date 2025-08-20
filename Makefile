# Docker Notify Makefile
# Build automation for the Docker image update notification service

# Variables
BINARY_NAME := docker-notify
DOCKER_IMAGE := docker-notify
DOCKER_TAG := latest
VERSION := 1.0.0
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)

# Go related variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOVET := $(GOCMD) vet
GOFMT := gofmt

# Docker related variables
DOCKER := docker
DOCKER_COMPOSE := docker-compose

# Architecture detection
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	DOCKER_PLATFORM := linux/amd64
else ifeq ($(ARCH),aarch64)
	DOCKER_PLATFORM := linux/arm64
else ifeq ($(ARCH),arm64)
	DOCKER_PLATFORM := linux/arm64
else ifeq ($(ARCH),armv7l)
	DOCKER_PLATFORM := linux/arm/v7
else
	DOCKER_PLATFORM := linux/amd64
endif

# Directories
BUILD_DIR := build
CMD_DIR := cmd
INTERNAL_DIR := internal
CONFIG_DIR := configs

.PHONY: all build clean test test-verbose test-race test-cover vet fmt lint
.PHONY: docker-build docker-build-auto docker-run docker-push docker-clean
.PHONY: compose-up compose-down compose-logs compose-test compose-build
.PHONY: deps deps-update deps-verify
.PHONY: install uninstall
.PHONY: release help show-arch
.PHONY: run-local run-local-test run-local-once run-local-debug

# Default target
all: clean test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		./$(CMD_DIR)/main.go
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for current platform
build-local:
	@echo "Building $(BINARY_NAME) for local platform..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		./$(CMD_DIR)/main.go
	@echo "Local binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all:
	@echo "Building $(BINARY_NAME) for multiple platforms..."
	@mkdir -p $(BUILD_DIR)

	# Linux AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 \
		./$(CMD_DIR)/main.go

	# Linux ARM64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 \
		./$(CMD_DIR)/main.go

	# macOS AMD64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 \
		./$(CMD_DIR)/main.go

	# macOS ARM64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 \
		./$(CMD_DIR)/main.go

	# Windows AMD64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe \
		./$(CMD_DIR)/main.go

	@echo "Multi-platform binaries built in $(BUILD_DIR)/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "Clean completed"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	$(GOTEST) -v -count=1 ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -w .

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@unformatted=$$($(GOFMT) -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "All files are properly formatted"

# Install golangci-lint and run it
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2)
	golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GOMOD) tidy
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Verify dependencies
deps-verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify

# Show detected architecture
show-arch:
	@echo "Detected architecture: $(ARCH)"
	@echo "Docker platform: $(DOCKER_PLATFORM)"

# Docker build (legacy - uses amd64)
docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	$(DOCKER) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Docker build with auto-detected architecture
docker-build-auto:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG) for $(DOCKER_PLATFORM)..."
	$(DOCKER) build --platform $(DOCKER_PLATFORM) -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG) for $(DOCKER_PLATFORM)"

# Docker build with version tag
docker-build-version:
	@echo "Building Docker image $(DOCKER_IMAGE):$(VERSION)..."
	$(DOCKER) build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(VERSION)"

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	$(DOCKER) run --rm -it \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v $(PWD)/$(CONFIG_DIR)/config.yaml:/etc/docker-notify/config.yaml:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run Docker container in test mode
docker-test:
	@echo "Running Docker container in test mode..."
	$(DOCKER) run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG) -test

# Push Docker image
docker-push:
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	$(DOCKER) push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@if [ "$(DOCKER_TAG)" != "$(VERSION)" ]; then \
		echo "Pushing Docker image $(DOCKER_IMAGE):$(VERSION)..."; \
		$(DOCKER) push $(DOCKER_IMAGE):$(VERSION); \
	fi

# Clean Docker images and containers
docker-clean:
	@echo "Cleaning Docker artifacts..."
	@$(DOCKER) rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	@$(DOCKER) rmi $(DOCKER_IMAGE):$(VERSION) 2>/dev/null || true
	@$(DOCKER) system prune -f

# Docker Compose operations
compose-build:
	@echo "Building services with Docker Compose for $(DOCKER_PLATFORM)..."
	DOCKER_DEFAULT_PLATFORM=$(DOCKER_PLATFORM) $(DOCKER_COMPOSE) build

compose-up:
	@echo "Starting services with Docker Compose..."
	$(DOCKER_COMPOSE) up -d

# Stop Docker Compose services
compose-down:
	@echo "Stopping Docker Compose services..."
	$(DOCKER_COMPOSE) down

# Show Docker Compose logs
compose-logs:
	@echo "Showing Docker Compose logs..."
	$(DOCKER_COMPOSE) logs -f docker-notify

# Test with Docker Compose
compose-test:
	@echo "Testing with Docker Compose..."
	$(DOCKER_COMPOSE) run --rm docker-notify -test

# Rebuild and restart with Docker Compose
compose-restart: compose-down docker-build-auto compose-up

# Rebuild with auto-architecture and restart with Docker Compose
compose-restart-auto: compose-down compose-build compose-up

# Install binary to system
install: build-local
	@echo "Installing $(BINARY_NAME) to /usr/local/bin/..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation completed"

# Uninstall binary from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin/..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation completed"

# Create release archive
release: clean test build-all
	@echo "Creating release archive..."
	@mkdir -p $(BUILD_DIR)/release
	@cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd $(BUILD_DIR) && zip release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "Release archives created in $(BUILD_DIR)/release/"

# Development workflow
dev: clean fmt vet test build-local

# Development workflow with Docker
dev-docker: clean fmt vet test docker-build-auto

# CI workflow
ci: clean fmt-check vet test-race test-cover build

# Quick check before commit
check: fmt vet test

# Local development targets
run-local: build-local
	@echo "Running Docker Notify with local config..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_DIR)/config.local.yaml

# Test notifications with local config
run-local-test: build-local
	@echo "Testing Docker Notify with local config..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_DIR)/config.local.yaml -test

# Single check with local config
run-local-once: build-local
	@echo "Running single check with local config..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_DIR)/config.local.yaml -check-once

# Debug mode with local config
run-local-debug: build-local
	@echo "Running Docker Notify in debug mode with local config..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_DIR)/config.local.yaml -log-level debug -check-once

# Show help
help:
	@echo "Docker Notify Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build         - Build binary for Linux AMD64"
	@echo "  build-local   - Build binary for current platform"
	@echo "  build-all     - Build binaries for all platforms"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-verbose  - Run tests with verbose output"
	@echo "  test-race     - Run tests with race detection"
	@echo "  test-cover    - Run tests with coverage report"
	@echo "  vet           - Run go vet"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check code formatting"
	@echo "  lint          - Run linter"
	@echo "  deps          - Download dependencies"
	@echo "  deps-update   - Update dependencies"
	@echo "  deps-verify   - Verify dependencies"
	@echo "  show-arch     - Show detected architecture"
	@echo ""
	@echo "Local development targets:"
	@echo "  run-local     - Run with local config (daemon mode)"
	@echo "  run-local-test - Test notifications with local config"
	@echo "  run-local-once - Single check with local config"
	@echo "  run-local-debug - Debug mode with local config"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build     - Build Docker image (amd64)"
	@echo "  docker-build-auto - Build Docker image (auto-detect arch)"
	@echo "  docker-run       - Run Docker container"
	@echo "  docker-test      - Run Docker container in test mode"
	@echo "  docker-push      - Push Docker image"
	@echo "  docker-clean     - Clean Docker artifacts"
	@echo ""
	@echo "Docker Compose targets:"
	@echo "  compose-build    - Build services (auto-detect arch)"
	@echo "  compose-up       - Start services"
	@echo "  compose-down     - Stop services"
	@echo "  compose-logs     - Show logs"
	@echo "  compose-test     - Test with compose"
	@echo "  compose-restart-auto - Rebuild (auto-arch) and restart"
	@echo ""
	@echo "Other targets:"
	@echo "  install       - Install binary to system"
	@echo "  uninstall     - Uninstall binary from system"
	@echo "  release       - Create release archives"
	@echo "  dev           - Development workflow"
	@echo "  dev-docker    - Development workflow with Docker"
	@echo "  ci            - CI workflow"
	@echo "  check         - Quick pre-commit check"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  DOCKER_TAG=$(DOCKER_TAG)"
	@echo "  DOCKER_IMAGE=$(DOCKER_IMAGE)"
	@echo "  ARCH=$(ARCH)"
	@echo "  DOCKER_PLATFORM=$(DOCKER_PLATFORM)"
