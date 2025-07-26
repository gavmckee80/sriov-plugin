package main

import (
	"encoding/json"
	"fmt"
	"log"

	"example.com/sriov-plugin/proto"
)

func main() {
	// Create a test device with ethtool info
	device := &proto.Device{
		PciAddress: "0000:31:00.1",
		Name:       "ens60f1npf1vf3",
		Driver:     "mlx5e_rep",
		EthtoolInfo: &proto.EthtoolInfo{
			Features: []*proto.EthtoolFeature{
				{
					Name:    "rx-checksumming",
					Enabled: true,
					Fixed:   false,
				},
			},
			Ring: &proto.EthtoolRingInfo{
				RxPending: 1024,
				TxPending: 1024,
			},
			Channels: &proto.EthtoolChannelInfo{
				CombinedCount: 1,
			},
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(device)
	if err != nil {
		log.Fatalf("Failed to marshal to JSON: %v", err)
	}

	fmt.Printf("JSON output:\n%s\n", string(jsonData))

	// Check if ethtool_info is present
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if ethtoolInfo, exists := result["ethtool_info"]; exists {
		fmt.Printf("✅ ethtool_info field exists: %v\n", ethtoolInfo)
	} else {
		fmt.Printf("❌ ethtool_info field is missing\n")
		fmt.Printf("Available fields: %v\n", result)
	}
}
