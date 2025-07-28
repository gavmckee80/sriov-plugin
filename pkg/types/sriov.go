package types

// PFInfo represents a Physical Function
type PFInfo struct {
	PCIAddress       string             `json:"pci_address"`
	InterfaceName    string             `json:"interface_name"`
	Driver           string             `json:"driver"`
	TotalVFs         int                `json:"total_vfs"`
	NumVFs           int                `json:"num_vfs"`
	SRIOVEnabled     bool               `json:"sriov_enabled"`
	NUMANode         string             `json:"numa_node"`
	LinkState        string             `json:"link_state"`
	LinkSpeed        string             `json:"link_speed"`
	MTU              string             `json:"mtu"`
	MACAddress       string             `json:"mac_address"`
	Features         map[string]bool    `json:"features"`
	Channels         map[string]int     `json:"channels"`
	Rings            map[string]int     `json:"rings"`
	Properties       map[string]string  `json:"properties"`
	Capabilities     []PCICapability    `json:"capabilities"`
	DeviceClass      string             `json:"device_class"`
	VendorID         string             `json:"vendor_id"`
	DeviceID         string             `json:"device_id"`
	SubsysVendor     string             `json:"subsys_vendor"`
	SubsysDevice     string             `json:"subsys_device"`
	Description      string             `json:"description"`
	VendorName       string             `json:"vendor_name"`
	DeviceName       string             `json:"device_name"`
	SubsysVendorName string             `json:"subsys_vendor_name"`
	SubsysDeviceName string             `json:"subsys_device_name"`
	VFs              map[string]*VFInfo `json:"vfs"`
}

// VFInfo represents a Virtual Function
type VFInfo struct {
	PCIAddress       string            `json:"pci_address"`
	PFPCIAddress     string            `json:"pf_pci_address"`
	InterfaceName    string            `json:"interface_name"`
	VFIndex          int               `json:"vf_index"`
	Driver           string            `json:"driver"`
	LinkState        string            `json:"link_state"`
	LinkSpeed        string            `json:"link_speed"`
	NUMANode         string            `json:"numa_node"`
	MTU              string            `json:"mtu"`
	MACAddress       string            `json:"mac_address"`
	Allocated        bool              `json:"allocated"`
	Masked           bool              `json:"masked"`
	Pool             string            `json:"pool"`
	Features         map[string]bool   `json:"features"`
	Channels         map[string]int    `json:"channels"`
	Rings            map[string]int    `json:"rings"`
	Properties       map[string]string `json:"properties"`
	Capabilities     []PCICapability   `json:"capabilities"`
	DeviceClass      string            `json:"device_class"`
	VendorID         string            `json:"vendor_id"`
	DeviceID         string            `json:"device_id"`
	SubsysVendor     string            `json:"subsys_vendor"`
	SubsysDevice     string            `json:"subsys_device"`
	Description      string            `json:"description"`
	VendorName       string            `json:"vendor_name"`
	DeviceName       string            `json:"device_name"`
	SubsysVendorName string            `json:"subsys_vendor_name"`
	SubsysDeviceName string            `json:"subsys_device_name"`
}

// PCICapability represents a PCI capability
type PCICapability struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version,omitempty"`
	Data        map[string]string `json:"data,omitempty"`
	Description string            `json:"description,omitempty"`
}

// SRIOVData represents the complete SR-IOV device information
type SRIOVData struct {
	PhysicalFunctions map[string]*PFInfo `json:"physical_functions"`
	VirtualFunctions  map[string]*VFInfo `json:"virtual_functions"`
}
