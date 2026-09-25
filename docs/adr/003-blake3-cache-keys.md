# ADR-003: BLAKE3 for Internal Cache Keys

## Status

Accepted

## Context

ADR-001 chose SHA-256 as the content hash used to name source-map cache entries
in `simulator/src/source_map_cache.rs`. Those keys are purely internal,
content-addressed lookup identifiers: they are never used to authenticate a
peer, verify a signature, or make a security decision, and the cache file is
local to the user's machine. SHA-256's cryptographic strength therefore buys
nothing for this use, while its throughput is lower than a modern hash built for
memory-bound work.

## Decision

Cache-key generation moves from `sha2` (SHA-256) to `blake3`:

- `SourceMapCache::compute_wasm_hash` hashes WASM bytes with `blake3`.
- `SourceMapCache::compute_cache_key` mixes the file mtime with a BLAKE3
  streaming hasher.
- `simulator/Cargo.toml` drops `sha2` and adds `blake3`.

The digest stays 32 bytes, so the hex-encoded key remains 64 characters and no
caller or on-disk filename format changes. Existing cache entries are simply
never hit again and are re-derived on next use.

## Rationale

- **Performance**: BLAKE3 is materially faster than SHA-256 for the
  memory-bound hashing done on every cache lookup, which is the stated problem.
- **No security regression in practice**: these keys do not back any security
  property. Cryptographic hashing is still used where it matters — the Go
  audit/verify paths keep `crypto/sha256`.
- **Same key shape**: a 32-byte digest keeps the 64-character hex key and the
  `{hash}.bin` cache layout intact, so the change stays internal.

## Consequences

- Existing local source-map caches are invalidated once (miss, then re-store).
- `sha2` is no longer a dependency of the simulator crate; `blake3` is added.
- ADR-001 is superseded for the hash-algorithm choice; its bincode and
  filesystem decisions still stand.
