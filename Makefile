.PHONY: all build clean proto test help

# Default target
all: build

# Build both daemon and CLI
build: proto
	@echo "Building SR-IOV management tools..."
	@mkdir -p bin
	go build -o bin/sriovd cmd/sriovd/main.go
	go build -o bin/sriovctl cmd/sriovctl/main.go
	@echo "Build complete!"

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/sriov.proto

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f proto/*.pb.go

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build  - Build both daemon and CLI (default)"
	@echo "  proto  - Generate protobuf files"
	@echo "  clean  - Remove build artifacts"
	@echo "  test   - Run tests"
	@echo "  help   - Show this help message" 