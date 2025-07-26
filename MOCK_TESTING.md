# Mock Testing Documentation

This document describes the mock testing system for the SR-IOV plugin, which allows development and testing without requiring actual SR-IOV hardware.

## Overview

The mock testing system provides:
- **Mock PCI device data** for testing sysfs-based PCI parsing
- **Mock lshw data** for testing network device discovery
- **Dependency injection** to override real hardware queries
- **Comprehensive test coverage** without requiring SR-IOV hardware

## Available Mock Functions

### Sysfs Mock Functions

- `MockParseSysfsPciDevices()` - Returns all sysfs mock PCI devices
- `MockParseSysfsPciDevicesByDriver(driver)` - Filter by driver type
- `MockParseSysfsPciDevicesByVendor(vendor)` - Filter by vendor
- `MockParseSysfsPciDevicesWithSRIOV()` - Returns only SR-IOV capable devices

### Filtering Examples

```go
// Get all mock devices
devices, err := pkg.MockParseSysfsPciDevices()

// Get only Mellanox devices
devices, err := pkg.MockParseSysfsPciDevicesByVendor("Mellanox Technologies")

// Get only mlx5_core driver devices
devices, err := pkg.MockParseSysfsPciDevicesByDriver("mlx5_core")

// Get only SR-IOV capable devices
devices, err := pkg.MockParseSysfsPciDevicesWithSRIOV()
```

## Enhanced Device Information

The `Device` struct now includes comprehensive information from both `lshw` and sysfs:

```go
type Device struct {
    // Basic information
    PCIAddress string
    Name       string
    Driver     string
    Vendor     string
    Product    string
    
    // SR-IOV information
    SRIOVCapable bool
    SRIOVInfo    *SRIOVInfo
    
    // Enhanced context from lshw
    Description string
    Serial      string
    Size        string
    Capacity    string
    Clock       string
    Width       string
    Capabilities []string
    
    // Device classification
    Class       string
    SubClass    string
    
    // Network-specific information
    LogicalName string
    BusInfo     string
    
    // Configuration details
    Configuration map[string]interface{}
}
```

## Dynamic vs Static Data Collection

### Production Mode (Default)
The server runs `lshw -class network -json` dynamically to gather real-time hardware information:

```bash
# Run server in production mode (default)
./bin/server

# Or explicitly
./bin/server -file=false
```

### Development/Testing Mode
Use static files for consistent testing:

```bash
# Use static file for testing
./bin/server -file -lshw-file=./lshw-network.json
```

## Test Scenarios

### 1. Basic Mock Testing

```go
func TestBasicMock(t *testing.T) {
    // Override PCI parsing with mock
    pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevices)
    
    // Create mock lshw data
    mockData := []map[string]interface{}{
        {
            "businfo": "pci@0000:01:00.0",
            "logicalname": "eth0",
            "vendor": "Mellanox Technologies",
            "product": "MT2910 Family [ConnectX-7]",
            "configuration": map[string]interface{}{
                "driver": "mlx5_core",
            },
        },
    }
    
    // Test parsing and enrichment
    devices, err := pkg.ParseLshwFromFile("mock-file.json")
    if err != nil {
        t.Fatalf("Failed to parse: %v", err)
    }
    
    devices, err = pkg.AttachPciInfo(devices)
    if err != nil {
        t.Fatalf("Failed to enrich: %v", err)
    }
    
    // Verify results
    if len(devices) != 1 {
        t.Errorf("Expected 1 device, got %d", len(devices))
    }
}
```

### 2. SR-IOV Testing

```go
func TestSRIOVMock(t *testing.T) {
    // Use SR-IOV specific mock
    pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevicesWithSRIOV)
    
    // Test SR-IOV capabilities
    devices, err := pkg.MockParseSysfsPciDevicesWithSRIOV()
    if err != nil {
        t.Fatalf("Failed to get SR-IOV devices: %v", err)
    }
    
    for _, device := range devices {
        if !device.SRIOVCapable {
            t.Errorf("Expected SR-IOV capable device")
        }
        if device.SRIOVInfo == nil {
            t.Errorf("Expected SR-IOV info")
        }
    }
}
```

### 3. Vendor-Specific Testing

```go
func TestVendorFiltering(t *testing.T) {
    // Test Mellanox devices
    devices, err := pkg.MockParseSysfsPciDevicesByVendor("Mellanox Technologies")
    if err != nil {
        t.Fatalf("Failed to get Mellanox devices: %v", err)
    }
    
    for _, device := range devices {
        if device.VendorName != "Mellanox Technologies" {
            t.Errorf("Expected Mellanox device, got %s", device.VendorName)
        }
    }
}
```

## Integration Testing

### Server with Mock Data

```go
func TestServerWithMock(t *testing.T) {
    // Override with mock data
    pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevices)
    
    // Create mock lshw data
    mockData := []map[string]interface{}{
        {
            "businfo": "pci@0000:01:00.0",
            "logicalname": "eth0",
            "vendor": "Mellanox Technologies",
            "product": "MT2910 Family [ConnectX-7]",
            "configuration": map[string]interface{}{
                "driver": "mlx5_core",
            },
        },
    }
    
    // Write to temp file
    tmpfile, err := os.CreateTemp("", "mock-*.json")
    if err != nil {
        t.Fatalf("Failed to create temp file: %v", err)
    }
    defer os.Remove(tmpfile.Name())
    
    if err := json.NewEncoder(tmpfile).Encode(mockData); err != nil {
        t.Fatalf("Failed to write mock data: %v", err)
    }
    tmpfile.Close()
    
    // Parse and test
    devices, err := pkg.ParseLshwFromFile(tmpfile.Name())
    if err != nil {
        t.Fatalf("Failed to parse: %v", err)
    }
    
    devices, err = pkg.AttachPciInfo(devices)
    if err != nil {
        t.Fatalf("Failed to enrich: %v", err)
    }
    
    // Test gRPC server
    srv := &server{devices: devices}
    resp, err := srv.ListDevices(context.Background(), &pb.ListDevicesRequest{})
    if err != nil {
        t.Fatalf("ListDevices failed: %v", err)
    }
    
    if len(resp.Devices) != 1 {
        t.Errorf("Expected 1 device, got %d", len(resp.Devices))
    }
}
```

## Running Tests

### All Tests
```bash
make test
```

### Specific Test Files
```bash
go test ./pkg -v
go test ./cmd/server -v
```

### With Coverage
```bash
make test-coverage
```

## Mock Data Structure

### SysfsPciDevice Mock
```go
SysfsPciDevice{
    Bus:          "0000:01:00.0",
    KernelDriver: "mlx5_core",
    VendorName:   "Mellanox Technologies",
    DeviceName:   "MT2910 Family [ConnectX-7]",
    VendorID:     "15b3",
    DeviceID:     "101e",
    SRIOVCapable: true,
    SRIOVInfo: &SRIOVInfo{
        TotalVFs:     16,
        NumberOfVFs:  4,
        VFOffset:     2,
        VFStride:     1,
        VFDeviceID:   "101e",
    },
}
```

### Enhanced Device Mock
```go
Device{
    PCIAddress:    "0000:01:00.0",
    Name:          "eth0",
    Driver:        "mlx5_core",
    Vendor:        "Mellanox Technologies",
    Product:       "MT2910 Family [ConnectX-7]",
    Description:   "Ethernet interface",
    Serial:        "00:1b:21:0a:8b:2a",
    Size:          "10Gbit/s",
    Capacity:      "10Gbit/s",
    Clock:         "33MHz",
    Width:         "8 bits",
    Class:         "network",
    SubClass:      "ethernet",
    Capabilities:  []string{"pm", "msi", "pciexpress", "msix", "bus_master", "cap_list"},
    SRIOVCapable:  true,
    SRIOVInfo:     &SRIOVInfo{...},
}
```

## Best Practices

1. **Use mocks for unit tests** - Don't depend on real hardware
2. **Test both success and failure cases** - Mock errors and edge cases
3. **Verify data enrichment** - Ensure PCI info is properly attached
4. **Test SR-IOV capabilities** - Verify SR-IOV detection and info
5. **Use appropriate mock functions** - Choose the right mock for your test scenario
6. **Clean up after tests** - Remove temporary files and reset overrides

## Troubleshooting

### Common Issues

1. **Mock not working**: Ensure you're calling `SetParsePciDevices()` before parsing
2. **Missing SR-IOV info**: Use `MockParseSysfsPciDevicesWithSRIOV()` for SR-IOV testing
3. **Test failures**: Check that mock data matches expected structure
4. **File not found**: Ensure mock files are created in tests

### Debug Tips

1. **Enable verbose logging**: Use `-v` flag with tests
2. **Check mock data**: Print mock devices to verify structure
3. **Verify enrichment**: Check that PCI info is properly attached
4. **Test isolation**: Ensure tests don't interfere with each other 