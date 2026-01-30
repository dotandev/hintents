# Custom Network Support

## Overview

Erst now supports debugging transactions on private Stellar networks, local development environments, and custom network configurations. This is essential for enterprises running private constellations or developers testing on local networks.

## Features

- ✅ Connect to private/local Stellar networks
- ✅ Secure storage of network configurations with encrypted passphrases
- ✅ Support for custom Horizon and Soroban RPC endpoints
- ✅ Manage multiple custom network profiles
- ✅ File permissions set to 0600 for security

## Usage

### Quick Start with Custom Network

```bash
# Debug a transaction on a custom network (one-time use)
erst debug <tx-hash> \
  --custom-network \
  --horizon-url http://localhost:8000 \
  --network-passphrase "Local Development Network" \
  --soroban-rpc http://localhost:8001
```

### Save a Custom Network Profile

```bash
# Add a custom network configuration
erst network add local-dev \
  --horizon-url http://localhost:8000 \
  --network-passphrase "Local Development Network" \
  --soroban-rpc http://localhost:8001

# Use the saved profile
erst debug <tx-hash> --network local-dev
```

### Manage Custom Networks

```bash
# List all saved custom networks
erst network list

# Show details of a specific network
erst network show local-dev

# Remove a custom network
erst network remove local-dev
```

## Configuration File

Custom networks are stored in `~/.erst/networks.json` with restricted permissions (0600):

```json
{
  "networks": {
    "local-dev": {
      "name": "local-dev",
      "horizon_url": "http://localhost:8000",
      "network_passphrase": "Local Development Network",
      "soroban_rpc_url": "http://localhost:8001"
    },
    "staging": {
      "name": "staging",
      "horizon_url": "https://horizon-staging.example.com",
      "network_passphrase": "Staging Network ; January 2025",
      "soroban_rpc_url": "https://soroban-staging.example.com"
    }
  }
}
```

## Common Use Cases

### Local Soroban Development

```bash
# Start local soroban network
stellar network start local

# Add the local network to erst
erst network add local \
  --horizon-url http://localhost:8000 \
  --network-passphrase "Standalone Network ; February 2017" \
  --soroban-rpc http://localhost:8000/soroban/rpc

# Debug transactions on local network
erst debug <tx-hash> --network local
```

### Enterprise Private Network

```bash
# Add your private constellation
erst network add private-prod \
  --horizon-url https://horizon.internal.company.com \
  --network-passphrase "Company Private Network ; 2025" \
  --soroban-rpc https://soroban.internal.company.com

# Debug on private network
erst debug <tx-hash> --network private-prod
```

### Staging Environment

```bash
# Add staging network
erst network add staging \
  --horizon-url https://horizon-staging.example.com \
  --network-passphrase "Staging Network" \
  --soroban-rpc https://soroban-staging.example.com

# Debug on staging
erst debug <tx-hash> --network staging
```

## Security

### Network Passphrase Protection

- Network passphrases are stored in `~/.erst/networks.json`
- File permissions are automatically set to `0600` (owner read/write only)
- Config directory permissions are set to `0700` (owner access only)
- Sensitive data is never logged or displayed in error messages

### Best Practices

1. **Never commit** `~/.erst/networks.json` to version control
2. **Use environment variables** for CI/CD environments:
   ```bash
   export ERST_HORIZON_URL="http://localhost:8000"
   export ERST_NETWORK_PASSPHRASE="Local Network"
   erst debug <tx-hash> --custom-network
   ```
3. **Rotate passphrases** regularly for production networks
4. **Restrict access** to the `.erst` directory on shared systems

## API Usage

### Programmatic Access

```go
import (
    "github.com/dotandev/hintents/internal/config"
    "github.com/dotandev/hintents/internal/rpc"
)

// Create a custom network client
customConfig := rpc.NetworkConfig{
    Name:              "my-network",
    HorizonURL:        "http://localhost:8000",
    NetworkPassphrase: "My Custom Network",
    SorobanRPCURL:     "http://localhost:8001",
}

client, err := rpc.NewCustomClient(customConfig)
if err != nil {
    log.Fatal(err)
}

// Use the client
tx, err := client.GetTransaction(ctx, txHash)
```

### Save and Load Networks

```go
// Save a custom network
err := config.AddCustomNetwork("local-dev", customConfig)

// Load a custom network
networkConfig, err := config.GetCustomNetwork("local-dev")

// Create client from saved config
client, err := rpc.NewCustomClient(*networkConfig)
```

## Troubleshooting

### Connection Issues

```bash
# Test connection to custom network
curl http://localhost:8000/

# Verify Horizon is accessible
curl http://localhost:8000/transactions/<tx-hash>

# Check Soroban RPC
curl -X POST http://localhost:8001 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"getHealth"}'
```

### Permission Errors

```bash
# Fix config file permissions
chmod 600 ~/.erst/networks.json
chmod 700 ~/.erst
```

### Invalid Passphrase

Ensure your network passphrase matches exactly:
- Mainnet: `Public Global Stellar Network ; September 2015`
- Testnet: `Test SDF Network ; September 2015`
- Local: Check your network configuration

## Examples

### Complete Workflow

```bash
# 1. Start local network
stellar network start local

# 2. Deploy a contract
stellar contract deploy --wasm contract.wasm --network local

# 3. Invoke the contract (this might fail)
stellar contract invoke --id <contract-id> --network local -- my_function

# 4. Add local network to erst
erst network add local \
  --horizon-url http://localhost:8000 \
  --network-passphrase "Standalone Network ; February 2017" \
  --soroban-rpc http://localhost:8000/soroban/rpc

# 5. Debug the failed transaction
erst debug <tx-hash> --network local
```

## Future Enhancements

- [ ] Support for network passphrase encryption
- [ ] Auto-detect local Stellar network
- [ ] Network health checks before debugging
- [ ] Import/export network configurations
- [ ] Support for multiple Horizon endpoints (load balancing)

## References

- [Stellar Networks](https://developers.stellar.org/docs/networks)
- [Soroban Local Development](https://developers.stellar.org/docs/smart-contracts/getting-started/setup)
- [Network Passphrases](https://developers.stellar.org/docs/encyclopedia/network-passphrases)
