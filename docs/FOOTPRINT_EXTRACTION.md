# Ledger Footprint Extraction

Extract the exact ledger state a transaction touched by parsing `TransactionMeta` XDR, supporting all meta versions (v1/v2/v3) with proper deduplication and comprehensive testing.

## Overview

The footprint extractor parses `TransactionMeta` XDR to identify all ledger entries (keys) that were read or written during transaction execution. This is critical for transaction replay and state reconstruction.

## Usage

```typescript
import { FootprintExtractor } from './footprint/extractor';

// Get transaction meta XDR from RPC
const metaXdr = transaction.result_meta_xdr;

// Extract footprint
const footprint = FootprintExtractor.extractFootprint(metaXdr);

console.log('All keys:', footprint.all);
console.log('Read-only keys:', footprint.readOnly);
console.log('Read-write keys:', footprint.readWrite);
```

## Output Format

```typescript
interface FootprintResult {
  readOnly: LedgerKey[];   // Keys only read
  readWrite: LedgerKey[];  // Keys modified
  all: LedgerKey[];        // All unique keys
}

interface LedgerKey {
  type: LedgerEntryType;   // ACCOUNT, TRUSTLINE, etc.
  key: string;             // Base64 encoded XDR
  hash: string;            // SHA256 hash for deduplication
}
```

## Supported Meta Versions

### TransactionMeta V1 (Pre-Protocol 10)
- Basic operation-level changes
- No before/after transaction changes

### TransactionMeta V2 (Protocol 10-19)
- Operation-level changes
- Transaction-level before/after changes
- Most common format

### TransactionMeta V3 (Protocol 20+, Soroban)
- All V2 features
- Soroban metadata (events, return values)
- Contract data/code keys
- Diagnostic events

## Ledger Key Types

| Type | Description |
|------|-------------|
| `ACCOUNT` | Account entries |
| `TRUSTLINE` | Trustline entries |
| `OFFER` | DEX offers |
| `DATA` | Account data |
| `CLAIMABLE_BALANCE` | Claimable balances |
| `LIQUIDITY_POOL` | AMM liquidity pools |
| `CONTRACT_DATA` | Soroban contract persistent storage |
| `CONTRACT_CODE` | Soroban contract WASM |
| `CONFIG_SETTING` | Network config |
| `TTL` | Time-to-live entries |

## Key Deduplication

The extractor automatically deduplicates keys using SHA256 hashing:

```typescript
// Before deduplication
keys: [
  { hash: 'abc123...', ... },
  { hash: 'abc123...', ... },  // Duplicate
  { hash: 'def456...', ... },
]

// After deduplication
keys: [
  { hash: 'abc123...', ... },
  { hash: 'def456...', ... },
]
```

## Read/Write Separation

Keys are categorized based on `LedgerEntryChange` type:
- `LEDGER_ENTRY_STATE` → Read-only
- `LEDGER_ENTRY_CREATED`, `LEDGER_ENTRY_UPDATED`, `LEDGER_ENTRY_REMOVED` → Read-write

## Categorization

Group keys by type for analysis:

```typescript
const categorized = FootprintExtractor.categorizeKeys(footprint.all);

console.log('Accounts:', categorized.get(LedgerEntryType.ACCOUNT));
console.log('Contract data:', categorized.get(LedgerEntryType.CONTRACT_DATA));
```

## Error Handling

```typescript
try {
  const footprint = FootprintExtractor.extractFootprint(metaXdr);
} catch (error) {
  if (error.message.includes('Failed to decode')) {
    console.error('Invalid XDR format');
  } else if (error.message.includes('Unsupported meta version')) {
    console.error('Meta version not supported');
  }
}
```

## Real-World Example

```typescript
// Fetch transaction from Horizon
const tx = await server.transactions()
  .transaction(txHash)
  .call();

// Extract footprint
const footprint = FootprintExtractor.extractFootprint(tx.result_meta_xdr);

// Analyze touched entries
console.log(`Transaction touched ${footprint.all.length} ledger entries`);

const categorized = FootprintExtractor.categorizeKeys(footprint.all);
for (const [type, keys] of categorized) {
  console.log(`  ${type.name}: ${keys.length}`);
}

// Use for transaction replay
for (const key of footprint.all) {
  // Fetch ledger entry at the time of transaction
  const entry = await fetchLedgerEntry(key, tx.ledger);
  // Use for replay simulation
}
```

## Best Practices

1. **Always deduplicate** - Use provided deduplication
2. **Handle all versions** - Support v1, v2, v3
3. **Check Soroban meta** - V3 has critical contract data
4. **Categorize for analysis** - Group by type
5. **Cache hashes** - Reuse for performance

## Testing

Run the test suite:

```bash
npm test -- src/footprint/__tests__/extractor.spec.ts
```

## Integration with ERST

The footprint extractor is designed to integrate with the ERST transaction replay system:

1. Fetch transaction meta from RPC
2. Extract footprint to identify required ledger entries
3. Fetch those entries at the transaction's ledger sequence
4. Use for local simulation/replay

## Future Enhancements

- Support for future protocol versions
- Enhanced Soroban event parsing
- Performance optimizations for large transactions
- Caching layer for frequently accessed keys
