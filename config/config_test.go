package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/seictl/pkg/types"
)

func init() {
	// Set test mode
	os.Setenv("SEICTL_TEST", "1")
}

func TestLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seictl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configData := []byte(`
version: "1.0"
global:
  home_dir: "~/.sei"
  backup_dir: "~/sei_backup"
  log_level: "INFO"
  timeout_seconds: 30
  max_retries: 3
  retry_delay_seconds: 5

environments:
  mainnet:
    chain_id: "pacific-1"
    version: "v5.9.0-hotfix"
    rpc_endpoints:
      - "https://rpc1.sei.io"
`)

	err = os.WriteFile(configPath, configData, 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, "INFO", config.Global.LogLevel)
	assert.Equal(t, 30, config.Global.TimeoutSeconds)
	assert.Equal(t, "~/.sei", config.Global.HomeDir) // Should not be expanded in test mode

	mainnet, exists := config.Environments["mainnet"]
	assert.True(t, exists)
	assert.Equal(t, "pacific-1", mainnet.ChainID)
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing version",
			content: `
global:
  home_dir: "~/.sei"
  log_level: "INFO"`,
			wantErr: "version is required in config file",
		},
		{
			name: "invalid yaml",
			content: `
version: 1.0
global:
  - invalid`,
			wantErr: "failed to parse config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "seictl-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			_, err = LoadConfig(configPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seictl-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	testConfig := &types.Config{
		Version: "1.0",
		Global: types.GlobalConfig{
			HomeDir:   "~/.sei",
			LogLevel:  "INFO",
			BackupDir: "~/sei_backup",
		},
	}

	err = SaveConfig(testConfig, configPath)
	require.NoError(t, err)

	loadedConfig, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, testConfig.Version, loadedConfig.Version)
	assert.Equal(t, testConfig.Global.HomeDir, loadedConfig.Global.HomeDir)
}
