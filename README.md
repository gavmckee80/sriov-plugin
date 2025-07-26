# sriov-plugin

This repository contains a simple gRPC service for discovering SR-IOV capable network devices.
It parses the output of `lshw -class network -json` and enriches the data using our own PCI parsing implementation.

## Features

- **SR-IOV Device Discovery**: Automatically detects and lists SR-IOV capable network devices
- **PCI Information Enrichment**: Enriches device data with driver, vendor, and product information
- **Mock Testing Support**: Comprehensive mock testing for development without SR-IOV hardware
- **gRPC API**: Clean gRPC interface for device management

## Building

```
go build ./cmd/server
go build ./cmd/client
```

## Running

Start the gRPC server:

```
./server
```

In another terminal, run the example client:

```
go run ./cmd/client
```

The client will print a list of detected devices from the sample `lshw-network.json` file.

## Mock Testing for Development

The plugin includes comprehensive mock testing capabilities that allow you to develop and test without requiring actual SR-IOV hardware.

### Quick Start with Mock Data

Run the mock testing script:

```bash
./scripts/test_with_mock.sh
```

### Manual Mock Testing

```bash
# Run all tests with mock data
go test ./pkg -v
go test ./cmd/server -v

# Run specific mock tests
go test ./pkg -run TestMockPciDevices
go test ./cmd/server -run TestServerWithMockData
```

### Development Example

See `examples/mock_development_example.go` for a complete example of using mock data for development.

For detailed documentation on mock testing, see [MOCK_TESTING.md](MOCK_TESTING.md).

## Testing

The project includes both real hardware tests and comprehensive mock tests:

- **Unit Tests**: Test individual components with mock data
- **Integration Tests**: Test the complete gRPC server-client flow
- **Mock Tests**: Test without requiring SR-IOV hardware
- **Real Hardware Tests**: Test with actual SR-IOV devices

Run all tests:

```bash
go test ./...
```
