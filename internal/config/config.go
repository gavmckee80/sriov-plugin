package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the SR-IOV configuration
type Config struct {
	Pools []Pool `yaml:"pools"`
}

// Pool represents a VF pool configuration
type Pool struct {
	Name             string   `yaml:"name"`
	PfPCI            string   `yaml:"pf_pci"`
	VFRange          string   `yaml:"vf_range"`
	Mask             bool     `yaml:"mask"`
	MaskReason       string   `yaml:"mask_reason"`
	RequiredFeatures []string `yaml:"required_features"`
	NUMA             string   `yaml:"numa"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// ParseVFRange parses a VF range string like "0-3,5,7-9"
func ParseVFRange(rangeStr string) ([]int, error) {
	var indices []int
	parts := strings.Split(rangeStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range like "0-3"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start index: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end index: %s", rangeParts[1])
			}
			for i := start; i <= end; i++ {
				indices = append(indices, i)
			}
		} else {
			// Single index
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid index: %s", part)
			}
			indices = append(indices, index)
		}
	}

	return indices, nil
}
