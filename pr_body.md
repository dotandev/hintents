## Overview

This PR optimizes the `SourceLocation` struct in the Rust simulator to use string interning via a global `PathInterner` for file paths. This deduplicates file path allocations in deep call stacks, significantly reducing memory usage.

## Related Issue

Closes #1903

## Changes

### SourceLocation Memory Optimization

* **ADD** Global `PathInterner` in `simulator/src/source_mapper.rs`
  * Thread-safe string interner using `OnceLock<Mutex<HashMap<String, Arc<str>>>>`
  * Returns deduplicated `Arc<str>` for any file path via `intern_path()`
  * Uses `Arc<str>` for thread-safe shared ownership across threads

* **MODIFY** `SourceLocation` struct in `simulator/src/source_mapper.rs`
  * Changed `file: String` to `file: Arc<str>`
  * Added custom serde serialization (`arc_str_serde`) to serialize as plain strings
  * Added `PartialEq` derive for use in `StackFrame`

* **MODIFY** All `SourceLocation` creation sites updated to use interner:
  * `extract_line_entries()` - DWARF line table parsing
  * `create_source_location()` - public API
  * All test files in `simulator/src/source_mapper.rs`, `simulator/src/source_map_cache.rs`, `simulator/tests/`

### Files Changed

- `simulator/src/source_mapper.rs` - Core interner implementation + SourceLocation changes
- `simulator/src/source_map_cache.rs` - Test updates for new SourceLocation format
- `simulator/tests/concurrency_test.rs` - Test update
- `simulator/tests/source_map_integration_test.rs` - Integration test update

## Verification Results

```
cargo test --all
443/443 tests passed

cargo clippy
No warnings or errors
```

| Acceptance Criteria | Status |
| --- | --- |
| SourceLocation uses string interning for file paths | Implemented via global PathInterner |
| File paths are deduplicated in memory | Arc<str> shared across all SourceLocation instances |
| cargo test passes | All 443 tests pass |
| cargo clippy passes | Clean, no warnings |
| Serialization works correctly | Custom serde module serializes as plain strings |

## Technical Details

The `PathInterner` maintains a global registry of file paths. When a new `SourceLocation` is created (e.g., during DWARF parsing in deep call stacks), the file path is interned via `intern_path()`. If the path already exists in the registry, the existing `Arc<str>` is returned, avoiding duplicate allocations. This is especially impactful in deep call stacks where the same file path appears many times.

Memory savings scale with call stack depth and number of unique source files referenced.