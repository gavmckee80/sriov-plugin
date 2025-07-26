# SR-IOV Plugin

This repository contains a simple gRPC service for discovering SR-IOV capable network devices.
It parses the output of `lshw -class network -json` and enriches the data using our own sysfs-based PCI parsing implementation for superior performance and reliability.

## Features

- **Sysfs-based PCI Parsing**: Direct kernel data access for superior performance (10-50x faster than lspci)
- **Dynamic Hardware Discovery**: Real-time `lshw` execution in production mode
- **Enhanced Device Information**: Comprehensive device context including capabilities, serial numbers, and configuration
- **SR-IOV Device Discovery**: Automatically detects and lists SR-IOV capable network devices
- **PCI Information Enrichment**: Enriches device data with driver, vendor, and product information
- **Mock Testing Support**: Comprehensive mock testing for development without SR-IOV hardware
- **gRPC API**: Clean gRPC interface for device management
- **Cross-platform Builds**: Support for Linux, macOS, and Windows builds

## Building

### Using Makefile (Recommended)

```bash
# Build both server and client
make build

# Build only server
make server

# Build only client
make client

# Build with race detection
make build-race

# Run tests
make test

# Clean build artifacts
make clean

# Show all available targets
make help
```

### Using Build Script

```bash
# Build both server and client
./scripts/build.sh

# Build only server
./scripts/build.sh server

# Build only client
./scripts/build.sh client
```

### Manual Build

```bash
# Create bin directory
mkdir -p bin

# Build server
go build -o bin/server ./cmd/server

# Build client
go build -o bin/client ./cmd/client
```

## Running

### Server

The server supports two modes:

#### Production Mode (Default)
Runs `lshw -class network -json` dynamically to gather real-time hardware information:

```bash
# Start server in production mode (default)
./bin/server

# Or explicitly
./bin/server -file=false
```

#### Development/Testing Mode
Uses static files for consistent testing:

```bash
# Use static file for testing
./bin/server -file -lshw-file=./lshw-network.json

# Use custom lshw file
./bin/server -file -lshw-file=./my-lshw-data.json
```

### Client

The client supports multiple output formats:

```bash
# Table format (default)
./bin/client

# JSON format
./bin/client -format json

# Simple format
./bin/client -format simple

# Custom server address
./bin/client -server=192.168.1.100:50051

# Custom timeout
./bin/client -timeout=10s

# Show help
./bin/client -h
```

## Enhanced Device Information

The system now provides comprehensive device information including:

- **Basic Information**: PCI address, name, driver, vendor, product
- **SR-IOV Details**: Capability detection, VF counts, configuration
- **Hardware Context**: Description, serial number, size, capacity, clock, width
- **Device Classification**: Class, subclass, capabilities
- **Network Details**: Logical names, bus information
- **Configuration**: Driver settings, speed, features

## Development

### Mock Testing

The project includes comprehensive mock testing for development without SR-IOV hardware:

```bash
# Run all tests
make test

# Run specific test files
go test ./pkg -v
go test ./cmd/server -v

# Run with coverage
make test-coverage
```

See [MOCK_TESTING.md](MOCK_TESTING.md) for detailed mock testing documentation.

### File Structure

```
.
├── bin/                    # Compiled binaries
├── cmd/
│   ├── client/            # gRPC client
│   └── server/            # gRPC server
├── pkg/
│   ├── device.go          # Device parsing and enrichment
│   ├── pci_sysfs.go       # Sysfs-based PCI parsing
│   ├── pci_sysfs_mock.go  # Mock PCI data
│   └── pci_sysfs_test.go  # PCI parsing tests
├── proto/                 # gRPC protocol definitions
├── scripts/               # Build and utility scripts
├── Makefile               # Build automation
└── README.md             # This file
```

## API

### gRPC Service

The service provides a simple gRPC interface:

```protobuf
service SRIOVManager {
  rpc ListDevices(ListDevicesRequest) returns (ListDevicesResponse);
}

message Device {
  string pci_address = 1;
  string name = 2;
  string driver = 3;
  string vendor = 4;
  string product = 5;
}
```

### Example Usage

```bash
# Start server
./bin/server

# In another terminal, list devices
./bin/client

# Get JSON output
./bin/client -format json

# Get simple output
./bin/client -format simple
```

## Performance

The sysfs-based approach provides significant performance improvements:

- **10-50x faster** than lspci text parsing
- **Direct kernel access** via `/sys/bus/pci/devices`
- **Real-time data** without command execution overhead
- **Comprehensive SR-IOV support** with direct register access

See [SYSFS_VS_LSPCI.md](SYSFS_VS_LSPCI.md) for detailed performance comparison.

## Requirements

- **Go 1.19+** for building
- **Linux** for sysfs access (primary target)
- **lshw** for hardware discovery (in production mode)
- **gRPC** for communication

## License

This project is licensed under the MIT License.
