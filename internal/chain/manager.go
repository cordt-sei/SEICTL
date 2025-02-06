package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/your-org/seictl/internal/binary"
	"github.com/your-org/seictl/internal/state"
	"github.com/your-org/seictl/pkg/types"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// InitOptions defines options for chain initialization
type InitOptions struct {
	SkipBinary    bool
	Moniker       string
	ChainID       string
	WithStateSync bool
}

// Manager handles chain operations
type Manager struct {
	config     *types.Config
	binMgr     *binary.Manager
	stateMgr   *state.Manager
	logger     zerolog.Logger
	homePath   string
	configPath string
	mu         sync.RWMutex
}

// NewManager creates a new chain manager
func NewManager(cfg *types.Config, logger zerolog.Logger) (*Manager, error) {
	homePath := os.ExpandEnv(cfg.Global.HomeDir)
	binMgr, err := binary.NewManager(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary manager: %w", err)
	}

	stateMgr, err := state.NewManager(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	return &Manager{
		config:     cfg,
		binMgr:     binMgr,
		stateMgr:   stateMgr,
		logger:     logger,
		homePath:   homePath,
		configPath: filepath.Join(homePath, "config"),
	}, nil
}

// InitChain initializes a new chain
func (m *Manager) InitChain(ctx context.Context, env types.Environment, opts InitOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chainCfg, ok := m.config.Environments[string(env)]
	if !ok {
		return fmt.Errorf("environment %s not found in configuration", env)
	}

	// Override chain ID if provided in options
	if opts.ChainID != "" {
		chainCfg.ChainID = opts.ChainID
	}

	// Only handle binary if not skipped
	if !opts.SkipBinary {
		if err := m.binMgr.EnsureBinary(ctx, chainCfg.Version); err != nil {
			return fmt.Errorf("failed to ensure binary: %w", err)
		}
	}

	// Initialize chain directory
	if err := m.initChainDir(chainCfg); err != nil {
		return fmt.Errorf("failed to initialize chain directory: %w", err)
	}

	// Configure node with options
	if err := m.configureNode(chainCfg, opts); err != nil {
		return fmt.Errorf("failed to configure node: %w", err)
	}

	// Handle genesis setup
	if err := m.setupGenesis(ctx, chainCfg); err != nil {
		return fmt.Errorf("failed to setup genesis: %w", err)
	}

	return nil
}

// CreateSnapshot creates a chain snapshot
func (m *Manager) CreateSnapshot(ctx context.Context, height int64) error {
	return m.stateMgr.CreateSnapshot(ctx, height)
}

// RestoreSnapshot restores from a snapshot
func (m *Manager) RestoreSnapshot(ctx context.Context, path string) error {
	return m.stateMgr.RestoreSnapshot(ctx, path)
}

// StateSync performs state synchronization
func (m *Manager) StateSync(ctx context.Context, targetHeight int64) error {
	return m.stateMgr.SyncState(ctx, targetHeight)
}

// StartNode starts the node
func (m *Manager) StartNode(ctx context.Context) error {
	m.logger.Info().Msg("Starting node...")

	cmd := exec.CommandContext(ctx, "seid", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// StopNode stops the node
func (m *Manager) StopNode(ctx context.Context) error {
	m.logger.Info().Msg("Stopping node...")

	// Send SIGTERM to the node process
	cmd := exec.CommandContext(ctx, "pkill", "-TERM", "seid")
	return cmd.Run()
}

func (m *Manager) initChainDir(cfg types.ChainConfig) error {
	// Use cfg parameter in implementation
	m.logger.Info().Str("chain_id", cfg.ChainID).Msg("Initializing chain directory")

	dirs := []string{
		m.homePath,
		m.configPath,
		filepath.Join(m.homePath, "data"),
		filepath.Join(m.homePath, "wasm"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (m *Manager) configureNode(cfg types.ChainConfig, opts InitOptions) error {
	// Use cfg parameter in implementation
	m.logger.Info().
		Str("chain_id", cfg.ChainID).
		Str("moniker", opts.Moniker).
		Msg("Configuring node")

	// Update node configs with options
	nodeConfigs := m.config.NodeConfigs

	// Create deep copies of the configs to modify
	configToml := make(map[string]interface{})
	for k, v := range nodeConfigs.ConfigToml {
		configToml[k] = v
	}

	// Set chain-specific configurations
	configToml["chain_id"] = cfg.ChainID
	if opts.Moniker != "" {
		configToml["moniker"] = opts.Moniker
	}

	// Enable state sync if requested
	if opts.WithStateSync {
		statesync, ok := configToml["statesync"].(map[string]interface{})
		if !ok {
			statesync = make(map[string]interface{})
			configToml["statesync"] = statesync
		}
		statesync["enable"] = true
	}

	// Write configs
	if err := m.writeConfig("app.toml", nodeConfigs.AppToml); err != nil {
		return fmt.Errorf("failed to write app.toml: %w", err)
	}

	if err := m.writeConfig("config.toml", configToml); err != nil {
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	return nil
}

func (m *Manager) writeConfig(filename string, data interface{}) error {
	path := filepath.Join(m.configPath, filename)
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

func (m *Manager) setupGenesis(ctx context.Context, cfg types.ChainConfig) error {
	genesisPath := filepath.Join(m.configPath, "genesis.json")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Download genesis if URL provided
		if cfg.GenesisURL != "" {
			if err := m.downloadGenesis(ctx, cfg.GenesisURL, genesisPath); err != nil {
				return fmt.Errorf("failed to download genesis: %w", err)
			}
		} else if len(cfg.GenesisAccounts) > 0 {
			if err := m.createLocalGenesis(cfg, genesisPath); err != nil {
				return fmt.Errorf("failed to create local genesis: %w", err)
			}
		}
		return nil
	}
}

func (m *Manager) downloadGenesis(_ context.Context, url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download genesis: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create genesis file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write genesis file: %w", err)
	}

	return nil
}

func (m *Manager) createLocalGenesis(cfg types.ChainConfig, genesisPath string) error {
	// Use cfg parameter to create local genesis file
	genesis := map[string]interface{}{
		"chain_id":     cfg.ChainID,
		"genesis_time": time.Now().Format(time.RFC3339),
		"accounts":     cfg.GenesisAccounts,
	}

	bytes, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write genesis file: %w", err)
	}

	return nil
}
