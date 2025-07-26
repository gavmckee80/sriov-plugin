#!/bin/bash
# Build script for SR-IOV Plugin
# Usage: ./scripts/build.sh [server|client|all]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go first."
    exit 1
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Function to build server
build_server() {
    print_status "Building server..."
    go build -o bin/server ./cmd/server
    if [ $? -eq 0 ]; then
        print_status "Server built successfully: bin/server"
    else
        print_error "Failed to build server"
        exit 1
    fi
}

# Function to build client
build_client() {
    print_status "Building client..."
    go build -o bin/client ./cmd/client
    if [ $? -eq 0 ]; then
        print_status "Client built successfully: bin/client"
    else
        print_error "Failed to build client"
        exit 1
    fi
}

# Function to build both
build_all() {
    print_status "Building server and client..."
    build_server
    build_client
    print_status "All binaries built successfully!"
}

# Main script logic
case "${1:-all}" in
    "server")
        build_server
        ;;
    "client")
        build_client
        ;;
    "all"|"")
        build_all
        ;;
    *)
        print_error "Invalid argument: $1"
        echo "Usage: $0 [server|client|all]"
        echo "  server - Build server only"
        echo "  client - Build client only"
        echo "  all    - Build both (default)"
        exit 1
        ;;
esac

print_status "Build completed successfully!" 