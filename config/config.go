package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/your-org/seictl/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from the specified path
func LoadConfig(path string) (*types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &types.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand home directory in paths
	config.Global.HomeDir = expandPath(config.Global.HomeDir)
	config.Global.BackupDir = expandPath(config.Global.BackupDir)

	return config, nil
}

// SaveConfig saves configuration to the specified path
func SaveConfig(config *types.Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if path == "" {
		return path
	}

	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}

	return path
}
