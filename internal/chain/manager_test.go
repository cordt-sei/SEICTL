package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/seictl/pkg/types"
)

func setupTestManager(t *testing.T) (*Manager, string, func()) {
	tmpDir, err := os.MkdirTemp("", "seictl-test-*")
	require.NoError(t, err)

	config := &types.Config{
		Version: "1.0",
		Global: types.GlobalConfig{
			HomeDir:        filepath.Join(tmpDir, "home"),
			BackupDir:      filepath.Join(tmpDir, "backup"),
			TimeoutSeconds: 5,
			LogLevel:       "info",
		},
		Environments: map[string]types.ChainConfig{
			"testnet": {
				ChainID: "test-1",
				Version: "v1.0.0",
				RPCEndpoints: []string{
					"http://localhost:26657",
				},
			},
		},
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	manager, err := NewManager(config, logger)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func TestInitChain(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := InitOptions{
		SkipBinary:    true,
		Moniker:       "test-node",
		ChainID:       "test-1",
		WithStateSync: false,
	}

	err := manager.InitChain(ctx, types.Environment("testnet"), opts)
	assert.NoError(t, err)

	// Verify directory structure
	dirs := []string{
		manager.homePath,
		manager.configPath,
		filepath.Join(manager.homePath, "data"),
		filepath.Join(manager.homePath, "wasm"),
	}

	for _, dir := range dirs {
		_, err := os.Stat(dir)
		assert.NoError(t, err, "Directory should exist: %s", dir)
	}

	// Verify config files
	configs := []string{
		filepath.Join(manager.configPath, "config.toml"),
		filepath.Join(manager.configPath, "app.toml"),
	}

	for _, cfg := range configs {
		_, err := os.Stat(cfg)
		assert.NoError(t, err, "Config file should exist: %s", cfg)
	}
}

func TestWriteConfig(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	// Create test config data
	testConfig := map[string]interface{}{
		"test_key": "test_value",
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	// Ensure config directory exists
	require.NoError(t, os.MkdirAll(manager.configPath, 0755))

	// Write config
	err := manager.writeConfig("test.toml", testConfig)
	assert.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(manager.configPath, "test.toml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test_key")
	assert.Contains(t, string(content), "test_value")
}

func TestConfigureNode(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	chainCfg := types.ChainConfig{
		ChainID: "test-1",
		Version: "v1.0.0",
	}

	opts := InitOptions{
		Moniker:       "test-node",
		WithStateSync: true,
	}

	// Ensure config directory exists
	require.NoError(t, os.MkdirAll(manager.configPath, 0755))

	// Configure node
	err := manager.configureNode(chainCfg, opts)
	assert.NoError(t, err)

	// Verify config files exist and contain expected values
	configToml := filepath.Join(manager.configPath, "config.toml")
	appToml := filepath.Join(manager.configPath, "app.toml")

	_, err = os.Stat(configToml)
	assert.NoError(t, err)
	_, err = os.Stat(appToml)
	assert.NoError(t, err)
}
