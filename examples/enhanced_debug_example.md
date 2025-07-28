# Enhanced Debug Messages Example

The SR-IOV plugin now includes enhanced debug messages that provide detailed information about devices being discovered and monitored.

## Before Enhancement

Previously, debug messages only showed basic device information:

```
DEBU[0001] added watch for PF directory                  device="0000:85:00.0"
DEBU[0001] added watch for PF network interface         device="0000:85:00.0" interface="ens60f0np0"
```

## After Enhancement

Now debug messages include comprehensive device information:

```
DEBU[0001] added watch for PF directory                  device="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" vendor_name="Mellanox Technologies" device_id="0x1017" device_name="MT2910 Family [ConnectX-7]"
DEBU[0001] added watch for PF network interface         device="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" vendor_name="Mellanox Technologies" device_id="0x1017" device_name="MT2910 Family [ConnectX-7]"
DEBU[0001] added watch for critical file                device="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" vendor_name="Mellanox Technologies" device_id="0x1017" device_name="MT2910 Family [ConnectX-7]" file="sriov_numvfs"
DEBU[0001] added watch for critical file                device="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" vendor_name="Mellanox Technologies" device_id="0x1017" device_name="MT2910 Family [ConnectX-7]" file="sriov_totalvfs"
```

## Enhanced Discovery Messages

Device discovery messages now include vendor and device information:

```
DEBU[0001] found SR-IOV capable device                 pci="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" device_id="0x1017" vendor_name="Mellanox Technologies" device_name="MT2910 Family [ConnectX-7]"
DEBU[0001] found SR-IOV capable device                 pci="0000:02:00.0" interface="enp2s0np0" vendor_id="0x1dd8" device_id="0x1001" vendor_name="Pensando Systems" device_name="DSC Ethernet Controller"
```

## Vendor Filtering Messages

When vendor filtering is enabled, you'll see clear messages about which devices are being skipped:

```
DEBU[0001] skipping device - vendor not allowed        pci="0000:03:00.0" vendor="0x8086" device="0x1572" interface="enp3s0np0"
DEBU[0001] skipping device - vendor not allowed        pci="0000:04:00.0" vendor="0x10df" device="0x0720" interface="enp4s0np0"
```

## Benefits

1. **Better Debugging**: Easier to identify which specific devices are being monitored
2. **Vendor Identification**: Clear indication of vendor and device types
3. **Troubleshooting**: Helps identify issues with specific device types
4. **Monitoring**: Better visibility into the discovery and monitoring process

## Usage Examples

### Run with vendor filtering and enhanced debug

```bash
# Start server with vendor filtering and debug logging
./sriovd -allowed-vendors="0x15b3,0x1dd8" -log-level=debug

# Example output:
DEBU[0001] discovery configuration                        allowed_vendors="[0x15b3 0x1dd8]" excluded_vendors="[]" enable_representors=true enable_switchdev_check=true
DEBU[0001] found SR-IOV capable device                 pci="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" device_id="0x1017" vendor_name="Mellanox Technologies" device_name="MT2910 Family [ConnectX-7]"
DEBU[0001] added watch for PF directory                  device="0000:85:00.0" interface="ens60f0np0" vendor_id="0x15b3" vendor_name="Mellanox Technologies" device_id="0x1017" device_name="MT2910 Family [ConnectX-7]"
DEBU[0001] skipping device - vendor not allowed        pci="0000:03:00.0" vendor="0x8086" device="0x1572" interface="enp3s0np0"
```

### Monitor specific device types

```bash
# Monitor only Intel devices
./sriovd -allowed-vendors="0x8086" -log-level=debug

# Example output:
DEBU[0001] found SR-IOV capable device                 pci="0000:03:00.0" interface="enp3s0np0" vendor_id="0x8086" device_id="0x1572" vendor_name="Intel Corporation" device_name="Ethernet Controller X710"
DEBU[0001] added watch for PF directory                  device="0000:03:00.0" interface="enp3s0np0" vendor_id="0x8086" vendor_name="Intel Corporation" device_id="0x1572" device_name="Ethernet Controller X710"
```

## Configuration

The enhanced debug messages work with all existing configuration options:

- Command-line vendor filtering
- Configuration file settings
- Representor discovery settings
- Switchdev mode checking

All debug messages now include the enhanced device information for better visibility and troubleshooting. 