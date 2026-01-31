# Troubleshooting Guide: XDR Parsing and Footprint Extraction

Common issues and solutions when working with Stellar XDR and footprint extraction.

## Common Errors

### "Failed to decode TransactionMeta XDR"

**Cause:** Invalid base64 XDR string or corrupted data

**Solutions:**
1. Verify the XDR is valid base64:
   ```typescript
   const isValid = /^[A-Za-z0-9+/]*={0,2}$/.test(xdrString);
   ```

2. Check you're using `TransactionMeta` not `TransactionEnvelope`:
   ```typescript
   // Wrong - this is the envelope
   const envelopeXdr = tx.envelope_xdr;
   
   // Correct - this is the meta
   const metaXdr = tx.result_meta_xdr;
   ```

3. Ensure the XDR hasn't been truncated or modified

### "Unknown TransactionMeta version"

**Cause:** Unsupported protocol version or future meta format

**Solutions:**
1. Update to latest `@stellar/stellar-sdk`
2. Check if a new protocol version was released
3. Verify the transaction is from a supported network

### "Unknown ledger entry type"

**Cause:** New ledger entry type introduced in newer protocol

**Solutions:**
1. Update dependencies
2. Add support for new entry type
3. Log and skip unknown types for backward compatibility

### Empty Footprint

**Cause:** Transaction has no ledger state changes

**Possible reasons:**
- Failed transaction (check `result_code`)
- Fee-only transaction
- Empty operations array

**Debug:**
```typescript
console.log('Operations:', meta.operations().length);
console.log('Changes before:', meta.txChangesBefore().length);
console.log('Changes after:', meta.txChangesAfter().length);
```

### Duplicate Keys Not Filtered

**Cause:** Hash collision or empty hash values

**Solution:**
The deduplication now validates hashes:
```typescript
if (key.hash && key.hash.length > 0 && !seen.has(key.hash)) {
  // Process key
}
```

### Missing Soroban Contract Data

**Cause:** Not extracting from `sorobanMeta` in v3

**Solution:**
Ensure v3 extraction includes:
```typescript
const sorobanMeta = meta.sorobanMeta();
if (sorobanMeta) {
  const sorobanKeys = this.extractFromSorobanMeta(sorobanMeta);
  keys.push(...sorobanKeys);
}
```

## Performance Issues

### Slow Extraction on Large Transactions

**Symptoms:** Extraction takes > 1 second

**Solutions:**
1. Profile the extraction:
   ```typescript
   console.time('extraction');
   const footprint = FootprintExtractor.extractFootprint(metaXdr);
   console.timeEnd('extraction');
   ```

2. Check transaction size:
   ```typescript
   const buffer = Buffer.from(metaXdr, 'base64');
   console.log('Meta size:', buffer.length, 'bytes');
   ```

3. Consider caching for repeated extractions

### High Memory Usage

**Cause:** Large number of ledger changes

**Solutions:**
1. Process in batches if extracting multiple transactions
2. Clear references after use
3. Use streaming for bulk processing

## XDR Specific Issues

### ConfigSetting Entries Not Supported

**Current Status:** ConfigSetting entries are logged but not fully extracted

**Workaround:**
```typescript
// These are rare and typically only in network upgrade transactions
// Most applications can safely ignore them
```

**Future:** Will be supported in future protocol versions

### TTL Entries Missing

**Cause:** TTL entries are Soroban-specific

**Solution:**
Ensure you're using v3 meta for Soroban transactions

## Debugging Tips

### Enable Verbose Logging

```typescript
// The extractor logs key information
console.log = (...args) => {
  // Your custom logging
};
```

### Inspect Raw XDR

```typescript
import { xdr } from '@stellar/stellar-sdk';

const meta = xdr.TransactionMeta.fromXDR(Buffer.from(metaXdr, 'base64'));
console.log(JSON.stringify(meta, null, 2));
```

### Compare with Stellar CLI

```bash
stellar tx inspect --xdr <transaction-xdr>
```

### Test with Known Transactions

Use transactions from Stellar's public networks:
- Testnet: https://horizon-testnet.stellar.org
- Mainnet: https://horizon.stellar.org

## Getting Help

1. Check the [main documentation](./FOOTPRINT_EXTRACTION.md)
2. Review test cases in `src/footprint/__tests__/`
3. Search for similar issues in the Stellar SDK
4. Ask in Stellar Developer Discord

## Reporting Bugs

When reporting issues, include:
1. Transaction hash
2. Network (testnet/mainnet)
3. Error message
4. Stellar SDK version
5. Meta version (v1/v2/v3)
