# Multi-stage build for SR-IOV Plugin
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    protobuf \
    protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf files
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/sriov.proto

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sriovd cmd/sriovd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sriovctl cmd/sriovctl/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S sriov && \
    adduser -u 1001 -S sriov -G sriov

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/sriovd /app/sriovctl /app/

# Create config directory
RUN mkdir -p /app/config && chown -R sriov:sriov /app

# Switch to non-root user
USER sriov

# Expose gRPC port
EXPOSE 50051

# Default command
CMD ["./sriovd", "-config", "/app/config/config.yaml"] 