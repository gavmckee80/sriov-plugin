# SR-IOV Device Monitoring System

## 🎯 **Enhanced Monitoring Features**

This implementation provides comprehensive real-time monitoring for SR-IOV devices, specifically designed to handle dynamic environments with VFIO driver binding and VM passthrough scenarios.

## 🔍 **What Gets Monitored**

### **1. Network Interface Changes**
- **Path**: `/sys/class/net/`
- **Events**: Interface creation, deletion, state changes
- **Impact**: Catches when VFs are bound/unbound to VFIO drivers
- **Use Case**: When `ens60f1npf1vf3` disappears from network stack due to VFIO binding

### **2. PCI Device Changes**
- **Path**: `/sys/bus/pci/devices/`
- **Events**: New devices, device removal, driver changes
- **Impact**: Detects SR-IOV VF creation/destruction
- **Use Case**: New VFs created or existing VFs destroyed

### **3. Driver Binding Changes**
- **VFIO Driver**: `/sys/bus/pci/drivers/vfio-pci/`
- **Mellanox Driver**: `/sys/bus/pci/drivers/mlx5e_rep/`
- **Events**: Driver binding/unbinding
- **Impact**: Detects when devices switch between network and passthrough modes
- **Use Case**: VF switching from `mlx5e_rep` to `vfio-pci` for VM passthrough

### **4. SR-IOV Configuration Changes**
- **Path**: `/sys/bus/pci/devices/*/sriov_numvfs`
- **Events**: VF count changes, SR-IOV enable/disable
- **Impact**: Detects SR-IOV configuration modifications
- **Use Case**: Dynamic VF allocation for ROCE or high-performance networking

## 🛠️ **Implementation Details**

### **Real-time Event Processing**
```go
// Event-driven architecture
type DeviceEvent struct {
    Type      string // "interface", "pci", "driver", "sriov"
    Action    string // "created", "deleted", "modified"
    Device    string
    Timestamp time.Time
}
```

### **Thread-safe Data Access**
```go
// Concurrent read/write protection
type server struct {
    devices     []pkg.Device
    devicesLock sync.RWMutex
    lastUpdate  time.Time
    watchers    []*DeviceWatcher
}
```

### **Selective Device Updates**
- **Event-driven**: Only refresh when changes detected
- **Full refresh**: Complete device list regeneration
- **Manual refresh**: API endpoint for forced updates

## 🎯 **Perfect for Your Environment**

### **Pensando VFs (ROCE)**
- **Use Case**: RDMA over Converged Ethernet
- **Monitoring**: VFIO binding for VM passthrough
- **Detection**: When VFs switch from network to passthrough mode

### **Mellanox VFs (High-performance Networking)**
- **Use Case**: High-throughput network interfaces
- **Monitoring**: Driver binding changes
- **Detection**: `mlx5e_rep` ↔ `vfio-pci` transitions

### **Dynamic SR-IOV Management**
- **Use Case**: On-demand VF allocation
- **Monitoring**: VF count changes
- **Detection**: Real-time VF creation/destruction

## 📊 **Performance Characteristics**

### **Fast Response Times**
- ✅ **Cached data**: Sub-second response for queries
- ✅ **Event-driven updates**: Only refresh when needed
- ✅ **Thread-safe**: Concurrent access without blocking

### **Real-time Accuracy**
- ✅ **Immediate detection**: Changes detected within seconds
- ✅ **Comprehensive monitoring**: All relevant system paths watched
- ✅ **Automatic recovery**: Failed monitors restart automatically

### **Resource Efficiency**
- ✅ **Selective updates**: Only affected devices refreshed
- ✅ **Memory efficient**: Cached data with automatic cleanup
- ✅ **CPU friendly**: Event-driven vs polling

## 🚀 **Usage Examples**

### **Manual Refresh**
```bash
# Force immediate refresh of all device data
./bin/client --refresh
```

### **Monitor Specific Device**
```bash
# Get real-time data for specific VF
./bin/client --device-name ens60f1npf1vf3 --format json
```

### **Watch for Changes**
```bash
# Monitor device list for 30 seconds
timeout 30s ./bin/client --device-name ens60f1npf1vf3 --format table
```

## 🔧 **Technical Architecture**

### **Monitoring Layers**
1. **Inotify-based**: File system event monitoring
2. **Driver-specific**: VFIO and Mellanox driver monitoring
3. **Periodic checks**: SR-IOV configuration monitoring
4. **Manual triggers**: API-based refresh capability

### **Event Processing Pipeline**
```
File System Event → DeviceEvent → Event Handler → Device Refresh → Cache Update
```

### **Concurrent Access Pattern**
```
Read Request → RLock → Return Cached Data → RUnlock
Write Event → WLock → Refresh Data → Update Cache → WUnlock
```

## ✅ **Benefits for Your Use Case**

### **VFIO Binding Detection**
- **Problem**: VFs disappear from network stack when bound to VFIO
- **Solution**: Real-time detection of driver binding changes
- **Result**: Accurate device state tracking

### **Dynamic VF Management**
- **Problem**: VFs created/destroyed dynamically
- **Solution**: PCI device change monitoring
- **Result**: Immediate detection of VF lifecycle events

### **Multi-vendor Support**
- **Problem**: Different vendors have different driver patterns
- **Solution**: Vendor-specific driver monitoring
- **Result**: Support for Pensando, Mellanox, and other vendors

## 🎉 **Summary**

This enhanced monitoring system provides:

1. **Real-time accuracy** with fast response times
2. **Comprehensive coverage** of all relevant device changes
3. **Thread-safe operation** for concurrent access
4. **Automatic recovery** from monitoring failures
5. **Manual control** for immediate refresh when needed

**Perfect for production SR-IOV environments with dynamic VF management and VM passthrough scenarios.** 