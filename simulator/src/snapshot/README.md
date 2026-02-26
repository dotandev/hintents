# Ledger Snapshot Module

This module provides standalone utilities for managing ledger snapshots and storage loading in Soroban simulations.

## Purpose

The snapshot module extracts and encapsulates the logic for:
- Decoding XDR-encoded ledger entries from base64
- Managing ledger state snapshots
- Loading ledger entries for transaction replay

This functionality can be shared across different Soroban tools that need to reconstruct ledger state for simulation or analysis.

## Usage

### Creating a Snapshot from Base64 Entries

```rust
use std::collections::HashMap;
use simulator::snapshot::LedgerSnapshot;

// Map of base64-encoded LedgerKey -> LedgerEntry
let entries = HashMap::from([
    ("base64_key".to_string(), "base64_entry".to_string()),
]);

let snapshot = LedgerSnapshot::from_base64_map(&entries)?;
println!("Loaded {} entries", snapshot.len());
```

### Decoding Individual Entries

```rust
use simulator::snapshot::{decode_ledger_key, decode_ledger_entry};

let key = decode_ledger_key("base64_encoded_key")?;
let entry = decode_ledger_entry("base64_encoded_entry")?;
```

### Working with Snapshots

```rust
let mut snapshot = LedgerSnapshot::new();

// Insert entries
snapshot.insert(key_bytes, entry);

// Query entries
if let Some(entry) = snapshot.get(&key_bytes) {
    println!("Found entry: {:?}", entry);
}

// Iterate over all entries
for (key, entry) in snapshot.iter() {
    // Process each entry
}
```

## Error Handling

The module provides a `SnapshotError` enum for comprehensive error handling:

```rust
use simulator::snapshot::SnapshotError;

match LedgerSnapshot::from_base64_map(&entries) {
    Ok(snapshot) => { /* use snapshot */ },
    Err(SnapshotError::Base64Decode(msg)) => {
        eprintln!("Base64 decoding failed: {}", msg);
    },
    Err(SnapshotError::XdrParse(msg)) => {
        eprintln!("XDR parsing failed: {}", msg);
    },
    Err(e) => {
        eprintln!("Other error: {}", e);
    }
}
```

## Integration with Soroban Host

The snapshot module is designed to work seamlessly with `soroban-env-host`:

```rust
use soroban_env_host::Host;
use simulator::snapshot::LedgerSnapshot;

// Load snapshot
let snapshot = LedgerSnapshot::from_base64_map(&entries)?;

// Initialize host
let host = Host::default();

// Use snapshot entries to populate host storage
// (Implementation depends on your specific use case)
```

## Testing

The module includes comprehensive unit tests:

```bash
cd simulator
cargo test snapshot::tests
```

## Design Decisions

1. **Standalone Module**: Extracted from main simulator logic to enable reuse across tools
2. **Type Safety**: Uses strongly-typed XDR structures from `soroban-env-host`
3. **Error Handling**: Provides detailed error types for debugging
4. **Public API**: All public methods are documented and designed for external use
5. **Zero-Copy Where Possible**: Uses references to avoid unnecessary cloning

## Future Enhancements

- Support for streaming large snapshots
- Compression support for snapshot storage
- Incremental snapshot updates
- Snapshot diffing capabilities
