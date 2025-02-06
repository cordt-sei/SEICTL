package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/seictl/internal/chain"
	"github.com/your-org/seictl/pkg/types"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile string
	config  *types.Config
	logger  zerolog.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "seictl",
		Short: "Sei node management tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file")

	// Initialize commands
	rootCmd.AddCommand(
		newInitCmd(),
		newSnapshotCmd(),
		newStateSyncCmd(),
		newStartCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initConfig() error {
	// Setup logger
	logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Read config file
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	config = &types.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

func newInitCmd() *cobra.Command {
	var env string
	var skipBinary bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Sei node",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := setupContext()

			mgr, err := chain.NewManager(config, logger)
			if err != nil {
				return err
			}

			opts := chain.InitOptions{
				SkipBinary: skipBinary,
			}

			return mgr.InitChain(ctx, types.Environment(env), opts)
		},
	}

	cmd.Flags().StringVar(&env, "env", "", "environment (local, testnet, mainnet)")
	cmd.Flags().BoolVar(&skipBinary, "skip-binary", false, "skip binary download/compilation")

	if err := cmd.MarkFlagRequired("env"); err != nil {
		logger.Fatal().Err(err).Msg("Failed to mark env flag as required")
	}

	return cmd
}

func newSnapshotCmd() *cobra.Command {
	var height int64

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Create a chain snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := setupContext()

			mgr, err := chain.NewManager(config, logger)
			if err != nil {
				return err
			}

			return mgr.CreateSnapshot(ctx, height)
		},
	}

	cmd.Flags().Int64Var(&height, "height", 0, "block height for snapshot")

	return cmd
}

func newStateSyncCmd() *cobra.Command {
	var rpcEndpoint string
	var trustHeight int64

	cmd := &cobra.Command{
		Use:   "state-sync",
		Short: "Perform state synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := setupContext()

			mgr, err := chain.NewManager(config, logger)
			if err != nil {
				return err
			}

			return mgr.StateSync(ctx, trustHeight)
		},
	}

	cmd.Flags().StringVar(&rpcEndpoint, "rpc", "", "RPC endpoint for state sync")
	cmd.Flags().Int64Var(&trustHeight, "trust-height", 0, "trusted block height")

	if err := cmd.MarkFlagRequired("rpc"); err != nil {
		logger.Fatal().Err(err).Msg("Failed to mark rpc flag as required")
	}
	if err := cmd.MarkFlagRequired("trust-height"); err != nil {
		logger.Fatal().Err(err).Msg("Failed to mark trust-height flag as required")
	}

	return cmd
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Sei node",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := setupContext()

			mgr, err := chain.NewManager(config, logger)
			if err != nil {
				return err
			}

			return mgr.StartNode(ctx)
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("seictl version %s\n", config.Version)
		},
	}
}

func setupContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
		time.Sleep(time.Second) // Give operations a chance to cleanup
		os.Exit(0)
	}()

	return ctx
}
