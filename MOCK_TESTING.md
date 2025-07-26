# Mock Testing for SR-IOV Plugin

This document explains how to test the SR-IOV plugin without requiring actual SR-IOV hardware on your development machine.

## Overview

The SR-IOV plugin includes comprehensive mock testing capabilities that allow you to:

- Test all functionality without SR-IOV hardware
- Develop and debug code on any machine
- Ensure code quality before deploying to production
- Test various device configurations and scenarios

## Mock Components

### 1. Mock PCI Data (`pkg/pci_mock.go`)

Provides realistic mock PCI device data including:
- Mellanox ConnectX-7 devices
- Pensando DSC Ethernet Controllers
- Intel I350 Gigabit Network Connections
- Intel X710 Ethernet Controllers
- Broadcom NetXtreme-E devices

### 2. Mock Tests (`pkg/device_mock_test.go`)

Comprehensive unit tests that verify:
- Mock PCI device parsing
- Device filtering by driver type
- Device filtering by vendor
- Device enrichment with mock data
- End-to-end workflows

### 3. Server Mock Tests (`cmd/server/mock_test.go`)

Tests the gRPC server with mock data:
- Full server-client communication
- Device discovery and listing
- Empty device scenarios
- Error handling

## Running Mock Tests

### Quick Test Script

Use the provided script to run all mock tests:

```bash
./scripts/test_with_mock.sh
```

### Manual Testing

1. **Run unit tests with mock data:**
   ```bash
   go test ./pkg -v
   ```

2. **Run server tests with mock data:**
   ```bash
   go test ./cmd/server -v
   ```

3. **Test specific mock functions:**
   ```bash
   go test ./pkg -run TestMockPciDevices
   go test ./pkg -run TestAttachPciInfoWithMock
   ```

## Mock Data Structure

### Mock PCI Devices

The mock system provides realistic PCI device data:

```go
PciDevice{
    Bus:          "0000:01:00.0",
    KernelDriver: "mlx5_core",
    VendorName:   "Mellanox Technologies",
    DeviceName:   "MT2910 Family [ConnectX-7]",
}
```

### Mock lshw Data

Sample mock lshw JSON structure:

```json
[
  {
    "id": "network",
    "class": "network",
    "claimed": true,
    "handle": "PCI:0000:01:00.0",
    "description": "Ethernet interface",
    "product": "MT2910 Family [ConnectX-7]",
    "vendor": "Mellanox Technologies",
    "businfo": "pci@0000:01:00.0",
    "logicalname": "ens1f0np0",
    "configuration": {
      "driver": "mlx5_core"
    }
  }
]
```

## Using Mock Data in Development

### 1. Override PCI Parsing

```go
// Use enhanced mock PCI data instead of real lspci
pkg.SetParsePciDevices(pkg.MockParseEnhancedPciDevices)
```

### 2. Create Mock lshw Files

```bash
# Create a mock lshw file for testing
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
    "businfo": "pci@0000:01:00.0",
    "logicalname": "ens1f0np0",
    "configuration": {
      "driver": "mlx5_core"
    }
  }
]
EOF
```

### 3. Test with Mock Data

```go
// Parse mock lshw data
devices, err := pkg.ParseLshw("mock-lshw.json")

// Override PCI parsing
pkg.SetParsePciDevices(pkg.MockParseEnhancedPciDevices)

// Enrich devices
enriched, err := pkg.AttachPciInfo(devices)
```

## Available Mock Functions

### Sysfs Mock Functions

- `MockParseSysfsPciDevices()` - Returns all sysfs mock PCI devices
- `MockParseSysfsPciDevicesByDriver(driver)` - Filter by driver type
- `MockParseSysfsPciDevicesByVendor(vendor)` - Filter by vendor
- `MockParseSysfsPciDevicesWithSRIOV()` - Returns only SR-IOV capable devices

### Filtering Examples

```go
// Get only Mellanox devices
devices, err := pkg.MockParseSysfsPciDevicesByDriver("mlx5_core")

// Get only Intel devices
devices, err := pkg.MockParseSysfsPciDevicesByVendor("Intel")

// Get all devices
devices, err := pkg.MockParseSysfsPciDevices()

// Get only SR-IOV capable devices
devices, err := pkg.MockParseSysfsPciDevicesWithSRIOV()
```

## Test Scenarios

### 1. Basic Device Discovery

Tests the complete flow from lshw parsing to gRPC response:

```bash
go test ./cmd/server -run TestServerWithMockData
```

### 2. Empty Device List

Tests behavior when no devices are found:

```bash
go test ./cmd/server -run TestServerWithEmptyData
```

### 3. Device Enrichment

Tests PCI data enrichment functionality:

```bash
go test ./pkg -run TestAttachPciInfoWithMock
```

### 4. Sysfs Filtering Tests

Tests sysfs device filtering capabilities:

```bash
go test ./pkg -run TestMockPciDevices
go test ./pkg -run TestMockPciDevicesWithFilter
go test ./pkg -run TestMockPciDevicesByVendor
```

## Development Workflow

1. **Write code** with mock data
2. **Test locally** using mock functions
3. **Verify functionality** with comprehensive tests
4. **Deploy to production** with confidence

## Benefits

- **No hardware required** for development
- **Fast iteration** on development machine
- **Comprehensive testing** of all code paths
- **Realistic data** that matches production scenarios
- **Easy debugging** with predictable mock data

## Production Deployment

When deploying to production:

1. Remove mock overrides
2. Use real lshw data
3. Use real lspci data
4. Verify with actual SR-IOV hardware

The mock system ensures your code is well-tested before reaching production hardware. 