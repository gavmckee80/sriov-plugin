#!/bin/bash

echo "Testing SR-IOV Device Monitoring System"
echo "=========================================="

# Test 1: Manual refresh
echo "1. Testing manual refresh..."
./bin/client --refresh

echo ""
echo "2. Getting current device list..."
./bin/client --device-name ens60f1npf1vf3 --format json | jq '.devices[0].name, .devices[0].driver, .devices[0].ethtool_info.features | length'

echo ""
echo "3. Monitoring for changes (this will run for 30 seconds)..."
echo "   You can manually trigger changes by:"
echo "   - Binding/unbinding VFIO drivers"
echo "   - Creating/destroying SR-IOV VFs"
echo "   - Changing network interface states"
echo ""

# Monitor for 30 seconds
timeout 30s ./bin/client --device-name ens60f1npf1vf3 --format table

echo ""
echo "Monitoring test completed!"
echo ""
echo "Key Features Implemented:"
echo "   - Real-time device change monitoring"
echo "   - VFIO driver binding detection"
echo "   - Network interface change detection"
echo "   - PCI device change detection"
echo "   - Manual refresh capability"
echo "   - Thread-safe concurrent access"
echo ""
echo "Perfect for your SR-IOV environment with:"
echo "   - Pensando VFs (ROCE)"
echo "   - Mellanox VFs (High-performance networking)"
echo "   - Dynamic VFIO binding/unbinding" 