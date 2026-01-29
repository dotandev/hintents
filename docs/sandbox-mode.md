# Sandbox Mode - Manual State Override

## Overview
Sandbox mode allows you to manually override ledger entry state when debugging Stellar transactions. This is useful for testing "what-if" scenarios without deploying changes to the network.

## Usage

```bash
erst debug --override-state ./overrides.json <transaction-hash>
```

## Override File Format

Create a JSON file with the following structure:

```json
{
  "ledger_entries": {
    "entry_key_1": "base64_encoded_xdr_value",
    "entry_key_2": "base64_encoded_xdr_value"
  }
}
```

### Example

```json
{
  "ledger_entries": {
    "AAAAAAAAAAC6hsKutUTv8P4rkKBTPJIKJvhqEMH3L9sEqKnG9nT/bQ==": "AAAABgAAAAFv8F+E0D/BE04jR47s+JhGi1Q/T/yxfC8UgG88j68rAAAAAAAAAAB+SCAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=",
    "test_account_id": "base64_encoded_account_state"
  }
}
```

## Behavior

- When `--override-state` is provided, the specified ledger entries are used for simulation
- The system logs: `Sandbox mode active: X entries overridden`
- Overrides completely replace fetched RPC data for testing purposes
- Without the flag, normal behavior continues (fetches from RPC)

## Use Cases

1. **Testing contract behavior with modified state**
   - Override account balances
   - Modify contract data entries
   - Test edge cases

2. **Debugging failed transactions**
   - Replay with corrected state
   - Identify state-dependent issues

3. **Development and testing**
   - Test without deploying to network
   - Rapid iteration on contract logic
