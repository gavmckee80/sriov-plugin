# SR-IOV Manager Service

## Overview

The SR-IOV Manager is a systemd service that automatically configures SR-IOV (Single Root I/O Virtualization) devices on system startup. It discovers SR-IOV capable network interfaces and applies configuration policies based on vendor and device IDs.

## Features

### **Core Functionality**
- **Automatic Device Discovery**: Discovers all SR-IOV capable devices using `lshw`
- **Policy-Based Configuration**: Applies device-specific policies based on vendor/device IDs
- **Switchdev Mode Support**: Enables switchdev mode for Mellanox ConnectX-7 devices
- **VF-LAG Bonding**: Supports VF-LAG mode for bonding multiple interfaces
- **Systemd Integration**: Runs as a systemd service with proper lifecycle management

### 🔧 **Supported Devices**
- **Mellanox ConnectX-7**: High-performance networking with switchdev mode
- **Pensando DSC**: ROCE (RDMA over Converged Ethernet) devices
- **Intel I350**: General-purpose networking devices
- **Virtual Functions**: Automatic VF creation and management

### **Configuration Modes**
- **Single-Home Mode**: Standard SR-IOV configuration
- **VF-LAG Mode**: Bond multiple interfaces for redundancy/performance

## Installation

### Prerequisites
```bash
# Install required tools
sudo apt update
sudo apt install -y lshw ethtool mlxconfig

# Ensure Go is installed
go version
```

### Installation Steps
```bash
# Clone the repository
git clone <repository-url>
cd sriov-plugin

# Run the installation script
sudo ./scripts/install.sh
```

The installation script will:
1. Build the SR-IOV manager binary
2. Install it to `/usr/local/bin/sriov-manager`
3. Create the systemd service file
4. Generate default configuration
5. Enable the service

## Configuration

### Configuration File Location
```
/etc/sriov-manager/config.json
```

### Default Configuration
```json
{
  "version": "1.0",
  "description": "SR-IOV Manager Configuration",
  "device_policies": [
    {
      "vendor_id": "15b3",
      "device_id": "101e",
      "num_vfs": 4,
      "mode": "single-home",
      "enable_switch": true,
      "description": "Mellanox ConnectX-7 in single-home mode"
    },
    {
      "vendor_id": "15b3",
      "device_id": "101e",
      "num_vfs": 4,
      "mode": "vf-lag",
      "enable_switch": true,
      "description": "Mellanox ConnectX-7 in VF-LAG mode"
    },
    {
      "vendor_id": "1dd8",
      "device_id": "1003",
      "num_vfs": 1,
      "mode": "single-home",
      "description": "Pensando DSC for ROCE"
    }
  ],
  "bond_configs": [
    {
      "bond_name": "bond0",
      "slave_interfaces": ["ens60f0np0", "ens60f1np1"],
      "mode": "active-backup",
      "mii_monitor": 100
    }
  ],
  "log_level": "info"
}
```

### Configuration Parameters

#### Device Policies
- **vendor_id**: PCI vendor ID (hex)
- **device_id**: PCI device ID (hex)
- **num_vfs**: Number of virtual functions to create
- **mode**: Configuration mode (`single-home` or `vf-lag`)
- **enable_switch**: Enable switchdev mode (Mellanox only)
- **description**: Human-readable description

#### Bond Configurations
- **bond_name**: Name of the bond interface
- **slave_interfaces**: List of interfaces to bond
- **mode**: Bond mode (`active-backup`, `balance-rr`, etc.)
- **mii_monitor**: MII monitoring interval (ms)

## Usage

### Command Line Interface

```bash
# Show version
sriov-manager --version

# Create default configuration
sriov-manager --create-config

# Discover devices only
sriov-manager --discover

# Validate configuration
sriov-manager --validate

# Run in dry-run mode
sriov-manager --dry-run

# Use custom config file
sriov-manager --config /path/to/config.json
```

### Systemd Service Management

```bash
# Start the service
sudo systemctl start sriov-manager

# Stop the service
sudo systemctl stop sriov-manager

# Enable auto-start
sudo systemctl enable sriov-manager

# Check status
sudo systemctl status sriov-manager

# View logs
sudo journalctl -u sriov-manager -f
```

## Device Support

### Mellanox ConnectX-7
- **Vendor ID**: `15b3`
- **Device ID**: `101e`
- **Features**: Switchdev mode, VF-LAG bonding
- **Use Cases**: High-performance networking, DPDK applications

### Pensando DSC
- **Vendor ID**: `1dd8`
- **Device ID**: `1003`
- **Features**: ROCE support, single VF per device
- **Use Cases**: RDMA applications, storage networking

### Intel I350
- **Vendor ID**: `8086`
- **Device ID**: `1520`
- **Features**: Standard SR-IOV support
- **Use Cases**: General networking, virtualization

## Monitoring and Troubleshooting

### Log Files
```bash
# View service logs
sudo journalctl -u sriov-manager

# View real-time logs
sudo journalctl -u sriov-manager -f

# View logs with timestamps
sudo journalctl -u sriov-manager --since "2024-01-01"
```

### Common Issues

#### Device Not Discovered
```bash
# Check if device is SR-IOV capable
lspci -vvv | grep -i sriov

# Verify lshw output
sudo lshw -class network
```

#### SR-IOV Not Enabled
```bash
# Check kernel support
cat /proc/cmdline | grep iommu

# Verify IOMMU is enabled
dmesg | grep -i iommu
```

#### Switchdev Mode Issues
```bash
# Check mlxconfig availability
which mlxconfig

# Verify device firmware
mlxconfig -d <pci-address> q
```

### Debug Mode
```bash
# Run with debug logging
sudo sriov-manager --config /etc/sriov-manager/config.json --dry-run
```

## Security Considerations

### File Permissions
- Configuration file: `644` (readable by all, writable by root)
- Binary: `755` (executable by all, writable by root)
- Service runs as root (required for PCI operations)

### Capabilities
The service requires:
- `CAP_NET_ADMIN`: Network interface management
- `CAP_SYS_ADMIN`: System administration (PCI operations)

### SELinux/AppArmor
If using SELinux or AppArmor, ensure the service has appropriate permissions for:
- `/sys/bus/pci/devices/*`
- `/sys/class/net/*`
- `/etc/sriov-manager/*`

## Performance Considerations

### Resource Usage
- **Memory**: ~10-50MB depending on device count
- **CPU**: Minimal during normal operation
- **Startup Time**: 5-30 seconds depending on device discovery

### Optimization Tips
1. **Limit VF Count**: Only create necessary VFs
2. **Use Dry-Run**: Test configurations before applying
3. **Monitor Logs**: Watch for device-specific issues
4. **Regular Validation**: Periodically validate configurations

## Integration Examples

### Kubernetes/OpenShift
```yaml
# Example SR-IOV Network Device Plugin configuration
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetworkNodePolicy
metadata:
  name: sriov-policy
spec:
  deviceType: netdevice
  nicSelector:
    vendor: "15b3"
    deviceID: "101e"
  numVfs: 4
  resourceName: mellanox_sriov
```

### OpenStack
```yaml
# Example Nova configuration
[neutron]
sriov_agent = True
sriov_agent_required = True

[ml2_type_sriov]
resource_provider_bandwidths = 15b3:101e:40000
```

## Development

### Building from Source
```bash
# Build the binary
go build -o bin/sriov-manager cmd/sriov-manager/main.go

# Build with debug symbols
go build -gcflags="-N -l" -o bin/sriov-manager cmd/sriov-manager/main.go
```

### Testing
```bash
# Run unit tests
go test ./pkg/...

# Run integration tests
go test ./cmd/sriov-manager/...

# Test device discovery
./bin/sriov-manager --discover
```

### Contributing
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

[Add your license information here]

## Support

For issues and questions:
- GitHub Issues: [repository-url]/issues
- Documentation: [repository-url]/docs
- Email: [support-email] 