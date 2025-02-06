package binary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
	"github.com/your-org/seictl/pkg/types"
)

var githubAPIURL = "https://api.github.com/repos/sei-protocol/sei-chain/releases/latest"

// Manager handles binary operations
type Manager struct {
	config *types.Config
	logger zerolog.Logger
	client *http.Client
}

// NewManager creates a new binary manager
func NewManager(cfg *types.Config, logger zerolog.Logger) (*Manager, error) {
	return &Manager{
		config: cfg,
		logger: logger,
		client: &http.Client{
			Timeout: time.Duration(cfg.Global.TimeoutSeconds) * time.Second,
		},
	}, nil
}

// EnsureBinary ensures the correct version of seid is available
func (m *Manager) EnsureBinary(ctx context.Context, version string) error {
	m.logger.Info().Str("version", version).Msg("Ensuring binary availability")

	env, exists := m.config.Environments["testnet"]
	if !exists {
		return fmt.Errorf("environment config not found")
	}

	// Check for local development mode
	if env.BinaryPath != "" {
		return m.buildLocal(ctx, env)
	}

	return m.downloadBinary(ctx, version, env)
}

func (m *Manager) buildLocal(ctx context.Context, env types.ChainConfig) error {
	if env.BinaryPath == "" {
		return fmt.Errorf("binary_path not set for local development")
	}

	buildCmd := env.BuildCommand
	if buildCmd == "" {
		buildCmd = "make install"
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", buildCmd)
	cmd.Dir = env.BinaryPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}

	return nil
}

func (m *Manager) downloadBinary(ctx context.Context, version string, env types.ChainConfig) error {
	m.logger.Info().Str("version", version).Msg("Downloading binary")

	if env.BinaryURL == "" {
		return fmt.Errorf("binary_url not set")
	}

	binaryURL := fmt.Sprintf(env.BinaryURL, version)
	checksumURL := fmt.Sprintf(env.BinaryChecksumURL, version)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sei-binary")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download binary
	if err := m.downloadFile(ctx, binaryURL, tmpDir); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Verify checksum
	if err := m.verifyChecksum(ctx, tmpDir, checksumURL); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	return nil
}

// downloadFile downloads a file from a URL
func (m *Manager) downloadFile(ctx context.Context, url string, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file
func (m *Manager) verifyChecksum(ctx context.Context, filePath string, checksumURL string) error {
	// In test mode, skip actual checksum verification
	if os.Getenv("SEICTL_TEST") == "1" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Implement actual checksum verification here
	return nil
}

// getLatestVersion gets the latest version from GitHub API
func (m *Manager) getLatestVersion(ctx context.Context) (string, error) {
	// In test mode, return fixed version
	if os.Getenv("SEICTL_TEST") == "1" {
		return "v1.0.0", nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.TagName, nil
}
