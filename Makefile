.PHONY: all build clean proto test test-unit test-integration test-coverage help install lint format docker-build docker-run

# Build variables
BINARY_DIR = bin
DAEMON_BINARY = sriovd
CLIENT_BINARY = sriovctl
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

# Default target
all: build

# Build both daemon and CLI
build: proto
	@echo "Building SR-IOV management tools (version: $(VERSION))..."
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(BINARY_DIR)/$(DAEMON_BINARY) cmd/sriovd/main.go
	go build $(LDFLAGS) -o $(BINARY_DIR)/$(CLIENT_BINARY) cmd/sriovctl/main.go
	@echo "Build complete!"

# Build with race detection
build-race: proto
	@echo "Building with race detection..."
	@mkdir -p $(BINARY_DIR)
	go build -race $(LDFLAGS) -o $(BINARY_DIR)/$(DAEMON_BINARY) cmd/sriovd/main.go
	go build -race $(LDFLAGS) -o $(BINARY_DIR)/$(CLIENT_BINARY) cmd/sriovctl/main.go
	@echo "Race build complete!"

# Build for different platforms
build-linux: proto
	@echo "Building for Linux..."
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(DAEMON_BINARY)-linux-amd64 cmd/sriovd/main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(CLIENT_BINARY)-linux-amd64 cmd/sriovctl/main.go

build-darwin: proto
	@echo "Building for macOS..."
	@mkdir -p $(BINARY_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(DAEMON_BINARY)-darwin-amd64 cmd/sriovd/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(CLIENT_BINARY)-darwin-amd64 cmd/sriovctl/main.go

build-windows: proto
	@echo "Building for Windows..."
	@mkdir -p $(BINARY_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(DAEMON_BINARY)-windows-amd64.exe cmd/sriovd/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(CLIENT_BINARY)-windows-amd64.exe cmd/sriovctl/main.go

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@which protoc > /dev/null || (echo "protoc not found. Please install Protocol Buffers compiler." && exit 1)
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/sriov.proto
	@echo "Protobuf generation complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BINARY_DIR)/
	rm -f proto/*.pb.go
	rm -f coverage.out
	@echo "Clean complete!"

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	go test -v ./internal/config/...
	go test -v ./pkg/sriov/...
	@echo "Unit tests complete!"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v ./cmd/sriovd/...
	@echo "Integration tests complete!"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show coverage summary
test-coverage-summary:
	@echo "Running tests with coverage summary..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Dependencies installed!"

# Lint code
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format code
format:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t sriov-plugin:$(VERSION) .
	docker tag sriov-plugin:$(VERSION) sriov-plugin:latest

# Docker run
docker-run:
	@echo "Running Docker container..."
	docker run -it --rm -v $(PWD)/config.yaml:/app/config.yaml sriov-plugin:latest

# Development server
dev-server:
	@echo "Starting development server..."
	@mkdir -p $(BINARY_DIR)
	go build -race -o $(BINARY_DIR)/$(DAEMON_BINARY) cmd/sriovd/main.go
	$(BINARY_DIR)/$(DAEMON_BINARY) -config config.yaml

# Development client
dev-client:
	@echo "Starting development client..."
	@mkdir -p $(BINARY_DIR)
	go build -race -o $(BINARY_DIR)/$(CLIENT_BINARY) cmd/sriovctl/main.go
	$(BINARY_DIR)/$(CLIENT_BINARY) list

# Show help
help:
	@echo "SR-IOV Plugin Build System"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@echo "  build                    - Build both daemon and CLI (default)"
	@echo "  build-race              - Build with race detection"
	@echo "  build-linux             - Build for Linux"
	@echo "  build-darwin            - Build for macOS"
	@echo "  build-windows           - Build for Windows"
	@echo "  proto                   - Generate protobuf files"
	@echo "  clean                   - Remove build artifacts"
	@echo "  test                    - Run all tests"
	@echo "  test-unit               - Run unit tests only"
	@echo "  test-integration        - Run integration tests only"
	@echo "  test-coverage           - Run tests with coverage report"
	@echo "  test-coverage-summary   - Run tests with coverage summary"
	@echo "  install                 - Install dependencies"
	@echo "  lint                    - Lint code with golangci-lint"
	@echo "  format                  - Format code"
	@echo "  docker-build            - Build Docker image"
	@echo "  docker-run              - Run Docker container"
	@echo "  dev-server              - Start development server"
	@echo "  dev-client              - Start development client"
	@echo "  help                    - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION                 - Version string (default: git describe)"
	@echo ""
	@echo "Examples:"
	@echo "  make build              # Build for current platform"
	@echo "  make test-coverage      # Run tests with coverage"
	@echo "  make dev-server         # Start development server"
	@echo "  make VERSION=v1.0.0 build  # Build with specific version" 