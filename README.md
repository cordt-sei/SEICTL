# SEICTL - Sei Node Management Tool

SEICTL is a comprehensive node management tool for the Sei blockchain network, providing a unified interface for node initialization, state management, snapshots, and maintenance operations.

## Features

- Node initialization for local, testnet, and mainnet environments
- Automated binary management and verification
- State synchronization with configurable trust parameters
- Snapshot creation and restoration
- Pruning management
- Performance optimization with tmpfs support
- Comprehensive logging and monitoring
- Environment-specific configurations

## Installation

### Prerequisites

- Go 1.19 or later
- Required system packages: `tar`, `make`, `git`
- Sufficient disk space (recommended: 500GB+)
- RAM: 32GB recommended

### Building from Source

```bash
# Clone the repository
git clone https://github.com/your-org/seictl.git
cd seictl

# Build the binary
go build -o seictl cmd/seictl/main.go

# Optional: Install system-wide
sudo mv seictl /usr/local/bin/
```

## Configuration

SEICTL uses YAML configuration files for managing different environments and node settings. 

### Basic Configuration Structure

```yaml
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
      - "https://rpc2.sei.io"
    # Additional mainnet configuration...

  testnet:
    chain_id: "atlantic-2"
    version: "v5.9.0-hotfix"
    # Additional testnet configuration...

  local:
    chain_id: "sei-local"
    version: "latest"
    # Additional local configuration...
```

## Usage

### Basic Commands

1. Initialize a New Node
```bash
seictl init --env mainnet
```

2. Create a Snapshot
```bash
seictl snapshot --height 1000000
```

3. Restore from Snapshot
```bash
seictl snapshot restore --path /path/to/snapshot
```

4. Perform State Sync
```bash
seictl state-sync --rpc https://rpc.sei.io:443 --trust-height 1000000
```

5. Start Node
```bash
seictl start
```

### Environment-Specific Operations

#### Local Development
```bash
# Initialize local development environment
seictl init --env local

# Start local node with default genesis
seictl start
```

#### Testnet
```bash
# Initialize testnet node
seictl init --env testnet

# Perform state sync for faster catch-up
seictl state-sync --env testnet
```

#### Mainnet
```bash
# Initialize mainnet node
seictl init --env mainnet

# Create periodic snapshots
seictl snapshot --interval 100000
```

## Advanced Features

### Pruning Configuration

Configure chain state pruning:
```bash
seictl config pruning --keep-recent 100 --keep-every 500 --interval 10
```

### Performance Optimization

Setup tmpfs for improved performance:
```bash
seictl optimize --tmpfs-size 12G
```

### Monitoring

Monitor node status:
```bash
seictl status
```

## Maintenance Operations

### Binary Management

Update node binary:
```bash
seictl binary update --version v5.9.0-hotfix
```

Verify binary checksum:
```bash
seictl binary verify
```

### Backup and Restore

Create full node backup:
```bash
seictl backup create
```

Restore from backup:
```bash
seictl backup restore --path /path/to/backup
```

## Troubleshooting

### Common Issues

1. Binary Verification Failures
   - Verify network connectivity
   - Check disk space
   - Ensure correct permissions

2. State Sync Issues
   - Verify RPC endpoint availability
   - Check trust height is within available range
   - Ensure sufficient disk space

3. Snapshot Creation Failures
   - Verify sufficient disk space
   - Check node is not actively syncing
   - Ensure proper permissions

### Logs

View node logs:
```bash
seictl logs --tail 100
```

## Best Practices

1. Regular Snapshots
   - Create snapshots at regular intervals
   - Maintain multiple snapshot copies
   - Verify snapshots after creation

2. State Management
   - Regular pruning configuration review
   - Monitor disk usage
   - Regular backup verification

3. Performance Optimization
   - Use tmpfs for improved performance
   - Monitor system resources
   - Regular performance tuning

## Security Considerations

1. Binary Verification
   - Always verify binary checksums
   - Use trusted RPC endpoints
   - Regular security updates

2. Access Control
   - Proper file permissions
   - Secure key management
   - Regular security audits

## Contributing

Contributions are welcome! Please see our contributing guidelines for more information.

## License

This project is licensed under the GNU GPL v3 License - see the LICENSE file for details.
