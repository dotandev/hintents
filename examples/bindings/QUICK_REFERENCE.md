# TypeScript Bindings - Quick Reference

## Generate Bindings

```bash
# Basic
erst generate-bindings contract.wasm

# With options
erst generate-bindings contract.wasm \
  --output ./src/bindings \
  --package my-contract \
  --contract-id CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2 \
  --network testnet
```

## Initialize Client

```typescript
import { MyContractClient } from './bindings';

const client = new MyContractClient({
  contractId: 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2',
  network: 'testnet',
  enableSimulation: true,
});
```

## Call Methods

```typescript
// Simulate
const simResult = await client.transfer(
  sourceKeypair,
  recipient,
  amount,
  { simulate: true }
);

// Execute
const result = await client.transfer(
  sourceKeypair,
  recipient,
  amount
);
```

## Check Simulation

```typescript
console.log('Status:', result.simulation.status);
console.log('CPU:', result.simulation.budgetUsage.cpuUsagePercent);
console.log('Memory:', result.simulation.budgetUsage.memoryUsagePercent);
console.log('Events:', result.simulation.events);
```

## Debug Transaction

```typescript
import { ErstSimulator } from './bindings';

const simulator = new ErstSimulator({
  network: 'testnet',
  rpcUrl: 'https://soroban-testnet.stellar.org',
});

const debug = await simulator.debugTransaction(txHash);
```

## Type Mapping

| Soroban | TypeScript |
|---------|-----------|
| Bool | boolean |
| U32, I32 | number |
| U64, I64, U128, I128 | bigint |
| String | string |
| Address | string |
| Bytes | Uint8Array |
| Option<T> | T \| null |
| Vec<T> | T[] |
| Map<K,V> | Map<K,V> |

## Error Handling

```typescript
try {
  await client.transfer(source, to, amount);
} catch (error) {
  if (error instanceof ContractErrorError) {
    console.log('Contract error:', error.code);
  }
}
```

## Testing

```typescript
it('should work', async () => {
  const result = await client.someMethod(
    sourceKeypair,
    args,
    { simulate: true }
  );
  
  expect(result.simulation.status).toBe('success');
});
```

## CI/CD

```yaml
- name: Generate Bindings
  run: erst generate-bindings contract.wasm --output ./bindings

- name: Install & Test
  run: |
    cd bindings
    npm install
    npm test
```

## Common Options

```typescript
{
  simulate: true,           // Use erst simulation
  fee: '10000',            // Custom fee
  memo: Memo.text('...'),  // Transaction memo
  timeoutInSeconds: 60,    // Timeout
}
```

## Files Generated

- `types.ts` - Type definitions
- `client.ts` - Client class
- `erst-integration.ts` - Simulator
- `index.ts` - Exports
- `package.json` - NPM config
- `README.md` - Documentation
