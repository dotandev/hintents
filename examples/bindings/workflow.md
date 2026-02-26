# Complete Workflow: From Contract to TypeScript Client

This guide walks through the complete process of creating a Soroban smart contract and generating TypeScript bindings with erst integration.

## Step 1: Create Soroban Contract

```rust
// src/lib.rs
#![no_std]
use soroban_sdk::{contract, contractimpl, contracttype, Address, Env};

#[contracttype]
pub struct TokenInfo {
    pub name: String,
    pub symbol: String,
    pub decimals: u32,
}

#[contracttype]
pub enum TransferStatus {
    Success = 0,
    Failed = 1,
}

#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    /// Initialize the token with metadata
    pub fn initialize(env: Env, name: String, symbol: String, decimals: u32) {
        // Implementation
    }

    /// Transfer tokens from one address to another
    pub fn transfer(env: Env, from: Address, to: Address, amount: i128) -> TransferStatus {
        // Implementation
        TransferStatus::Success
    }

    /// Get token balance for an address
    pub fn balance(env: Env, address: Address) -> i128 {
        // Implementation
        0
    }

    /// Get token metadata
    pub fn get_info(env: Env) -> TokenInfo {
        // Implementation
        TokenInfo {
            name: String::from_str(&env, "MyToken"),
            symbol: String::from_str(&env, "MTK"),
            decimals: 7,
        }
    }
}
```

## Step 2: Build Contract

```bash
# Build the contract
soroban contract build

# Output: target/wasm32-unknown-unknown/release/token_contract.wasm
```

## Step 3: Generate TypeScript Bindings

```bash
# Generate bindings
erst generate-bindings \
  target/wasm32-unknown-unknown/release/token_contract.wasm \
  --output ./frontend/src/bindings \
  --package token-contract \
  --network testnet

# Output:
# Generated: frontend/src/bindings/types.ts
# Generated: frontend/src/bindings/client.ts
# Generated: frontend/src/bindings/erst-integration.ts
# Generated: frontend/src/bindings/index.ts
# Generated: frontend/src/bindings/package.json
# Generated: frontend/src/bindings/README.md
```

## Step 4: Install Dependencies

```bash
cd frontend/src/bindings
npm install
npm run build
```

## Step 5: Deploy Contract (Optional)

```bash
# Deploy to testnet
soroban contract deploy \
  --wasm target/wasm32-unknown-unknown/release/token_contract.wasm \
  --network testnet \
  --source ADMIN_SECRET_KEY

# Output: CONTRACT_ID
```

## Step 6: Use Generated Client

```typescript
// frontend/src/app.ts
import { TokenContractClient } from './bindings';
import * as StellarSdk from '@stellar/stellar-sdk';

async function main() {
  // Initialize client
  const client = new TokenContractClient({
    contractId: 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2',
    network: 'testnet',
    enableSimulation: true,
  });

  // Create keypair
  const sourceKeypair = StellarSdk.Keypair.fromSecret(
    process.env.SECRET_KEY!
  );

  // Initialize token (simulate first)
  console.log('Simulating initialize...');
  const initSim = await client.initialize(
    sourceKeypair,
    'MyToken',
    'MTK',
    7,
    { simulate: true }
  );

  console.log('Simulation result:', initSim.simulation?.status);
  console.log('CPU usage:', initSim.simulation?.budgetUsage?.cpuUsagePercent);

  // Execute if simulation succeeds
  if (initSim.simulation?.status === 'success') {
    console.log('Executing initialize...');
    const initResult = await client.initialize(
      sourceKeypair,
      'MyToken',
      'MTK',
      7
    );
    console.log('Transaction hash:', initResult.transactionHash);
  }

  // Transfer tokens
  console.log('Simulating transfer...');
  const transferSim = await client.transfer(
    sourceKeypair,
    sourceKeypair.publicKey(),
    'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37',
    BigInt(1000000),
    { simulate: true }
  );

  console.log('Transfer simulation:', transferSim.simulation?.status);

  if (transferSim.simulation?.status === 'success') {
    const transferResult = await client.transfer(
      sourceKeypair,
      sourceKeypair.publicKey(),
      'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37',
      BigInt(1000000)
    );
    console.log('Transfer hash:', transferResult.transactionHash);
  }

  // Get balance
  const balance = await client.balance(
    sourceKeypair,
    'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37',
    { simulate: true }
  );
  console.log('Balance:', balance.result);

  // Get token info
  const info = await client.get_info(
    sourceKeypair,
    { simulate: true }
  );
  console.log('Token info:', info.result);
}

main().catch(console.error);
```

## Step 7: Add Tests

```typescript
// frontend/src/__tests__/token.test.ts
import { describe, it, expect, beforeAll } from '@jest/globals';
import { TokenContractClient } from '../bindings';
import * as StellarSdk from '@stellar/stellar-sdk';

describe('Token Contract', () => {
  let client: TokenContractClient;
  let sourceKeypair: StellarSdk.Keypair;

  beforeAll(() => {
    client = new TokenContractClient({
      contractId: process.env.CONTRACT_ID!,
      network: 'testnet',
      enableSimulation: true,
    });

    sourceKeypair = StellarSdk.Keypair.fromSecret(
      process.env.SECRET_KEY!
    );
  });

  it('should initialize token', async () => {
    const result = await client.initialize(
      sourceKeypair,
      'TestToken',
      'TST',
      7,
      { simulate: true }
    );

    expect(result.simulation?.status).toBe('success');
    expect(result.simulation?.budgetUsage?.cpuUsagePercent).toBeLessThan(80);
  });

  it('should transfer tokens', async () => {
    const recipient = 'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37';
    const amount = BigInt(1000000);

    const result = await client.transfer(
      sourceKeypair,
      sourceKeypair.publicKey(),
      recipient,
      amount,
      { simulate: true }
    );

    expect(result.simulation?.status).toBe('success');
  });

  it('should get balance', async () => {
    const address = 'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37';

    const result = await client.balance(
      sourceKeypair,
      address,
      { simulate: true }
    );

    expect(result.result).toBeGreaterThanOrEqual(0);
  });

  it('should get token info', async () => {
    const result = await client.get_info(
      sourceKeypair,
      { simulate: true }
    );

    expect(result.result).toHaveProperty('name');
    expect(result.result).toHaveProperty('symbol');
    expect(result.result).toHaveProperty('decimals');
  });

  it('should fail with insufficient balance', async () => {
    const recipient = 'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37';
    const hugeAmount = BigInt(999999999999);

    await expect(
      client.transfer(
        sourceKeypair,
        sourceKeypair.publicKey(),
        recipient,
        hugeAmount,
        { simulate: true }
      )
    ).rejects.toThrow();
  });
});
```

## Step 8: CI/CD Integration

```yaml
# .github/workflows/contract-tests.yml
name: Contract Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Setup Rust
        uses: actions-rs/toolchain@v1
        with:
          toolchain: stable
          target: wasm32-unknown-unknown

      - name: Install Soroban CLI
        run: |
          cargo install --locked soroban-cli

      - name: Build Contract
        run: |
          soroban contract build

      - name: Install erst
        run: |
          go install github.com/dotandev/hintents/cmd/erst@latest

      - name: Generate TypeScript Bindings
        run: |
          erst generate-bindings \
            target/wasm32-unknown-unknown/release/token_contract.wasm \
            --output ./frontend/src/bindings \
            --package token-contract

      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install Dependencies
        run: |
          cd frontend
          npm install

      - name: Run Tests
        run: |
          cd frontend
          npm test
        env:
          CONTRACT_ID: ${{ secrets.CONTRACT_ID }}
          SECRET_KEY: ${{ secrets.SECRET_KEY }}

      - name: Check Budget Usage
        run: |
          cd frontend
          npm run test:budget
```

## Step 9: Production Deployment

```typescript
// frontend/src/production.ts
import { TokenContractClient } from './bindings';
import * as StellarSdk from '@stellar/stellar-sdk';

// Production configuration
const client = new TokenContractClient({
  contractId: process.env.PROD_CONTRACT_ID!,
  network: 'mainnet',
  rpcUrl: 'https://soroban-mainnet.stellar.org',
  enableSimulation: false, // Disable simulation in production
});

// Always simulate in staging first
const stagingClient = new TokenContractClient({
  contractId: process.env.STAGING_CONTRACT_ID!,
  network: 'testnet',
  enableSimulation: true,
});

async function deployToProduction() {
  // Test in staging first
  console.log('Testing in staging...');
  const stagingResult = await stagingClient.transfer(
    sourceKeypair,
    from,
    to,
    amount,
    { simulate: true }
  );

  if (stagingResult.simulation?.status !== 'success') {
    throw new Error('Staging simulation failed');
  }

  // Deploy to production
  console.log('Deploying to production...');
  const prodResult = await client.transfer(
    sourceKeypair,
    from,
    to,
    amount
  );

  console.log('Production transaction:', prodResult.transactionHash);
}
```

## Best Practices Summary

1. **Always simulate first** - Use `{ simulate: true }` during development
2. **Monitor resource usage** - Check CPU and memory usage in simulations
3. **Test thoroughly** - Write comprehensive tests with erst simulation
4. **Version control bindings** - Commit generated bindings to git
5. **Regenerate on changes** - Always regenerate when contract changes
6. **Use staging** - Test on testnet before mainnet deployment
7. **Handle errors** - Implement proper error handling for all contract calls
8. **CI/CD integration** - Automate testing and deployment

## Troubleshooting

### Common Issues

1. **Contract spec not found**
   - Ensure contract is built with `soroban contract build`
   - Check WASM file includes spec section

2. **Type errors**
   - Regenerate bindings after contract changes
   - Ensure dependencies are up to date

3. **Simulation failures**
   - Check erst is installed and in PATH
   - Verify network connectivity
   - Review contract logic for errors

4. **Transaction failures**
   - Always simulate before executing
   - Check account has sufficient balance
   - Verify contract is deployed

## Next Steps

- Explore [TypeScript Bindings Documentation](../../docs/TYPESCRIPT_BINDINGS.md)
- Read [Erst CLI Reference](../../docs/CLI.md)
- Check [Example Contracts](../plugins/)
- Join [Stellar Discord](https://discord.gg/stellar) for support
