# Changelog

All notable changes to the **Stellar ERST SDK** project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Foundation for **Time-Travel Debugging & State Rollback**.
- `StateSnapshot` models in Rust simulator and Go SDK.
- Support for capturing ledger state before/after host function calls.

### Impact
- Developers will soon be able to rewind transactions in the interactive debugger.

---

## [v2.0.0] - 2026-03-20

### Added
- **Interactive Trace Viewer**: Searchable terminal UI for exploring transaction traces.
- **Performance Profiling**: Flamegraph generation for CPU and Memory consumption.
- **Audit Log Signing**: Support for software and PKCS#11 HSM (YubiKey) signing.
- **Protocol Handler**: `erst://` URI scheme for deep-linking into debug sessions.

### Changed
- Refactored RPC client for improved failover and rotation logic.
- Standardized error handling across all SDK modules.

### Fixed
- Resolved race conditions in concurrent contract invocations.
- Fixed XDR pointer compatibility issues in ledger tests.

### Impact
- Major upgrade providing much higher visibility into contract execution.
- HSM support enables production-grade audit trails.

---

## [v1.0.0] - 2026-01-15

### Added
- Initial release of the ERST toolchain.
- Basic transaction replay and simulation.
- Support for Stellar Testnet and Mainnet.
