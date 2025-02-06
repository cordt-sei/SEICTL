package state

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/your-org/seictl/pkg/types"
)

// Manager handles chain state operations
type Manager struct {
	config *types.Config
	logger zerolog.Logger
}

// NewManager creates a new state manager
func NewManager(cfg *types.Config, logger zerolog.Logger) (*Manager, error) {
	return &Manager{
		config: cfg,
		logger: logger,
	}, nil
}

// CreateSnapshot creates a chain state snapshot
func (m *Manager) CreateSnapshot(ctx context.Context, height int64) error {
	m.logger.Info().Int64("height", height).Msg("Creating snapshot")

	// Create snapshot directory
	snapshotDir := filepath.Join(m.config.Global.BackupDir, fmt.Sprintf("snapshot_%d", height))
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Backup validator state
	if err := m.backupValidatorState(snapshotDir); err != nil {
		return fmt.Errorf("failed to backup validator state: %w", err)
	}

	// Create data snapshot
	if err := m.createDataSnapshot(ctx, snapshotDir, height); err != nil {
		return fmt.Errorf("failed to create data snapshot: %w", err)
	}

	// Create WASM snapshot if exists
	wasmDir := filepath.Join(m.config.Global.HomeDir, "wasm")
	if _, err := os.Stat(wasmDir); err == nil {
		if err := m.createWasmSnapshot(ctx, snapshotDir); err != nil {
			return fmt.Errorf("failed to create wasm snapshot: %w", err)
		}
	}

	m.logger.Info().Str("path", snapshotDir).Msg("Snapshot created successfully")
	return nil
}

// RestoreSnapshot restores chain state from a snapshot
func (m *Manager) RestoreSnapshot(ctx context.Context, snapshotPath string) error {
	m.logger.Info().Str("path", snapshotPath).Msg("Restoring from snapshot")

	// Verify snapshot
	if err := m.verifySnapshot(snapshotPath); err != nil {
		return fmt.Errorf("snapshot verification failed: %w", err)
	}

	// Stop node if running
	if err := m.stopNode(ctx); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	// Backup current state
	if err := m.backupCurrentState(); err != nil {
		return fmt.Errorf("failed to backup current state: %w", err)
	}

	// Restore data
	if err := m.restoreData(ctx, snapshotPath); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}

	// Restore WASM if exists
	wasmSnapshot := filepath.Join(snapshotPath, "wasm.tar.gz")
	if _, err := os.Stat(wasmSnapshot); err == nil {
		if err := m.restoreWasm(ctx, wasmSnapshot); err != nil {
			return fmt.Errorf("failed to restore wasm: %w", err)
		}
	}

	m.logger.Info().Msg("Snapshot restored successfully")
	return nil
}

// SyncState performs state synchronization
func (m *Manager) SyncState(ctx context.Context, targetHeight int64) error {
	m.logger.Info().Int64("target_height", targetHeight).Msg("Starting state sync")

	// Configure state sync
	if err := m.configureStateSync(targetHeight); err != nil {
		return fmt.Errorf("failed to configure state sync: %w", err)
	}

	// Start sync process
	if err := m.startStateSync(ctx); err != nil {
		return fmt.Errorf("state sync failed: %w", err)
	}

	return nil
}

func (m *Manager) startStateSync(ctx context.Context) error {
	m.logger.Info().Msg("Starting state sync process")

	// Start the node in state sync mode
	cmd := exec.CommandContext(ctx, "seid", "start", "--state-sync")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the node
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	// Monitor state sync in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := m.MonitorStateSync(ctx); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for either context cancellation or monitoring completion
	select {
	case <-ctx.Done():
		return fmt.Errorf("state sync cancelled: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("state sync monitoring failed: %w", err)
		}
	}

	return cmd.Wait()
}

func (m *Manager) backupValidatorState(snapshotDir string) error {
	valStateFile := filepath.Join(m.config.Global.HomeDir, "data", "priv_validator_state.json")
	backupPath := filepath.Join(snapshotDir, "priv_validator_state.json")

	if err := copyFile(valStateFile, backupPath); err != nil {
		return fmt.Errorf("failed to backup validator state: %w", err)
	}

	return nil
}

func (m *Manager) createDataSnapshot(ctx context.Context, snapshotDir string, height int64) error {
	dataDir := filepath.Join(m.config.Global.HomeDir, "data")
	outFile := filepath.Join(snapshotDir, "data.tar.gz")

	cmd := exec.CommandContext(ctx, "tar", "-czf", outFile, "-C", dataDir, ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create data snapshot: %w", err)
	}

	return nil
}

func (m *Manager) createWasmSnapshot(ctx context.Context, snapshotDir string) error {
	wasmDir := filepath.Join(m.config.Global.HomeDir, "wasm")
	outFile := filepath.Join(snapshotDir, "wasm.tar.gz")

	cmd := exec.CommandContext(ctx, "tar", "-czf", outFile, "-C", wasmDir, ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create wasm snapshot: %w", err)
	}

	return nil
}

func (m *Manager) verifySnapshot(snapshotPath string) error {
	required := []string{
		filepath.Join(snapshotPath, "data.tar.gz"),
		filepath.Join(snapshotPath, "priv_validator_state.json"),
	}

	for _, file := range required {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("required file missing: %s", file)
		}
	}

	return nil
}

func (m *Manager) stopNode(ctx context.Context) error {
	m.logger.Info().Msg("Stopping node")

	// First try graceful shutdown
	cmd := exec.CommandContext(ctx, "pkill", "-TERM", "seid")
	if err := cmd.Run(); err != nil {
		m.logger.Warn().Err(err).Msg("Graceful shutdown failed, forcing stop")
		// Force kill if graceful shutdown fails
		cmd = exec.CommandContext(ctx, "pkill", "-KILL", "seid")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stop node: %w", err)
		}
	}

	// Wait for process to fully stop
	time.Sleep(5 * time.Second)

	// Verify process is stopped
	if m.isNodeRunning() {
		return fmt.Errorf("node process still running after stop attempt")
	}

	return nil
}

func (m *Manager) isNodeRunning() bool {
	cmd := exec.Command("pgrep", "seid")
	return cmd.Run() == nil
}
func (m *Manager) backupCurrentState() error {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := filepath.Join(m.config.Global.BackupDir, fmt.Sprintf("backup_%s", timestamp))

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup current data
	dataDir := filepath.Join(m.config.Global.HomeDir, "data")
	if err := copyDir(dataDir, filepath.Join(backupDir, "data")); err != nil {
		return fmt.Errorf("failed to backup data: %w", err)
	}

	// Backup WASM if exists
	wasmDir := filepath.Join(m.config.Global.HomeDir, "wasm")
	if _, err := os.Stat(wasmDir); err == nil {
		if err := copyDir(wasmDir, filepath.Join(backupDir, "wasm")); err != nil {
			return fmt.Errorf("failed to backup wasm: %w", err)
		}
	}

	return nil
}

func (m *Manager) restoreData(ctx context.Context, snapshotPath string) error {
	dataFile := filepath.Join(snapshotPath, "data.tar.gz")
	dataDir := filepath.Join(m.config.Global.HomeDir, "data")

	// Clear existing data
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("failed to clear existing data: %w", err)
	}

	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Extract data
	cmd := exec.CommandContext(ctx, "tar", "-xzf", dataFile, "-C", dataDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract data: %w", err)
	}

	return nil
}

func (m *Manager) restoreWasm(ctx context.Context, wasmFile string) error {
	wasmDir := filepath.Join(m.config.Global.HomeDir, "wasm")

	// Clear existing WASM
	if err := os.RemoveAll(wasmDir); err != nil {
		return fmt.Errorf("failed to clear existing wasm: %w", err)
	}

	// Create WASM directory
	if err := os.MkdirAll(wasmDir, 0755); err != nil {
		return fmt.Errorf("failed to create wasm directory: %w", err)
	}

	// Extract WASM
	cmd := exec.CommandContext(ctx, "tar", "-xzf", wasmFile, "-C", wasmDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract wasm: %w", err)
	}

	return nil
}

func (m *Manager) configureStateSync(targetHeight int64) error {
	m.logger.Info().Int64("target_height", targetHeight).Msg("Configuring state sync")

	// Fetch trust block
	block, err := m.fetchTrustBlock(targetHeight)
	if err != nil {
		return fmt.Errorf("failed to fetch trust block: %w", err)
	}

	// Update config with trust block info
	for _, env := range m.config.Environments {
		if len(env.RPCEndpoints) > 0 {
			return m.setupStateSync(context.Background(), env.RPCEndpoints[0], block.Height, block.Hash)
		}
	}

	return fmt.Errorf("no RPC endpoints configured")
}

type Block struct {
	Height int64
	Hash   string
}

func (m *Manager) fetchTrustBlock(height int64) (*Block, error) {
	// Try automatic fetch first
	block, err := m.fetchTrustBlockAutomatic(height)
	if err == nil {
		return block, nil
	}

	// If automatic fetch fails, try interactive mode
	m.logger.Info().Msg("Automatic trust block fetch failed, switching to interactive mode")
	return m.fetchTrustBlockInteractive()
}

func (m *Manager) fetchTrustBlockAutomatic(height int64) (*Block, error) {
	var rpcEndpoints []string
	for _, env := range m.config.Environments {
		if len(env.RPCEndpoints) > 0 {
			rpcEndpoints = append(rpcEndpoints, env.RPCEndpoints...)
		}
	}

	if len(rpcEndpoints) == 0 {
		return nil, fmt.Errorf("no RPC endpoints configured")
	}

	for _, endpoint := range rpcEndpoints {
		block, err := m.queryBlockFromRPC(endpoint, height)
		if err == nil {
			m.logger.Info().
				Str("endpoint", endpoint).
				Int64("height", block.Height).
				Str("hash", block.Hash).
				Msg("Successfully fetched trust block")
			return block, nil
		}
		m.logger.Warn().Str("endpoint", endpoint).Err(err).Msg("Failed to fetch block from RPC")
	}

	return nil, fmt.Errorf("failed to fetch trust block from any configured RPC endpoint")
}

func (m *Manager) fetchTrustBlockInteractive() (*Block, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nPlease provide trust block information:")

	// Get height
	fmt.Print("Enter block height: ")
	heightStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read height: %w", err)
	}
	heightStr = strings.TrimSpace(heightStr)
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid height input: %w", err)
	}

	// Get hash
	fmt.Print("Enter block hash: ")
	hash, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read hash: %w", err)
	}
	hash = strings.TrimSpace(hash)

	// Validate hash format
	if !isValidHash(hash) {
		return nil, fmt.Errorf("invalid hash format: should be a 64-character hex string")
	}

	block := &Block{
		Height: height,
		Hash:   hash,
	}

	m.logger.Info().
		Int64("height", block.Height).
		Str("hash", block.Hash).
		Msg("Trust block set manually")

	return block, nil
}

func (m *Manager) queryBlockFromRPC(endpoint string, height int64) (*Block, error) {
	// Ensure endpoint has proper scheme
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}

	// Add proper path if not a full URL
	if !strings.Contains(endpoint, "/block") {
		endpoint = strings.TrimSuffix(endpoint, "/") + "/block"
	}

	// Add height parameter
	url := fmt.Sprintf("%s?height=%d", endpoint, height)

	// Make request with timeout
	client := &http.Client{
		Timeout: time.Duration(m.config.Global.TimeoutSeconds) * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query RPC endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC request failed with status: %s", resp.Status)
	}

	var result struct {
		Result struct {
			BlockID struct {
				Hash string `json:"hash"`
			} `json:"block_id"`
			Block struct {
				Header struct {
					Height string `json:"height"`
				} `json:"header"`
			} `json:"block"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode RPC response: %w", err)
	}

	blockHeight, err := strconv.ParseInt(result.Result.Block.Header.Height, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block height: %w", err)
	}

	return &Block{
		Height: blockHeight,
		Hash:   result.Result.BlockID.Hash,
	}, nil
}

func isValidHash(hash string) bool {
	// Remove "0x" prefix if present
	hash = strings.TrimPrefix(hash, "0x")

	// Check length (32 bytes = 64 chars in hex)
	if len(hash) != 64 {
		return false
	}

	// Check if it's a valid hex string
	_, err := hex.DecodeString(hash)
	return err == nil
}

// Helper functions

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0644)
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			if err := copyDir(sourcePath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(sourcePath, destPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) setupStateSync(ctx context.Context, rpcEndpoint string, trustHeight int64, trustHash string) error {
	m.logger.Info().
		Str("rpc", rpcEndpoint).
		Int64("height", trustHeight).
		Msg("Setting up state sync")

	configPath := filepath.Join(m.config.Global.HomeDir, "config", "config.toml")

	// Read current config
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Update state sync configuration
	updates := map[string]string{
		"statesync.enable":       "true",
		"statesync.rpc_servers":  fmt.Sprintf("\"%s,%s\"", rpcEndpoint, rpcEndpoint),
		"statesync.trust_height": fmt.Sprintf("%d", trustHeight),
		"statesync.trust_hash":   fmt.Sprintf("\"%s\"", trustHash),
	}

	newContent := string(content)
	for key, value := range updates {
		newContent = updateConfig(newContent, key, value)
	}

	// Write updated config
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (m *Manager) MonitorStateSync(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := m.getNodeStatus(ctx)
			if err != nil {
				m.logger.Error().Err(err).Msg("Failed to get node status")
				continue
			}

			if status.Syncing {
				m.logger.Info().
					Int64("current_height", status.LatestHeight).
					Msg("State sync in progress")
			} else {
				m.logger.Info().Msg("State sync completed")
				return nil
			}
		}
	}
}

type NodeStatus struct {
	Syncing      bool
	LatestHeight int64
}

func (m *Manager) getNodeStatus(ctx context.Context) (*NodeStatus, error) {
	// Get RPC endpoint from config
	var rpcEndpoint string
	for _, env := range m.config.Environments {
		if len(env.RPCEndpoints) > 0 {
			rpcEndpoint = env.RPCEndpoints[0]
			break
		}
	}

	if rpcEndpoint == "" {
		return nil, fmt.Errorf("no RPC endpoint configured")
	}

	// Ensure endpoint has proper scheme
	if !strings.HasPrefix(rpcEndpoint, "http") {
		rpcEndpoint = "http://" + rpcEndpoint
	}

	// Add status endpoint
	rpcEndpoint = strings.TrimSuffix(rpcEndpoint, "/") + "/status"

	// Make request with timeout
	client := &http.Client{
		Timeout: time.Duration(m.config.Global.TimeoutSeconds) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rpcEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query node status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status request failed: %s: %s", resp.Status, string(body))
	}

	var result struct {
		Result struct {
			SyncInfo struct {
				CatchingUp        bool   `json:"catching_up"`
				LatestBlockHeight string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	height, err := strconv.ParseInt(result.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block height: %w", err)
	}

	return &NodeStatus{
		Syncing:      result.Result.SyncInfo.CatchingUp,
		LatestHeight: height,
	}, nil
}

func updateConfig(content, key, value string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+" =") {
			lines[i] = fmt.Sprintf("%s = %s", key, value)
			break
		}
	}
	return strings.Join(lines, "\n")
}

// UpdatePruning updates the pruning configuration
func (m *Manager) UpdatePruning(ctx context.Context, keepRecent, keepEvery, interval int64) error {
	m.logger.Info().
		Int64("keep_recent", keepRecent).
		Int64("keep_every", keepEvery).
		Int64("interval", interval).
		Msg("Updating pruning configuration")

	configPath := filepath.Join(m.config.Global.HomeDir, "config", "app.toml")

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	updates := map[string]string{
		"pruning":             "\"custom\"",
		"pruning-keep-recent": fmt.Sprintf("%d", keepRecent),
		"pruning-keep-every":  fmt.Sprintf("%d", keepEvery),
		"pruning-interval":    fmt.Sprintf("%d", interval),
	}

	newContent := string(content)
	for key, value := range updates {
		newContent = updateConfig(newContent, key, value)
	}

	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SetupTmpfs sets up a tmpfs mount for improved performance
func (m *Manager) SetupTmpfs(ctx context.Context, size string) error {
	m.logger.Info().Str("size", size).Msg("Setting up tmpfs")

	cmd := exec.CommandContext(ctx, "sudo", "mount", "-t", "tmpfs",
		"-o", fmt.Sprintf("size=%s,mode=1777", size), "overflow", "/tmp")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to setup tmpfs: %w", err)
	}

	return nil
}
