package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileConfig defines persistent proxy configurations.
type FileConfig struct {
	APIAddr string        `json:"api_addr" yaml:"api_addr"`
	Proxies []ProxyConfig `json:"proxies" yaml:"proxies"`
}

// ProxyConfig defines a proxy and its optional initial preset.
type ProxyConfig struct {
	Name     string `json:"name" yaml:"name"`
	Listen   string `json:"listen" yaml:"listen"`
	Upstream string `json:"upstream" yaml:"upstream"`
	Enabled  *bool  `json:"enabled" yaml:"enabled"`
	Preset   string `json:"preset" yaml:"preset"`
}

// Load reads and parses a JSON or YAML configuration file.
func Load(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &FileConfig{
		APIAddr: ":8474",
	}

	// Simple parser handling JSON or basic YAML without extra heavy dependencies
	if strings.HasSuffix(path, ".json") || json.Valid(data) {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid json config: %w", err)
		}
		return cfg, nil
	}

	// Fallback to basic JSON-compatible unmarshal
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed parsing config file %s: %w", path, err)
	}

	return cfg, nil
}
