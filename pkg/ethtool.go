package pkg

import (
	"fmt"
	"os/exec"
	"strings"
)



// EthtoolFeatureInfo represents a single feature
type EthtoolFeatureInfo struct {
	Name   string
	Enabled bool
	Fixed  bool
}

// EthtoolRingInfo represents ring parameters
type EthtoolRingInfo struct {
	RxMaxPending   uint32
	RxMiniMaxPending uint32
	RxJumboMaxPending uint32
	TxMaxPending   uint32
	RxPending      uint32
	RxMiniPending  uint32
	RxJumboPending uint32
	TxPending      uint32
}

// EthtoolChannelInfo represents channel parameters
type EthtoolChannelInfo struct {
	MaxRx          uint32
	MaxTx          uint32
	MaxOther       uint32
	MaxCombined    uint32
	RxCount        uint32
	TxCount        uint32
	OtherCount     uint32
	CombinedCount  uint32
}

// EthtoolInfo represents comprehensive ethtool information
type EthtoolInfo struct {
	Features  []EthtoolFeatureInfo
	Ring      EthtoolRingInfo
	Channels  EthtoolChannelInfo
}



// GetEthtoolInfo retrieves comprehensive ethtool information for a network interface
func GetEthtoolInfo(ifname string) (*EthtoolInfo, error) {
	info := &EthtoolInfo{}

	// Get features
	features, err := getEthtoolFeatures(ifname)
	if err != nil {
		return nil, fmt.Errorf("failed to get features: %v", err)
	}
	info.Features = features

	// Get ring parameters
	ring, err := getEthtoolRingParam(ifname)
	if err != nil {
		return nil, fmt.Errorf("failed to get ring parameters: %v", err)
	}
	info.Ring = *ring

	// Get channel parameters
	channels, err := getEthtoolChannels(ifname)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel parameters: %v", err)
	}
	info.Channels = *channels

	return info, nil
}

// getEthtoolFeatures retrieves ethtool features for a network interface
func getEthtoolFeatures(ifname string) ([]EthtoolFeatureInfo, error) {
	cmd := exec.Command("ethtool", "-k", ifname)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ethtool -k: %v", err)
	}

	features := []EthtoolFeatureInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Features for") {
			continue
		}

		// Parse lines like "rx-checksumming: on"
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Check if it's enabled
		enabled := strings.Contains(value, "on")
		
		// Check if it's fixed
		fixed := strings.Contains(value, "[fixed]")

		features = append(features, EthtoolFeatureInfo{
			Name:    name,
			Enabled: enabled,
			Fixed:   fixed,
		})
	}

	return features, nil
}

// getEthtoolRingParam retrieves ethtool ring parameters for a network interface
func getEthtoolRingParam(ifname string) (*EthtoolRingInfo, error) {
	cmd := exec.Command("ethtool", "-g", ifname)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ethtool -g: %v", err)
	}

	ring := &EthtoolRingInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse lines like "RX:             8192"
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse numeric value
		var num uint32
		if value == "n/a" {
			num = 0
		} else {
			fmt.Sscanf(value, "%d", &num)
		}

		switch key {
		case "RX":
			if strings.Contains(line, "Pre-set maximums") {
				ring.RxMaxPending = num
			} else {
				ring.RxPending = num
			}
		case "TX":
			if strings.Contains(line, "Pre-set maximums") {
				ring.TxMaxPending = num
			} else {
				ring.TxPending = num
			}
		case "RX Mini":
			if strings.Contains(line, "Pre-set maximums") {
				ring.RxMiniMaxPending = num
			} else {
				ring.RxMiniPending = num
			}
		case "RX Jumbo":
			if strings.Contains(line, "Pre-set maximums") {
				ring.RxJumboMaxPending = num
			} else {
				ring.RxJumboPending = num
			}
		}
	}

	return ring, nil
}

// getEthtoolChannels retrieves ethtool channel parameters for a network interface
func getEthtoolChannels(ifname string) (*EthtoolChannelInfo, error) {
	cmd := exec.Command("ethtool", "-l", ifname)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ethtool -l: %v", err)
	}

	channels := &EthtoolChannelInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse lines like "Combined:       63"
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse numeric value
		var num uint32
		if value == "n/a" {
			num = 0
		} else {
			fmt.Sscanf(value, "%d", &num)
		}

		switch key {
		case "RX":
			if strings.Contains(line, "Pre-set maximums") {
				channels.MaxRx = num
			} else {
				channels.RxCount = num
			}
		case "TX":
			if strings.Contains(line, "Pre-set maximums") {
				channels.MaxTx = num
			} else {
				channels.TxCount = num
			}
		case "Other":
			if strings.Contains(line, "Pre-set maximums") {
				channels.MaxOther = num
			} else {
				channels.OtherCount = num
			}
		case "Combined":
			if strings.Contains(line, "Pre-set maximums") {
				channels.MaxCombined = num
			} else {
				channels.CombinedCount = num
			}
		}
	}

	return channels, nil
}

// GetEthtoolFeaturesString returns features as a formatted string
func (info *EthtoolInfo) GetEthtoolFeaturesString() string {
	if len(info.Features) == 0 {
		return "No features available"
	}

	result := "Features:\n"
	for _, feature := range info.Features {
		status := "off"
		if feature.Enabled {
			status = "on"
		}
		
		fixed := ""
		if feature.Fixed {
			fixed = " [fixed]"
		}
		
		result += fmt.Sprintf("  %s: %s%s\n", feature.Name, status, fixed)
	}
	
	return result
}

// GetEthtoolRingString returns ring parameters as a formatted string
func (info *EthtoolInfo) GetEthtoolRingString() string {
	ring := info.Ring
	return fmt.Sprintf(`Ring parameters:
Pre-set maximums:
RX:             %d
RX Mini:        %s
RX Jumbo:       %s
TX:             %d
Current hardware settings:
RX:             %d
RX Mini:        %s
RX Jumbo:       %s
TX:             %d`,
		ring.RxMaxPending,
		formatRingValue(ring.RxMiniMaxPending),
		formatRingValue(ring.RxJumboMaxPending),
		ring.TxMaxPending,
		ring.RxPending,
		formatRingValue(ring.RxMiniPending),
		formatRingValue(ring.RxJumboPending),
		ring.TxPending)
}

// GetEthtoolChannelsString returns channel parameters as a formatted string
func (info *EthtoolInfo) GetEthtoolChannelsString() string {
	channels := info.Channels
	return fmt.Sprintf(`Channel parameters:
Pre-set maximums:
RX:             %s
TX:             %s
Other:          %s
Combined:       %d
Current hardware settings:
RX:             %s
TX:             %s
Other:          %s
Combined:       %d`,
		formatChannelValue(channels.MaxRx),
		formatChannelValue(channels.MaxTx),
		formatChannelValue(channels.MaxOther),
		channels.MaxCombined,
		formatChannelValue(channels.RxCount),
		formatChannelValue(channels.TxCount),
		formatChannelValue(channels.OtherCount),
		channels.CombinedCount)
}

// Helper functions
func formatRingValue(value uint32) string {
	if value == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d", value)
}

func formatChannelValue(value uint32) string {
	if value == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d", value)
} 