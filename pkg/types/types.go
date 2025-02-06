package types

import (
	"time"
)

// Environment represents the chain environment type
type Environment string

const (
	Local   Environment = "local"
	Testnet Environment = "testnet"
	Mainnet Environment = "mainnet"
)

// Config represents the main configuration structure
type Config struct {
	Version      string                 `yaml:"version"`
	Global       GlobalConfig           `yaml:"global"`
	Environments map[string]ChainConfig `yaml:"environments"`
	NodeConfigs  NodeConfigs            `yaml:"node_configs"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
	HomeDir        string `yaml:"home_dir"`
	BackupDir      string `yaml:"backup_dir"`
	LogLevel       string `yaml:"log_level"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	MaxRetries     int    `yaml:"max_retries"`
	RetryDelay     string `yaml:"retry_delay_seconds"`
}

// GetRetryDelay returns the retry delay as time.Duration
func (g GlobalConfig) GetRetryDelay() time.Duration {
	d, err := time.ParseDuration(g.RetryDelay + "s")
	if err != nil {
		return 5 * time.Second // default to 5 seconds if parsing fails
	}
	return d
}

// GetTimeout returns the timeout as time.Duration
func (g GlobalConfig) GetTimeout() time.Duration {
	return time.Duration(g.TimeoutSeconds) * time.Second
}

// ChainConfig contains chain-specific configuration
type ChainConfig struct {
	ChainID           string   `yaml:"chain_id"`
	Version           string   `yaml:"version"`
	RPCEndpoints      []string `yaml:"rpc_endpoints,omitempty"`
	GenesisURL        string   `yaml:"genesis_url,omitempty"`
	BinaryURL         string   `yaml:"binary_url,omitempty"`
	BinaryChecksumURL string   `yaml:"binary_checksum_url,omitempty"`
	// Local development options
	BinaryPath      string           `yaml:"binary_path,omitempty"`
	BuildCommand    string           `yaml:"build_command,omitempty"`
	StateSync       *StateSyncConfig `yaml:"state_sync,omitempty"`
	Ports           *NodePorts       `yaml:"ports,omitempty"`
	GenesisAccounts []Account        `yaml:"genesis_accounts,omitempty"`
	GenesisParams   GenesisParams    `yaml:"genesis_params,omitempty"`
}

// StateSyncConfig contains state sync specific configuration
type StateSyncConfig struct {
	TrustHeightDelta int64 `yaml:"trust_height_delta"`
	BlockTimeSeconds int   `yaml:"block_time_seconds"`
	SnapshotInterval int64 `yaml:"snapshot_interval"`
}

// NodePorts contains port configuration
type NodePorts struct {
	RPC     int `yaml:"rpc"`
	P2P     int `yaml:"p2p"`
	API     int `yaml:"api"`
	GRPC    int `yaml:"grpc"`
	GRPCWeb int `yaml:"grpc_web"`
	PProf   int `yaml:"pprof"`
}

// Account represents a genesis account
type Account struct {
	Name  string   `yaml:"name"`
	Coins []string `yaml:"coins"`
}

// GenesisParams contains genesis parameters
type GenesisParams struct {
	VotingPeriod          string `yaml:"voting_period"`
	ExpeditedVotingPeriod string `yaml:"expedited_voting_period"`
	DepositPeriod         string `yaml:"deposit_period"`
	OracleVotePeriod      string `yaml:"oracle_vote_period"`
	CommunityTax          string `yaml:"community_tax"`
	BlockMaxGas           string `yaml:"block_max_gas"`
	MaxVotingPowerRatio   string `yaml:"max_voting_power_ratio"`
}

// NodeConfigs contains node configuration templates
type NodeConfigs struct {
	AppToml    map[string]interface{} `yaml:"app_toml"`
	ConfigToml map[string]interface{} `yaml:"config_toml"`
}
