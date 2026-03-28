# TypeScript Bindings Generator

The `erst generate-bindings` command generates strongly-typed TypeScript client libraries for Soroban smart contracts. These bindings provide type-safe method calls with built-in erst simulation and debugging capabilities.

## Overview

The bindings generator:

1. Extracts the contract specification from WASM files
2. Generates TypeScript type definitions for all contract types
3. Creates a client class with typed methods
4. Integrates erst for local simulation and debugging
5. Provides full IntelliSense support in IDEs

## Installation

```bash
# Install erst
go install github.com/dotandev/hintents/cmd/erst@latest

# Verify installation
erst --version
```

## Quick Start

### Generate Bindings

```bash
# Basic usage
erst generate-bindings contract.wasm

# With options
erst generate-bindings contract.wasm \
  --output ./src/bindings \
  --package my-contract \
  --contract-id CDLZFC... \
  --network testnet
```

### Command Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output directory | Current directory |
| `--package` | `-p` | Package name | WASM filename |
| `--contract-id` | | Contract ID for network calls | None |
| `--network` | `-n` | Stellar network | `testnet` |

## Generated Structure

```
output-directory/
├── types.ts              # Type definitions
├── client.ts             # Main client class
├── erst-integration.ts   # Erst simulator
├── index.ts              # Exports
├── package.json          # NPM config
└── README.md             # Usage docs
```

## Usage Examples

### Basic Client Usage

```typescript
import { MyContractClient } from './bindings';
import * as StellarSdk from '@stellar/stellar-sdk';

const client = new MyContractClient({
  contractId: 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2',
  network: 'testnet',
  enableSimulation: true,
});

const sourceKeypair = StellarSdk.Keypair.fromSecret('SECRET_KEY');

// Call contract method
const result = await client.transfer(
  sourceKeypair,
  'RECIPIENT_ADDRESS',
  BigInt(1000000)
);
```


### Simulation with Erst

```typescript
// Simulate before executing
const simResult = await client.transfer(
  sourceKeypair,
  recipient,
  amount,
  { simulate: true }
);

// Check simulation results
console.log('Status:', simResult.simulation.status);
console.log('CPU usage:', simResult.simulation.budgetUsage.cpuUsagePercent);
console.log('Memory usage:', simResult.simulation.budgetUsage.memoryUsagePercent);
console.log('Events:', simResult.simulation.events);

// Execute if simulation succeeds
if (simResult.simulation.status === 'success') {
  const txResult = await client.transfer(sourceKeypair, recipient, amount);
  console.log('Transaction hash:', txResult.transactionHash);
}
```

### Type Safety

All contract types are fully typed:

```typescript
// Structs
interface TokenMetadata {
  name: string;
  symbol: string;
  decimals: number;
}

// Enums
enum TransferStatus {
  Success = 0,
  Failed = 1,
  Pending = 2,
}

// Unions (discriminated)
type Result = 
  | { tag: 'Ok'; values: [bigint] }
  | { tag: 'Err'; values: [string] };

// Error enums with classes
enum ContractError {
  InsufficientBalance = 1,
  Unauthorized = 2,
}

class ContractErrorError extends Error {
  constructor(public code: ContractError, message?: string);
}
```

### Debug Failed Transactions

```typescript
import { ErstSimulator } from './bindings';

const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
});

const debugResult = await simulator.debugTransaction(
  '5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab'
);

console.log('Error:', debugResult.error);
console.log('Events:', debugResult.events);
console.log('Logs:', debugResult.logs);
console.log('Budget:', debugResult.budgetUsage);
```

## Type Mapping

Soroban types are mapped to TypeScript as follows:

| Soroban Type | TypeScript Type |
|--------------|-----------------|
| `Bool` | `boolean` |
| `U32`, `I32` | `number` |
| `U64`, `I64`, `U128`, `I128`, `U256`, `I256` | `bigint` |
| `String` | `string` |
| `Symbol` | `string` |
| `Address` | `string` |
| `Bytes` | `Uint8Array` |
| `BytesN(N)` | `Uint8Array` |
| `Timepoint` | `Date` |
| `Duration` | `number` |
| `Option<T>` | `T \| null` |
| `Vec<T>` | `T[]` |
| `Map<K, V>` | `Map<K, V>` |
| `Tuple<T1, T2>` | `[T1, T2]` |
| `Result<T, E>` | `Result<T, E>` |
| Custom structs | `interface` |
| Custom enums | `enum` |
| Custom unions | Discriminated union |

## Advanced Features

### Custom RPC URL

```typescript
const client = new MyContractClient({
  contractId: 'CONTRACT_ID',
  network: 'testnet',
  rpcUrl: 'https://custom-rpc.example.com',
});
```

### Transaction Options

```typescript
const result = await client.someMethod(
  sourceKeypair,
  args,
  {
    fee: '10000',
    memo: StellarSdk.Memo.text('My transaction'),
    timeoutInSeconds: 60,
    simulate: false,
  }
);
```

### Custom Erst Path

```typescript
const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
  erstPath: '/custom/path/to/erst',
});
```

## CI/CD Integration

### Jest Testing

```typescript
import { describe, it, expect } from '@jest/globals';
import { MyContractClient } from './bindings';

describe('Contract Tests', () => {
  let client: MyContractClient;

  beforeAll(() => {
    client = new MyContractClient({
      contractId: process.env.CONTRACT_ID!,
      network: 'testnet',
      enableSimulation: true,
    });
  });

  it('should transfer tokens successfully', async () => {
    const result = await client.transfer(
      sourceKeypair,
      recipient,
      BigInt(1000),
      { simulate: true }
    );
    
    expect(result.simulation.status).toBe('success');
    expect(result.simulation.budgetUsage.cpuUsagePercent).toBeLessThan(80);
  });

  it('should fail with insufficient balance', async () => {
    await expect(
      client.transfer(
        sourceKeypair,
        recipient,
        BigInt(999999999999),
        { simulate: true }
      )
    ).rejects.toThrow();
  });
});
```

### GitHub Actions

```yaml
name: Contract Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install erst
        run: |
          go install github.com/dotandev/hintents/cmd/erst@latest
      
      - name: Generate bindings
        run: |
          erst generate-bindings contract.wasm --output ./bindings
      
      - name: Install dependencies
        run: |
          cd bindings
          npm install
      
      - name: Run tests
        run: |
          npm test
```

## Best Practices

### 1. Version Control

Commit generated bindings to version control for consistency:

```bash
erst generate-bindings contract.wasm --output ./src/bindings
git add src/bindings
git commit -m "Update contract bindings"
```

### 2. Regenerate on Contract Changes

Always regenerate bindings when the contract changes:

```bash
# After updating contract
soroban contract build
erst generate-bindings target/wasm32-unknown-unknown/release/contract.wasm
```

### 3. Use Simulation in Development

Always simulate transactions during development:

```typescript
const result = await client.someMethod(
  sourceKeypair,
  args,
  { simulate: true }
);

// Check results before executing for real
if (result.simulation.status === 'success') {
  // Execute transaction
}
```

### 4. Handle Errors Gracefully

```typescript
try {
  const result = await client.transfer(source, to, amount);
  console.log('Success:', result.transactionHash);
} catch (error) {
  if (error instanceof ContractErrorError) {
    console.error('Contract error:', error.code);
  } else {
    console.error('Network error:', error);
  }
}
```

### 5. Monitor Resource Usage

```typescript
const result = await client.someMethod(
  sourceKeypair,
  args,
  { simulate: true }
);

const usage = result.simulation.budgetUsage;
if (usage.cpuUsagePercent > 80) {
  console.warn('High CPU usage detected');
}
if (usage.memoryUsagePercent > 80) {
  console.warn('High memory usage detected');
}
```

## Troubleshooting

### Contract Spec Not Found

**Problem:** `contract spec not found in WASM file`

**Solution:** Ensure your contract includes the spec:

```rust
// In your Rust contract
#[contractimpl]
impl MyContract {
    // Your methods
}
```

Build with spec:
```bash
soroban contract build
```

### Erst Not Found

**Problem:** `erst: command not found`

**Solution:** Install erst or specify path:

```bash
# Install
go install github.com/dotandev/hintents/cmd/erst@latest

# Or specify path
const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
  erstPath: '/path/to/erst',
});
```

### Type Errors

**Problem:** TypeScript type errors in generated code

**Solution:** Ensure you have the correct dependencies:

```bash
npm install @stellar/stellar-sdk@^12.0.0
npm install --save-dev @types/node typescript
```

### Simulation Timeout

**Problem:** Simulation takes too long

**Solution:** Increase timeout or check contract complexity:

```typescript
const result = await client.someMethod(
  sourceKeypair,
  args,
  { 
    simulate: true,
    timeoutInSeconds: 120 
  }
);
```

## API Reference

### ClientConfig

```typescript
interface ClientConfig {
  contractId: string;
  network: 'testnet' | 'mainnet' | 'futurenet';
  rpcUrl?: string;
  enableSimulation?: boolean;
}
```

### CallOptions

```typescript
interface CallOptions {
  simulate?: boolean;
  fee?: string;
  memo?: StellarSdk.Memo;
  timeoutInSeconds?: number;
}
```

### CallResult<T>

```typescript
interface CallResult<T> {
  result: T;
  transactionHash?: string;
  simulation?: SimulationResult;
}
```

### SimulationResult

```typescript
interface SimulationResult {
  status: 'success' | 'error';
  result?: any;
  error?: string;
  events?: any[];
  logs?: string[];
  budgetUsage?: {
    cpuInstructions: number;
    memoryBytes: number;
    cpuUsagePercent: number;
    memoryUsagePercent: number;
  };
}
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## License

Apache-2.0
