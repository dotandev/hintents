// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine;
use soroban_env_host::{
    budget::Budget,
    events::{Events, HostEvent},
    storage::{AccessType, Footprint, FootprintMap, Storage, StorageMap},
    xdr::{Hash, HostFunction, Limits, ScErrorCode, ScErrorType, ScVal, WriteXdr},
    DiagnosticLevel, Error as EnvError, Host, HostError, TryIntoVal, Val,
};
use std::io::{Read, Write};
use std::process::{Command, ExitStatus, Stdio};
use std::rc::Rc;
use std::thread;
use std::time::{Duration, Instant};

use crate::memory;
use crate::snapshot::{LedgerSnapshot, SnapshotError};
use crate::time::ClockDrift;
use tracing::{debug, instrument};

/// The highest ledger protocol version this build of the host understands.
///
/// Reading it from the host keeps the simulator's default in step with whatever
/// `soroban-env-host` the binary was compiled against, instead of a literal
/// that silently becomes "too old for host" after a dependency bump.
pub fn host_protocol_version() -> u32 {
    soroban_env_host::meta::INTERFACE_VERSION.protocol
}

#[derive(Debug, thiserror::Error)]
pub enum SimHostError {
    #[error(transparent)]
    Host(#[from] HostError),
    #[error(transparent)]
    Snapshot(#[from] SnapshotError),
    #[error(transparent)]
    AuthDepthExceeded(#[from] crate::host::AuthDepthExceeded),
    #[error("host execution panicked: {0}")]
    Panic(String),
    #[error("simulation exceeded its {limit_ms} ms timeout after {elapsed_ms} ms")]
    Timeout { elapsed_ms: u64, limit_ms: u64 },
    #[error("simulation worker failed: {0}")]
    Worker(String),
}

/// Configuration for a complete simulator run.
#[derive(Debug, Clone, Copy)]
pub struct RunnerConfig {
    timeout: Duration,
}

#[derive(Debug)]
pub struct RunnerOutput {
    pub status: ExitStatus,
    pub stdout: Vec<u8>,
}

impl Default for RunnerConfig {
    fn default() -> Self {
        Self {
            timeout: Duration::from_secs(30),
        }
    }
}

impl RunnerConfig {
    /// Sets the wall-clock limit for one simulator run.
    #[must_use]
    pub const fn with_timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    /// Returns the configured wall-clock limit.
    #[must_use]
    pub const fn timeout(&self) -> Duration {
        self.timeout
    }
}

const TIMEOUT_WORKER_ENV: &str = "ERST_SIM_TIMEOUT_WORKER";

/// Whether this process was started as the isolated worker for a timed run.
#[must_use]
pub fn is_timeout_worker() -> bool {
    std::env::var_os(TIMEOUT_WORKER_ENV).is_some()
}

/// Runs this simulator executable in a child process and bounds the complete
/// request, including contract execution, by `config.timeout()`.
pub fn run_request_with_timeout(
    request: &str,
    config: RunnerConfig,
) -> Result<RunnerOutput, SimHostError> {
    let executable = std::env::current_exe()
        .map_err(|error| SimHostError::Worker(format!("could not locate simulator: {error}")))?;
    let mut command = Command::new(executable);
    command
        .args(std::env::args_os().skip(1))
        .env(TIMEOUT_WORKER_ENV, "1");

    run_command_with_timeout(command, request.as_bytes(), config.timeout())
}

fn run_command_with_timeout(
    mut command: Command,
    request: &[u8],
    timeout: Duration,
) -> Result<RunnerOutput, SimHostError> {
    let started = Instant::now();
    if timeout.is_zero() {
        return Err(SimHostError::Timeout {
            elapsed_ms: 0,
            limit_ms: 0,
        });
    }

    command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit());
    let mut child = command
        .spawn()
        .map_err(|error| SimHostError::Worker(format!("could not start simulator: {error}")))?;

    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| SimHostError::Worker("simulator stdout was not captured".to_owned()))?;
    let output_reader = thread::Builder::new()
        .name("erst-sim-output".to_owned())
        .spawn(move || {
            let mut output = Vec::new();
            let mut stdout = stdout;
            stdout.read_to_end(&mut output).map(|_| output)
        })
        .map_err(|error| {
            let _ = child.kill();
            let _ = child.wait();
            SimHostError::Worker(format!("could not capture simulator output: {error}"))
        })?;

    let stdin = match child.stdin.take() {
        Some(stdin) => stdin,
        None => {
            let _ = child.kill();
            let _ = child.wait();
            let _ = output_reader.join();
            return Err(SimHostError::Worker(
                "simulator stdin was not captured".to_owned(),
            ));
        }
    };
    let request = request.to_vec();
    let request_writer = thread::Builder::new()
        .name("erst-sim-input".to_owned())
        .spawn(move || {
            let mut stdin = stdin;
            stdin.write_all(&request)
        });
    let request_writer = match request_writer {
        Ok(writer) => writer,
        Err(error) => {
            let _ = child.kill();
            let _ = child.wait();
            let _ = output_reader.join();
            return Err(SimHostError::Worker(format!(
                "could not send request to simulator: {error}"
            )));
        }
    };

    let status = loop {
        let elapsed = started.elapsed();
        if elapsed >= timeout {
            let _ = child.kill();
            let _ = child.wait();
            let _ = request_writer.join();
            let _ = output_reader.join();
            return Err(SimHostError::Timeout {
                elapsed_ms: duration_millis(elapsed),
                limit_ms: duration_millis(timeout),
            });
        }

        match child.try_wait() {
            Ok(Some(status)) => break status,
            Ok(None) => thread::sleep(
                timeout
                    .saturating_sub(elapsed)
                    .min(Duration::from_millis(10)),
            ),
            Err(error) => {
                let _ = child.kill();
                let _ = child.wait();
                let _ = request_writer.join();
                let _ = output_reader.join();
                return Err(SimHostError::Worker(format!(
                    "could not wait for simulator: {error}"
                )));
            }
        }
    };

    let request_result = request_writer
        .join()
        .map_err(|_| SimHostError::Worker("simulator input writer panicked".to_owned()))?;
    if let Err(error) = request_result {
        let _ = output_reader.join();
        return Err(SimHostError::Worker(format!(
            "could not send request to simulator: {error}"
        )));
    }

    let stdout = output_reader
        .join()
        .map_err(|_| SimHostError::Worker("simulator output reader panicked".to_owned()))?
        .map_err(|error| {
            SimHostError::Worker(format!("could not read simulator output: {error}"))
        })?;
    if stdout.is_empty() {
        return Err(SimHostError::Worker(format!(
            "simulator exited with {status} without a response"
        )));
    }

    Ok(RunnerOutput { status, stdout })
}

fn duration_millis(duration: Duration) -> u64 {
    u64::try_from(duration.as_millis()).unwrap_or(u64::MAX)
}

#[derive(Debug, Clone, Default)]
pub struct HostConfig {
    budget_limits: Option<(u64, u64)>,
    calibration: Option<crate::types::ResourceCalibration>,
    memory_limit: Option<u64>,
}

impl HostConfig {
    // The budget-specific builder is also used by the standalone fuzz target.
    #[allow(dead_code)]
    #[must_use]
    pub fn with_budget_limits(mut self, budget_limits: Option<(u64, u64)>) -> Self {
        self.budget_limits = budget_limits;
        self
    }

    #[must_use]
    pub fn with_calibration(
        mut self,
        calibration: Option<crate::types::ResourceCalibration>,
    ) -> Self {
        self.calibration = calibration;
        self
    }

    #[must_use]
    pub fn with_memory_limit(mut self, memory_limit: Option<u64>) -> Self {
        self.memory_limit = memory_limit;
        self
    }
}

pub struct SimHost {
    pub inner: Host,
    ledger_snapshot: LedgerSnapshot,
    config: HostConfig,
    pending_events: Vec<String>,
    clock: ClockDrift,
}

impl SimHost {
    fn panic_payload_to_string(panic_info: &(dyn std::any::Any + Send)) -> String {
        if let Some(s) = panic_info.downcast_ref::<String>() {
            s.clone()
        } else if let Some(s) = panic_info.downcast_ref::<&'static str>() {
            (*s).to_string()
        } else if let Some(s) = panic_info.downcast_ref::<&str>() {
            (*s).to_string()
        } else {
            "Unknown panic".to_string()
        }
    }

    pub fn with_panic_recovery<T, F>(&self, f: F) -> Result<T, SimHostError>
    where
        F: FnOnce() -> T,
    {
        match std::panic::catch_unwind(std::panic::AssertUnwindSafe(f)) {
            Ok(result) => Ok(result),
            Err(panic_info) => {
                let message = Self::panic_payload_to_string(panic_info.as_ref());
                Err(SimHostError::Panic(message))
            }
        }
    }

    pub fn invoke_function(&self, host_function: HostFunction) -> Result<ScVal, SimHostError> {
        self.with_panic_recovery(|| self.inner.invoke_function(host_function))
            .and_then(|result| result.map_err(SimHostError::Host))
    }

    /// Initialize a new Host with optional budget settings and resource calibration.
    #[instrument(
        level = "debug",
        fields(config = ?config)
    )]
    pub fn new(config: HostConfig) -> Self {
        debug!("initializing simulator host");
        let budget = Budget::default();

        if let Some((cpu, mem)) = config.budget_limits {
            let _ = budget.reset_limits(cpu, mem);
        } else if let Some(mem) = config.memory_limit {
            // soroban sdk uses u32::MAX for no limit but casted to u64, or u64::MAX.
            let _ = budget.reset_unlimited();
            let _ = budget.reset_limits(
                budget
                    .get_cpu_insns_consumed()
                    .unwrap_or(0)
                    .saturating_add(u32::MAX as u64),
                mem,
            );
        }

        // Host::with_storage_and_budget is available in recent versions
        let host = Host::with_storage_and_budget(Storage::default(), budget);

        host.set_diagnostic_level(DiagnosticLevel::Debug)
            .expect("failed to set diagnostic level");

        debug!("simulator host initialized");

        Self {
            inner: host,
            ledger_snapshot: LedgerSnapshot::new(),
            config,
            pending_events: Vec::new(),
            clock: ClockDrift::new(),
        }
    }

    /// Creates a new host initialized with the provided snapshot contents.
    #[instrument(
        level = "debug",
        fields(config = ?config, snapshot_len = snapshot.len())
    )]
    pub fn from_snapshot(
        config: HostConfig,
        snapshot: &LedgerSnapshot,
    ) -> Result<Self, SimHostError> {
        debug!("restoring simulator host from snapshot");
        let budget = Budget::default();

        if let Some((cpu, mem)) = config.budget_limits {
            let _ = budget.reset_limits(cpu, mem);
        } else if let Some(mem) = config.memory_limit {
            let _ = budget.reset_unlimited();
            let _ = budget.reset_limits(
                budget
                    .get_cpu_insns_consumed()
                    .unwrap_or(0)
                    .saturating_add(u32::MAX as u64),
                mem,
            );
        }

        let storage = Self::storage_from_snapshot(snapshot, &budget)?;
        let host = Host::with_storage_and_budget(storage, budget);
        host.set_diagnostic_level(DiagnosticLevel::Debug)?;

        debug!("simulator host restored from snapshot");

        Ok(Self {
            inner: host,
            ledger_snapshot: snapshot.fork(),
            config,
            pending_events: Vec::new(),
            clock: ClockDrift::new(),
        })
    }

    /// Checks whether the current memory consumption exceeds the configured limit.
    ///
    /// # Panics
    ///
    /// Panics with `ERR_MEMORY_LIMIT_EXCEEDED` when the Budget reports more
    /// memory consumed than the configured `memory_limit`.
    pub fn check_memory_limit(&self) {
        if let Some(limit) = self.config.memory_limit {
            if let Ok(consumed) = self.inner.budget_cloned().get_mem_bytes_consumed() {
                memory::check_memory_limit(consumed, limit);
            }
        }
    }

    /// Replaces the current host with a freshly initialized host loaded from the snapshot.
    ///
    /// # Allocator Safety
    ///
    /// This method drops the old [`SimHost`] (including the Soroban `Host` and its
    /// `Budget`) and moves a newly constructed `SimHost` into place.  Rust's standard
    /// drop semantics guarantee that:
    ///
    /// 1. All allocations owned by the old `Host` (Wasm linear memory, storage maps,
    ///    event buffers, etc.) are freed before the move.
    /// 2. The new `Host` starts with a fresh `Budget` whose limits match the original
    ///    `SimHost`'s configuration, ensuring post-rollback memory accounting is
    ///    consistent with the snapshot point.
    /// 3. `Arc`-based sharing in [`LedgerSnapshot`](crate::snapshot::LedgerSnapshot)
    ///    is properly reference-counted, so forked snapshots that are still alive
    ///    elsewhere are not affected.
    #[instrument(level = "debug", fields(snapshot_len = snapshot.len()), skip(self))]
    pub fn restore_from_snapshot(&mut self, snapshot: &LedgerSnapshot) -> Result<(), SimHostError> {
        debug!("restoring current host from snapshot");
        let restored = Self::from_snapshot(self.config.clone(), snapshot)?;
        // `*self = restored` drops the old SimHost (including the old Soroban Host
        // and its Budget) and moves the freshly-constructed SimHost into place.
        // The old Host's Drop impl frees all its allocations — this is the key
        // guarantee that prevents dangling pointers and double-frees after rollback.
        // NOTE: We preserve the clock state across rollback so that time progression
        // continues even when rewinding ledger state.
        let preserved_clock = self.clock.clone();
        // The ledger context belongs to the replay, not to any one snapshot of
        // storage. Entries are injected one at a time and every injection comes
        // through here, so without this a ledger context established before
        // loading state would be discarded by the very first entry and the host
        // would fail execution with "missing ledger info".
        let preserved_ledger_info = self.inner.with_ledger_info(|li| Ok(li.clone())).ok();
        *self = restored;
        self.clock = preserved_clock;
        if let Some(info) = preserved_ledger_info {
            self.inner
                .set_ledger_info(info)
                .map_err(SimHostError::Host)?;
        }
        Ok(())
    }

    /// Captures the current host storage as a reusable ledger snapshot.
    #[instrument(level = "debug", skip(self))]
    pub fn capture_snapshot(&self) -> Result<LedgerSnapshot, SimHostError> {
        debug!(
            snapshot_len = self.ledger_snapshot.len(),
            "capturing ledger snapshot"
        );
        Ok(self.ledger_snapshot.fork())
    }

    /// Returns the host events that have been emitted so far.
    #[instrument(level = "debug", skip(self))]
    pub fn events(&self) -> Result<Events, SimHostError> {
        debug!("fetching host events");
        Ok(self.inner.get_events()?)
    }

    /// Returns the host events as a cloned vector for external history tracking.
    #[instrument(level = "debug", skip(self))]
    pub fn event_log(&self) -> Result<Vec<HostEvent>, SimHostError> {
        debug!("fetching host event log");
        Ok(self.events()?.0)
    }

    /// Stores or replaces a ledger entry by rebuilding the host from the updated snapshot.
    #[instrument(level = "debug", skip(self, entry))]
    pub fn set_ledger_entry(
        &mut self,
        key: soroban_env_host::xdr::LedgerKey,
        entry: soroban_env_host::xdr::LedgerEntry,
    ) -> Result<(), SimHostError> {
        debug!("setting ledger entry");
        let key_bytes = key
            .to_xdr(Limits::none())
            .map_err(|e| SnapshotError::XdrEncoding(format!("Failed to encode key: {e}")))?;
        self.ledger_snapshot.insert(key_bytes, entry);
        let snapshot = self.ledger_snapshot.fork();
        self.restore_from_snapshot(&snapshot)
    }

    #[instrument(level = "debug", fields(snapshot_len = snapshot.len()), skip(budget))]
    fn storage_from_snapshot(
        snapshot: &LedgerSnapshot,
        budget: &Budget,
    ) -> Result<Storage, SimHostError> {
        debug!("building storage from snapshot");
        let mut footprint_map = FootprintMap::new();
        let mut storage_map = StorageMap::new();

        let mut ttl_by_hash = std::collections::HashMap::new();
        for (_, entry) in snapshot.iter() {
            if let soroban_env_host::xdr::LedgerEntryData::Ttl(ref ttl) = entry.data {
                ttl_by_hash.insert(ttl.key_hash.clone(), ttl.live_until_ledger_seq);
            }
        }

        for (key_bytes, entry) in snapshot.iter() {
            let key = Rc::new(crate::snapshot::decode_ledger_key(
                &base64::engine::general_purpose::STANDARD.encode(key_bytes),
            )?);
            // LedgerKey::Ttl is metadata used for TTL tracking and is attached to contract entries,
            // not stored directly in host storage map.
            if matches!(entry.data, soroban_env_host::xdr::LedgerEntryData::Ttl(_))
                || matches!(*key, soroban_env_host::xdr::LedgerKey::Ttl(_))
            {
                continue;
            }
            let live_until = match &entry.data {
                soroban_env_host::xdr::LedgerEntryData::ContractData(_)
                | soroban_env_host::xdr::LedgerEntryData::ContractCode(_) => {
                    crate::state::compute_key_hash(&key)
                        .ok()
                        .and_then(|h| ttl_by_hash.get(&h).copied())
                }
                _ => None,
            };
            footprint_map = footprint_map.insert(Rc::clone(&key), AccessType::ReadWrite, budget)?;
            storage_map =
                storage_map.insert(key, Some((Rc::new(entry.clone()), live_until)), budget)?;
        }

        let storage =
            Storage::with_enforcing_footprint_and_map(Footprint(footprint_map), storage_map);

        debug!("storage built from snapshot");
        Ok(storage)
    }

    /// Set the contract ID for execution context.
    #[allow(dead_code)]
    pub fn set_contract_id(&mut self, _id: Hash) {}

    /// Set the function name to invoke.
    #[allow(dead_code)]
    pub fn set_fn_name(&mut self, _name: &str) -> Result<(), HostError> {
        Ok(())
    }

    /// Convert a u32 to a Soroban Val.
    #[allow(dead_code)]
    pub fn val_from_u32(&self, v: u32) -> Val {
        Val::from_u32(v).into()
    }

    /// Convert a Val back to u32.
    #[allow(dead_code)]
    pub fn val_to_u32(&self, v: Val) -> Result<u32, HostError> {
        v.try_into_val(&self.inner).map_err(|_| {
            EnvError::from_type_and_code(ScErrorType::Value, ScErrorCode::InvalidInput).into()
        })
    }

    /// Buffer a contract event for inclusion in the next snapshot.
    ///
    /// Call this from the simulation loop each time an event is emitted so that
    /// `drain_events_for_snapshot` can associate the right events with each
    /// snapshot window.
    #[allow(dead_code)]
    pub fn push_event(&mut self, event: String) {
        debug!(event = %event, "buffering pending simulator event");
        self.pending_events.push(event);
    }

    /// Return all events buffered since the last snapshot and clear the buffer.
    ///
    /// The returned `Vec` is moved into the `events` field of the `StateSnapshot`
    /// being constructed.  After this call the buffer is empty and ready for the
    /// next snapshot window.
    #[allow(dead_code)]
    #[instrument(level = "debug", skip(self))]
    pub fn drain_events_for_snapshot(&mut self) -> Vec<String> {
        let drained = std::mem::take(&mut self.pending_events);
        debug!(drained = drained.len(), "drained pending simulator events");
        drained
    }

    // ========== Clock Drift API ==========

    /// Advances the simulated ledger clock by the specified number of seconds.
    ///
    /// This is useful for testing time-bound contracts. The clock advancement
    /// is preserved across snapshot capture/restore operations, allowing tests
    /// to advance time during rollback-and-resume scenarios.
    ///
    /// # Arguments
    /// * `seconds` - Number of seconds to advance (can be negative to go backwards)
    ///
    /// # Example
    /// ```ignore
    /// host.advance_clock(3600); // Advance by 1 hour
    /// ```
    #[allow(dead_code)]
    pub fn advance_clock(&mut self, seconds: i64) {
        self.clock.advance_clock(seconds);
        debug!(seconds = seconds, "advanced simulator clock");
    }

    /// Sets the simulated ledger clock to an absolute timestamp.
    ///
    /// This overrides any previous base timestamp and accumulated drift,
    /// useful for jump-to-time test scenarios.
    ///
    /// # Arguments
    /// * `timestamp` - Absolute ledger timestamp in seconds since Unix epoch
    ///
    /// # Example
    /// ```ignore
    /// host.set_clock(1700000000); // Set to Nov 15, 2023
    /// ```
    #[allow(dead_code)]
    pub fn set_clock(&mut self, timestamp: u64) {
        self.clock.set_clock(timestamp);
        debug!(
            timestamp = timestamp,
            "set simulator clock to absolute value"
        );
    }

    /// Returns the current simulated ledger clock timestamp.
    ///
    /// Returns the ledger timestamp in seconds since Unix epoch.
    /// If no base was set, returns system time plus any accumulated drift.
    ///
    /// # Returns
    /// Ledger timestamp in seconds since Unix epoch
    #[allow(dead_code)]
    pub fn get_clock(&self) -> u64 {
        self.clock.get_clock()
    }

    /// Initializes the clock to a specific base timestamp.
    ///
    /// This is typically called during simulator setup to establish a
    /// consistent starting time for test scenarios.
    ///
    /// # Arguments
    /// * `timestamp` - Base ledger timestamp in seconds since Unix epoch
    #[allow(dead_code)]
    pub fn init_clock(&mut self, timestamp: u64) {
        self.clock = ClockDrift::with_timestamp(timestamp);
        debug!(timestamp = timestamp, "initialized simulator clock");
    }

    /// Resets the clock to its initial state (no drift, no override).
    #[allow(dead_code)]
    pub fn reset_clock(&mut self) {
        self.clock.reset();
        debug!("reset simulator clock");
    }

    /// Sets the replay ledger sequence number on the host's ledger info.
    #[allow(dead_code)]
    pub fn set_ledger_sequence(&self, sequence: u32) -> Result<(), SimHostError> {
        self.set_ledger_context(sequence, None, None)
    }

    /// Establishes the ledger a replay runs against.
    ///
    /// The host refuses to execute without ledger info at all ("missing ledger
    /// info"), and refuses a protocol older than its own floor ("ledger
    /// protocol version too old for host"). Defaulting to the protocol this
    /// build of the host understands keeps the default working across host
    /// upgrades, where a pinned number quietly ages out.
    pub fn set_ledger_context(
        &self,
        sequence: u32,
        timestamp: Option<u64>,
        protocol_version: Option<u32>,
    ) -> Result<(), SimHostError> {
        let mut info = self
            .inner
            .with_ledger_info(|li| Ok(li.clone()))
            .unwrap_or_else(|_| soroban_env_host::LedgerInfo {
                protocol_version: host_protocol_version(),
                sequence_number: sequence,
                timestamp: timestamp.unwrap_or_else(|| self.clock.get_clock()),
                network_id: [0u8; 32],
                base_reserve: 10,
                min_temp_entry_ttl: 16,
                min_persistent_entry_ttl: 4096,
                max_entry_ttl: 6_312_000,
            });

        info.sequence_number = sequence;
        if let Some(ts) = timestamp {
            info.timestamp = ts;
        }
        if let Some(version) = protocol_version {
            info.protocol_version = version;
        }

        self.inner.set_ledger_info(info).map_err(SimHostError::Host)
    }

    /// Returns the current simulated ledger sequence, if configured.
    #[allow(dead_code)]
    pub fn get_ledger_sequence(&self) -> Option<u32> {
        self.inner
            .with_ledger_info(|li| Ok(li.sequence_number))
            .ok()
    }
}

impl Drop for SimHost {
    /// Clean up resources when the SimHost is dropped.
    /// This ensures temp files, sockets, and other resources are properly released.
    fn drop(&mut self) {
        // 1. Clear pending events to release any references
        self.pending_events.clear();

        // 2. The inner Host will be dropped automatically, but we can explicitly release resources
        // The Host's storage and budget will be cleaned up when it's dropped

        // 3. Clear the ledger snapshot to release any held references
        // LedgerSnapshot holds Rc pointers that need to be released
        // The fork() method creates clones, so clearing the snapshot helps with memory
        // We use std::mem::take to clear it without causing a double borrow
        let _ = std::mem::take(&mut self.ledger_snapshot);

        // HostConfig contains only configuration values and does not require
        // explicit cleanup.
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::xdr::{
        ContractDataDurability, ContractDataEntry, ContractId, Hash, LedgerEntry, LedgerEntryData,
        LedgerEntryExt, LedgerKey, LedgerKeyContractData, ScAddress, ScVal,
    };
    use soroban_env_host::EnvBase;

    const TIMEOUT_TEST_SLEEP_ENV: &str = "ERST_SIM_TIMEOUT_TEST_SLEEP";

    #[test]
    fn timeout_test_worker() {
        if std::env::var_os(TIMEOUT_TEST_SLEEP_ENV).is_some() {
            loop {
                std::hint::spin_loop();
            }
        }
    }

    #[test]
    fn test_runner_timeout_configuration() {
        assert_eq!(RunnerConfig::default().timeout(), Duration::from_secs(30));
        assert_eq!(
            RunnerConfig::default()
                .with_timeout(Duration::from_millis(250))
                .timeout(),
            Duration::from_millis(250)
        );
    }

    #[test]
    fn test_timed_out_worker_returns_timeout_error() {
        let executable = std::env::current_exe().expect("test executable should be available");
        let mut command = Command::new(executable);
        command
            .args([
                "--exact",
                "runner::tests::timeout_test_worker",
                "--nocapture",
            ])
            .env(TIMEOUT_TEST_SLEEP_ENV, "1");

        let result = run_command_with_timeout(command, b"", Duration::from_millis(100));
        assert!(matches!(
            result,
            Err(SimHostError::Timeout { limit_ms: 100, .. })
        ));
    }

    #[test]
    fn test_host_initialization() {
        let host = SimHost::new(HostConfig::default());
        // Basic assertion that host is functional
        assert!(host.inner.budget_cloned().get_cpu_insns_consumed().is_ok());
    }

    #[test]
    fn test_panic_recovery_wraps_host_execution() {
        let host = SimHost::new(HostConfig::default());
        let result = host.with_panic_recovery(|| panic!("memory violation"));

        assert!(
            matches!(result, Err(SimHostError::Panic(message)) if message.contains("memory violation"))
        );
    }

    #[test]
    fn test_configuration() {
        let mut host = SimHost::new(HostConfig::default());
        // Test setting contract ID (dummy hash)
        let hash = Hash([0u8; 32]);
        host.set_contract_id(hash);

        host.set_fn_name("add")
            .expect("failed to set function name");
    }

    #[test]
    fn test_simple_value_handling() {
        let host = SimHost::new(HostConfig::default());

        let val_a = host.val_from_u32(10);
        let val_b = host.val_from_u32(20);

        let res_a = host.val_to_u32(val_a).expect("conversion failed");
        let res_b = host.val_to_u32(val_b).expect("conversion failed");

        assert_eq!(res_a + res_b, 30);
    }

    #[test]
    fn test_restore_from_snapshot_replaces_mutated_storage_and_clears_host_events() {
        let mut host = SimHost::new(HostConfig::default());
        let first_key = Rc::new(LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([1u8; 32]))),
            key: ScVal::U32(1),
            durability: ContractDataDurability::Persistent,
        }));
        let first_entry = Rc::new(LedgerEntry {
            last_modified_ledger_seq: 1,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([1u8; 32]))),
                key: ScVal::U32(1),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(10),
            }),
            ext: LedgerEntryExt::V0,
        });
        host.set_ledger_entry(first_key.as_ref().clone(), first_entry.as_ref().clone())
            .expect("initial entry should be stored");
        host.inner
            .log_from_slice("before snapshot", &[])
            .expect("diagnostic event should be recorded");

        let snapshot = host.capture_snapshot().expect("snapshot should capture");

        let second_key = Rc::new(LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([2u8; 32]))),
            key: ScVal::U32(2),
            durability: ContractDataDurability::Persistent,
        }));
        let second_entry = Rc::new(LedgerEntry {
            last_modified_ledger_seq: 2,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([2u8; 32]))),
                key: ScVal::U32(2),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(20),
            }),
            ext: LedgerEntryExt::V0,
        });
        host.set_ledger_entry(second_key.as_ref().clone(), second_entry.as_ref().clone())
            .expect("mutated entry should be stored");
        host.inner
            .log_from_slice("after snapshot", &[])
            .expect("later event should be recorded");

        host.restore_from_snapshot(&snapshot)
            .expect("restoring snapshot should succeed");

        let restored = host
            .capture_snapshot()
            .expect("restored snapshot should capture");
        assert_eq!(restored.len(), 1);
        assert!(restored
            .get(&first_key.to_xdr(Limits::none()).unwrap())
            .is_some());
        assert!(restored
            .get(&second_key.to_xdr(Limits::none()).unwrap())
            .is_none());
        assert!(
            host.events().expect("events should read").0.is_empty(),
            "fresh host should not retain post-rollback host events"
        );
    }

    #[test]
    fn test_clock_advance() {
        let mut host = SimHost::new(HostConfig::default());
        host.init_clock(1700000000);
        assert_eq!(host.get_clock(), 1700000000);

        host.advance_clock(3600);
        assert_eq!(host.get_clock(), 1700003600);

        host.advance_clock(3600);
        assert_eq!(host.get_clock(), 1700007200);
    }

    #[test]
    fn test_clock_set_absolute() {
        let mut host = SimHost::new(HostConfig::default());
        host.init_clock(1700000000);
        host.advance_clock(5000);
        assert_eq!(host.get_clock(), 1700005000);

        host.set_clock(1800000000);
        assert_eq!(host.get_clock(), 1800000000);
    }

    #[test]
    fn test_clock_preserved_across_snapshot_restore() {
        let mut host = SimHost::new(HostConfig::default());
        host.init_clock(1700000000);

        let snapshot = host.capture_snapshot().expect("snapshot should capture");

        // Advance clock after snapshot
        host.advance_clock(3600);
        assert_eq!(host.get_clock(), 1700003600);

        // Restore from snapshot (ledger state resets but clock continues)
        host.restore_from_snapshot(&snapshot)
            .expect("restore should succeed");

        // Clock should be preserved with the advancement
        assert_eq!(host.get_clock(), 1700003600);
    }

    #[test]
    fn test_clock_reset() {
        let mut host = SimHost::new(HostConfig::default());
        host.init_clock(1700000000);
        host.advance_clock(5000);
        assert_eq!(host.get_clock(), 1700005000);

        host.reset_clock();
        // After reset, should be system time (approximately now)
        let current = host.get_clock();
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        // Should be within 1 second
        assert!((current as i64 - now as i64).abs() <= 1);
    }
}
