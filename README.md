# SR-IOV Plugin

A comprehensive SR-IOV device discovery and management tool with vendor ID filtering capabilities.

## Features

- **SR-IOV Device Discovery**: Automatically discovers Physical Functions (PFs) and Virtual Functions (VFs)
- **Vendor ID Filtering**: Limit discovery to specific vendor IDs or exclude unwanted vendors
- **Representor Discovery**: Discover and manage representor interfaces for switchdev mode
- **Real-time Monitoring**: File system monitoring for dynamic SR-IOV changes
- **gRPC API**: Programmatic access to SR-IOV device information
- **Configuration Management**: YAML-based configuration with command-line overrides

## Vendor ID Filtering

The plugin supports filtering devices by vendor ID to limit discovery and management to specific device types.

### Supported Vendors

| Vendor ID | Vendor Name | Description |
|-----------|-------------|-------------|
| `0x15b3` | Mellanox Technologies | High-performance networking |
| `0x8086` | Intel Corporation | Ethernet controllers |
| `0x1dd8` | Pensando Systems | SmartNIC solutions |
| `0x10df` | Emulex Corporation | Fibre Channel adapters |
| `0x1077` | QLogic Corp | Storage networking |
| `0x14e4` | Broadcom Inc | Network adapters |
| `0x1924` | Solarflare Communications | High-frequency trading |
| `0x19e5` | Huawei Technologies Co., Ltd | Enterprise networking |

### Configuration Options

#### Configuration File (`config.yaml`)

```yaml
# Discovery configuration
discovery:
  # List of vendor IDs to discover and manage
  # If empty, all vendors are allowed
  allowed_vendor_ids:
    - "0x15b3"  # Mellanox Technologies
    - "0x8086"  # Intel Corporation
    - "0x1dd8"  # Pensando Systems
  
  # List of vendor IDs to exclude from discovery
  # This takes precedence over allowed_vendor_ids
  excluded_vendor_ids:
    - "0x1234"  # Example excluded vendor
  
  # Enable/disable representor discovery
  enable_representor_discovery: true
  
  # Enable/disable switchdev mode checking
  enable_switchdev_mode_check: true

# VF Pool configurations
pools:
  - name: "high-performance"
    pf_pci: "0000:31:00.0"
    vf_range: "0-63"
    mask: false
    required_features: ["rx_checksum", "tx_checksum"]
    numa: "0"
```

#### Command-Line Options

```bash
# Start server with specific vendor filtering
./sriovd -allowed-vendors="0x15b3,0x8086" -excluded-vendors="0x1234"

# Disable representor discovery
./sriovd -enable-representors=false

# Disable switchdev mode checking
./sriovd -enable-switchdev-check=false

# Use custom configuration file
./sriovd -config="/path/to/config.yaml"

# Change server port
./sriovd -port="50052"
```

### Filtering Logic

1. **Excluded vendors take precedence**: If a vendor ID is in the excluded list, it will be skipped regardless of the allowed list
2. **Empty allowed list**: If no vendors are specified in `allowed_vendor_ids`, all vendors are allowed (except excluded ones)
3. **Specific allowed vendors**: If vendors are specified in `allowed_vendor_ids`, only those vendors are discovered
4. **Command-line override**: Command-line flags override configuration file settings

## Building

```bash
# Build both server and client
make build

# Build with race detection
make build-race

# Build for Linux
make build-linux
```

## Running

### Server

```bash
# Start with default configuration
./bin/sriovd

# Start with vendor filtering
./bin/sriovd -allowed-vendors="0x15b3,0x8086" -excluded-vendors="0x1234"

# Start with custom config
./bin/sriovd -config="my-config.yaml"

# Start with specific port
./bin/sriovd -port="50052"
```

### Client

```bash
# List all devices
./bin/sriovctl list

# List devices by vendor
./bin/sriovctl list --vendor=0x15b3

# List devices by PCI address
./bin/sriovctl list --pci=0000:31:00.0

# List devices by interface name
./bin/sriovctl list --interface=ens60f0np0

# Get detailed information
./bin/sriovctl get --interface=ens60f0np0

# Get vendor statistics
./bin/sriovctl vendors

# Get server status
./bin/sriovctl status
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint code
make lint

# Format code
make format

# Start development server
make dev-server

# Start development client
make dev-client
```

## Docker

```bash
# Build Docker image
make docker-build

# Run Docker container
make docker-run
```

## Examples

### Filter for Mellanox and Intel devices only

```bash
./sriovd -allowed-vendors="0x15b3,0x8086"
```

### Exclude specific vendors

```bash
./sriovd -excluded-vendors="0x1234,0x5678"
```

### Disable representor discovery for performance

```bash
./sriovd -enable-representors=false
```

### Use configuration file with vendor filtering

```yaml
# config.yaml
discovery:
  allowed_vendor_ids:
    - "0x15b3"  # Mellanox
    - "0x8086"  # Intel
  excluded_vendor_ids:
    - "0x1234"  # Exclude specific vendor
  enable_representor_discovery: true
  enable_switchdev_mode_check: true
```

```bash
./sriovd -config="config.yaml"
```

## API

The server exposes a gRPC API for programmatic access:

```go
// Connect to server
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := pb.NewSriovDeviceManagerClient(conn)

// Get interface dump
resp, err := client.DumpInterfaces(ctx, &pb.Empty{})
```

## License

[Add your license information here]
