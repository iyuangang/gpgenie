# Makefile for GPGenie

# Go parameters
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_TEST=$(GO_CMD) test
GO_CLEAN=$(GO_CMD) clean
GO_FMT=gofumpt
GO_VET=$(GO_CMD) vet
GO_MOD=$(GO_CMD) mod
GO_LINT=golangci-lint run

# Binary name
BINARY_NAME=gpgenie

# Directories
CMD_DIR=cmd/gpgenie
BUILD_DIR=build

# Default target
all: build

# Build the project for the current OS and architecture
build:
	@echo "Building for current OS and architecture..."
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go

# Cross-compile for different OS and architectures
release:
	@echo "Cross-compiling for all supported OS and architectures..."
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)/main.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)/main.go
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)/main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)/main.go
	GOOS=windows GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_DIR)/main.go

# Run tests
test:
	@echo "Running tests..."
	$(GO_TEST) ./...

# Format the code
fmt:
	@echo "Formatting code..."
	$(GO_FMT) -l -w .

# Run go vet
vet:
	@echo "Running go vet..."
	$(GO_VET) ./...

# Run linter
lint:
	@echo "Running linter..."
	$(GO_LINT)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GO_CLEAN)
	rm -rf $(BUILD_DIR)

# Tidy up go.mod and go.sum
tidy:
	@echo "Tidying up go.mod and go.sum..."
	$(GO_MOD) tidy

# Run all checks
check: fmt vet lint test

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO_MOD) download

# Run the application
run: build
	@echo "Running the application..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Help
help:
	@echo "Makefile commands:"
	@echo "  make build       - Build the project for the current OS and architecture"
	@echo "  make release     - Cross-compile for all supported OS and architectures"
	@echo "  make test        - Run tests"
	@echo "  make fmt         - Format the code"
	@echo "  make vet         - Run go vet"
	@echo "  make lint        - Run linter"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make tidy        - Tidy up go.mod and go.sum"
	@echo "  make check       - Run all checks (fmt, vet, lint, test)"
	@echo "  make deps        - Install dependencies"
	@echo "  make run         - Run the application"
	@echo "  make help        - Show this help message"

.PHONY: all build build-all test fmt vet lint clean tidy check deps run help
