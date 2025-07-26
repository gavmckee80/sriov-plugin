# SR-IOV Plugin Makefile
# Builds binaries to bin/ directory

.PHONY: all clean build test server client sriov help

# Default target
all: build

# Build both server and client (legacy) plus new Cobra CLI
build: server client sriov

# Build server binary (legacy)
server:
	@echo "Building server..."
	@mkdir -p bin
	go build -o bin/server ./cmd/server

# Build client binary (legacy)
client:
	@echo "Building client..."
	@mkdir -p bin
	go build -o bin/client ./cmd/client

# Build new Cobra-based CLI
sriov:
	@echo "Building SR-IOV CLI..."
	@mkdir -p bin
	go build -o bin/sriov ./cmd/sriov

# Build both with race detection
build-race: server-race client-race sriov-race

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

# Build Cobra CLI with race detection
sriov-race:
	@echo "Building SR-IOV CLI with race detection..."
	@mkdir -p bin
	go build -race -o bin/sriov ./cmd/sriov

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
	GOOS=linux GOARCH=amd64 go build -o bin/sriov-linux-amd64 ./cmd/sriov

# Build for macOS
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -o bin/client-darwin-amd64 ./cmd/client
	GOOS=darwin GOARCH=amd64 go build -o bin/sriov-darwin-amd64 ./cmd/sriov

# Build for Windows
build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/server-windows-amd64.exe ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o bin/client-windows-amd64.exe ./cmd/client
	GOOS=windows GOARCH=amd64 go build -o bin/sriov-windows-amd64.exe ./cmd/sriov

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build server, client (legacy) and new SR-IOV CLI"
	@echo "  server       - Build server binary only (legacy)"
	@echo "  client       - Build client binary only (legacy)"
	@echo "  sriov        - Build new Cobra-based SR-IOV CLI"
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
	@echo ""
	@echo "Usage examples:"
	@echo "  ./bin/sriov list                    # List devices"
	@echo "  ./bin/sriov server                  # Start server"
	@echo "  ./bin/sriov monitor                 # Monitor devices"
	@echo "  ./bin/sriov --help                  # Show help" 