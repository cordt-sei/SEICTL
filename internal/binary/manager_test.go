package binary

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/your-org/seictl/pkg/types"
)

func setupTestManager(t *testing.T) (*Manager, string, func()) {
	tmpDir, err := os.MkdirTemp("", "seictl-test-*")
	require.NoError(t, err)

	config := &types.Config{
		Version: "1.0",
		Global: types.GlobalConfig{
			TimeoutSeconds: 5,
			LogLevel:       "info",
		},
		Environments: map[string]types.ChainConfig{
			"testnet": {
				Version: "v1.0.0",
				// Use proper URL formatting strings
				BinaryURL:         "https://example.com/seid-%v-linux-amd64",
				BinaryChecksumURL: "https://example.com/seid-%v-linux-amd64.sha256",
			},
		},
	}

	os.Setenv("SEICTL_TEST", "1") // Enable test mode
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	manager, err := NewManager(config, logger)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
		os.Unsetenv("SEICTL_TEST")
	}

	return manager, tmpDir, cleanup
}
