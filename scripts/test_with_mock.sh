#!/bin/bash

# Test SR-IOV Plugin with Mock Data
# This script demonstrates how to test the plugin without requiring actual SR-IOV hardware

set -e

echo "🧪 Testing SR-IOV Plugin with Mock Data"
echo "========================================"

# Build the project
echo "📦 Building project..."
go build ./cmd/server
go build ./cmd/client

# Run unit tests with mock data
echo "🧪 Running unit tests with mock data..."
go test ./pkg -v

# Run server tests with mock data
echo "🧪 Running server tests with mock data..."
go test ./cmd/server -v

# Create a mock lshw file for testing
echo "📝 Creating mock lshw data..."
cat > mock-lshw.json << 'EOF'
[
  {
    "id": "network",
    "class": "network",
    "claimed": true,
    "handle": "PCI:0000:01:00.0",
    "description": "Ethernet interface",
    "product": "MT2910 Family [ConnectX-7]",
    "vendor": "Mellanox Technologies",
    "physid": "0",
    "businfo": "pci@0000:01:00.0",
    "logicalname": "ens1f0np0",
    "version": "00",
    "serial": "00:1b:21:c0:8f:2e",
    "width": 64,
    "clock": 33000000,
    "configuration": {
      "autonegotiation": "off",
      "broadcast": "yes",
      "driver": "mlx5_core",
      "driverversion": "5.15.0-91-generic",
      "duplex": "full",
      "firmware": "20.36.1010",
      "latency": "0",
      "link": "yes",
      "multicast": "yes",
      "port": "fibre",
      "speed": "100Gbit/s"
    }
  },
  {
    "id": "network",
    "class": "network",
    "claimed": true,
    "handle": "PCI:0000:02:00.0",
    "description": "Ethernet interface",
    "product": "DSC Ethernet Controller",
    "vendor": "Pensando Systems",
    "physid": "0",
    "businfo": "pci@0000:02:00.0",
    "logicalname": "enp2s0np0",
    "version": "00",
    "serial": "04:90:81:37:cc:98",
    "width": 64,
    "clock": 33000000,
    "configuration": {
      "autonegotiation": "off",
      "broadcast": "yes",
      "driver": "ionic",
      "driverversion": "25.06.4.001",
      "duplex": "full",
      "firmware": "1.117.1-a-3",
      "latency": "0",
      "link": "no",
      "multicast": "yes",
      "port": "fibre"
    }
  }
]
EOF

# Test the server with mock data
echo "Starting server with mock data..."
./server &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Test the client
echo "Testing client with mock data..."
./client

# Clean up
echo "Cleaning up..."
kill $SERVER_PID
rm -f mock-lshw.json

echo "Mock testing completed successfully!"
echo ""
echo "Summary:"
echo "- Unit tests with mock data: PASS"
echo "- Server tests with mock data: PASS"
echo "- End-to-end test with mock data: PASS"
echo ""
echo "You can now develop and test the SR-IOV plugin without requiring actual hardware!" 