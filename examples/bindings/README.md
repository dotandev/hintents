# TypeScript Bindings Generator Example

This example demonstrates how to generate TypeScript bindings for a Soroban smart contract using `erst generate-bindings`.

## Prerequisites

- `erst` CLI tool installed
- Node.js and npm installed
- A compiled Soroban smart contract WASM file

## Quick Start

### 1. Generate Bindings

```bash
# Generate bindings from a WASM file
erst generate-bindings contract.wasm --output ./generated --package my-contract

# With contract ID and network
erst generate-bindings contract.wasm \
  --output ./generated \
  --package my-contract \
  --contract-id CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2 \
  --network testnet
```

### 2. Install Dependencies

```bash
cd generated
npm install
npm run build
```

### 3. Use the Generated Client

```typescript
import { MyContractClient } from './generated';
import * as StellarSdk from '@stellar/stellar-sdk';

// Initialize the client
const client = new MyContractClient({
  contractId: 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2',
  network: 'testnet',
  enableSimulation: true, // Enable erst simulation
});

// Create a keypair
const sourceKeypair = StellarSdk.Keypair.fromSecret('SECRET_KEY');

// Call contract methods with type safety
async function example() {
  try {
    // Simulate first (no transaction sent)
    const simResult = await client.transfer(
      sourceKeypair,
      'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37',
      BigInt(1000000),
      { simulate: true }
    );
    
    console.log('Simulation result:', simResult);
    console.log('Budget usage:', simResult.simulation?.budgetUsage);
    
    // If simulation looks good, execute for real
    const result = await client.transfer(
      sourceKeypair,
      'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37',
      BigInt(1000000)
    );
    
    console.log('Transaction hash:', result.transactionHash);
    console.log('Result:', result.result);
  } catch (error) {
    console.error('Error:', error);
  }
}

example();
```

## Generated Files

The generator creates the following files:

- `types.ts` - TypeScript type definitions for all contract types
- `client.ts` - Main client class with typed methods
- `erst-integration.ts` - Erst simulator integration
- `index.ts` - Main export file
- `package.json` - NPM package configuration
- `README.md` - Usage documentation

## Features

### Type Safety

All contract methods are fully typed:

```typescript
// Function signature is generated from contract spec
async transfer(
  source: StellarSdk.Keypair,
  to: Address,
  amount: bigint,
  options?: CallOptions
): Promise<CallResult<void>>
```

### Erst Simulation

Test transactions locally before submitting:

```typescript
const result = await client.someMethod(
  sourceKeypair,
  args,
  { simulate: true } // Runs through erst simulator
);

// Check resource usage
console.log('CPU:', result.simulation.budgetUsage.cpuInstructions);
console.log('Memory:', result.simulation.budgetUsage.memoryBytes);
```

### Error Handling

Strongly-typed error enums:

```typescript
try {
  await client.transfer(source, to, amount);
} catch (error) {
  if (error instanceof InsufficientBalanceError) {
    console.log('Error code:', error.code);
  }
}
```

### Custom Types

All contract structs, enums, and unions are generated:

```typescript
// From contract spec
interface TokenMetadata {
  name: string;
  symbol: string;
  decimals: number;
}

enum TransferStatus {
  Success = 0,
  Failed = 1,
  Pending = 2,
}
```

## Advanced Usage

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
  }
);
```

### Debug Failed Transactions

```typescript
import { ErstSimulator } from './generated';

const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
});

// Debug a failed transaction
const debugResult = await simulator.debugTransaction(
  '5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab'
);

console.log('Error:', debugResult.error);
console.log('Events:', debugResult.events);
console.log('Logs:', debugResult.logs);
```

## Integration with CI/CD

Use erst simulation in your test pipeline:

```typescript
import { describe, it, expect } from '@jest/globals';

describe('Contract Tests', () => {
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
});
```

## Troubleshooting

### Erst not found

Make sure `erst` is in your PATH:

```bash
which erst
# or specify custom path
const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
  erstPath: '/path/to/erst',
});
```

### Contract spec not found

Ensure your WASM file includes the contract specification:

```bash
# Check if spec exists
erst inspect contract.wasm
```

## License

Apache-2.0
