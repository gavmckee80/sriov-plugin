# SR-IOV Plugin Makefile
# Builds binaries to bin/ directory

.PHONY: all clean build test server client help

# Default target
all: build

# Build both server and client
build: server client

# Build server binary
server:
	@echo "Building server..."
	@mkdir -p bin
	go build -o bin/server ./cmd/server

# Build client binary
client:
	@echo "Building client..."
	@mkdir -p bin
	go build -o bin/client ./cmd/client

# Build both with race detection
build-race: server-race client-race

# Build server with race detection
server-race:
	@echo "Building server with race detection..."
	@mkdir -p bin
	go build -race -o bin/server ./cmd/server

# Build client with race detection
client-race:
	@echo "Building client with race detection..."
	@mkdir -p bin
	go build -race -o bin/client ./cmd/client

# Run tests
test:
	@echo "Running tests..."
	go test ./... -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -v -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race ./... -v

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f *.test
	rm -f coverage.out coverage.html
	rm -f *.exe *.exe~

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Build for different architectures
build-all: build-linux build-darwin build-windows

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/server-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build -o bin/client-linux-amd64 ./cmd/client

# Build for macOS
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -o bin/client-darwin-amd64 ./cmd/client

# Build for Windows
build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/server-windows-amd64.exe ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o bin/client-windows-amd64.exe ./cmd/client

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build server and client binaries"
	@echo "  server       - Build server binary only"
	@echo "  client       - Build client binary only"
	@echo "  test         - Run all tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  test-race    - Run tests with race detection"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  build-all    - Build for Linux, macOS, and Windows"
	@echo "  build-linux  - Build for Linux only"
	@echo "  build-darwin - Build for macOS only"
	@echo "  build-windows- Build for Windows only"
	@echo "  help         - Show this help message" 