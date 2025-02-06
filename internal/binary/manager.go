package binary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/your-org/seictl/pkg/types"

	"github.com/rs/zerolog"
)

// Manager handles Sei binary operations
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
			Timeout: cfg.Global.GetTimeout(),
		},
	}, nil
}

// EnsureBinary ensures the correct version of seid is available
func (m *Manager) EnsureBinary(ctx context.Context, version string) error {
	if version == "latest" {
		var err error
		version, err = m.getLatestVersion(ctx)
		if err != nil {
			return fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	binPath := "/usr/local/bin/seid"
	if m.isBinaryInstalled(binPath, version) {
		m.logger.Info().Str("version", version).Msg("Binary already installed")
		return nil
	}

	return m.downloadAndInstall(ctx, version, binPath)
}

// CompileAndInstall compiles and installs Sei from source
func (m *Manager) CompileAndInstall(ctx context.Context, version string) error {
	m.logger.Info().Str("version", version).Msg("Compiling from source")

	// Clone repository
	cmd := exec.CommandContext(ctx, "git", "clone", "https://github.com/sei-protocol/sei-chain.git")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout version
	cmd = exec.CommandContext(ctx, "git", "checkout", version)
	cmd.Dir = "sei-chain"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout version: %w", err)
	}

	// Build
	cmd = exec.CommandContext(ctx, "make", "install")
	cmd.Dir = "sei-chain"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	return nil
}

func (m *Manager) isBinaryInstalled(binPath, version string) bool {
	cmd := exec.Command(binPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), version)
}

func (m *Manager) downloadAndInstall(ctx context.Context, version, binPath string) error {
	m.logger.Info().Str("version", version).Msg("Downloading binary")

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sei-binary")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download binary
	binaryURL := fmt.Sprintf(m.config.Environments["mainnet"].BinaryURL, version)
	tmpBinPath := filepath.Join(tmpDir, "seid")
	if err := m.downloadFile(ctx, binaryURL, tmpBinPath); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Verify checksum
	checksumURL := fmt.Sprintf(m.config.Environments["mainnet"].BinaryChecksumURL, version)
	if err := m.verifyChecksum(ctx, tmpBinPath, checksumURL); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpBinPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Move to final location
	if err := os.Rename(tmpBinPath, binPath); err != nil {
		return fmt.Errorf("failed to move binary to final location: %w", err)
	}

	m.logger.Info().Str("version", version).Msg("Binary installed successfully")
	return nil
}

func (m *Manager) downloadFile(ctx context.Context, url, dest string) error {
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

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (m *Manager) verifyChecksum(ctx context.Context, binPath, checksumURL string) error {
	// Download checksum file
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

	checksumBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksum: %w", err)
	}

	expectedChecksum := strings.Split(string(checksumBytes), " ")[0]

	// Calculate file checksum
	f, err := os.Open(binPath)
	if err != nil {
		return fmt.Errorf("failed to open binary: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(h.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

func (m *Manager) getLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/sei-protocol/sei-chain/releases/latest", nil)
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

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return release.TagName, nil
}
