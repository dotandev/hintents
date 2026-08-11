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
use std::rc::Rc;

use crate::memory;
use crate::snapshot::{LedgerSnapshot, SnapshotError};
use tracing::{debug, instrument};

#[derive(Debug, thiserror::Error)]
pub enum SimHostError {
    #[error(transparent)]
    Host(#[from] HostError),
    #[error(transparent)]
    Snapshot(#[from] SnapshotError),
    #[error("host execution panicked: {0}")]
    Panic(String),
}

pub struct SimHost {
    pub inner: Host,
    ledger_snapshot: LedgerSnapshot,
    budget_limits: Option<(u64, u64)>,
    calibration: Option<crate::types::ResourceCalibration>,
    memory_limit: Option<u64>,
    pending_events: Vec<String>,
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
        fields(
            budget_limits = ?budget_limits,
            calibration = ?calibration,
            memory_limit = ?memory_limit
        )
    )]
    pub fn new(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
    ) -> Self {
        debug!("initializing simulator host");
        let budget = Budget::default();

        if let Some((cpu, mem)) = budget_limits {
            let _ = budget.reset_limits(cpu, mem);
        } else if let Some(mem) = memory_limit {
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
            budget_limits,
            calibration,
            memory_limit,
            pending_events: Vec::new(),
        }
    }

    /// Creates a new host initialized with the provided snapshot contents.
    #[instrument(
        level = "debug",
        fields(
            budget_limits = ?budget_limits,
            calibration = ?calibration,
            memory_limit = ?memory_limit,
            snapshot_len = snapshot.len()
        )
    )]
    pub fn from_snapshot(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
        snapshot: &LedgerSnapshot,
    ) -> Result<Self, SimHostError> {
        debug!("restoring simulator host from snapshot");
        let budget = Budget::default();

        if let Some((cpu, mem)) = budget_limits {
            let _ = budget.reset_limits(cpu, mem);
        } else if let Some(mem) = memory_limit {
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
            budget_limits,
            calibration,
            memory_limit,
            pending_events: Vec::new(),
        })
    }

    /// Checks whether the current memory consumption exceeds the configured limit.
    ///
    /// # Panics
    ///
    /// Panics with `ERR_MEMORY_LIMIT_EXCEEDED` when the Budget reports more
    /// memory consumed than the configured `memory_limit`.
    pub fn check_memory_limit(&self) {
        if let Some(limit) = self.memory_limit {
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
        let restored = Self::from_snapshot(
            self.budget_limits,
            self.calibration.clone(),
            self.memory_limit,
            snapshot,
        )?;
        // `*self = restored` drops the old SimHost (including the old Soroban Host
        // and its Budget) and moves the freshly-constructed SimHost into place.
        // The old Host's Drop impl frees all its allocations — this is the key
        // guarantee that prevents dangling pointers and double-frees after rollback.
        *self = restored;
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

        for (key_bytes, entry) in snapshot.iter() {
            let key = Rc::new(crate::snapshot::decode_ledger_key(
                &base64::engine::general_purpose::STANDARD.encode(key_bytes),
            )?);
            footprint_map = footprint_map.insert(Rc::clone(&key), AccessType::ReadWrite, budget)?;
            storage_map = storage_map.insert(key, Some((Rc::new(entry.clone()), None)), budget)?;
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

        // Note: budget_limits, calibration, and memory_limit are simple types
        // that don't require explicit cleanup
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

    #[test]
    fn test_host_initialization() {
        let host = SimHost::new(None, None, None);
        // Basic assertion that host is functional
        assert!(host.inner.budget_cloned().get_cpu_insns_consumed().is_ok());
    }

    #[test]
    fn test_panic_recovery_wraps_host_execution() {
        let host = SimHost::new(None, None, None);
        let result = host.with_panic_recovery(|| panic!("memory violation"));

        assert!(
            matches!(result, Err(SimHostError::Panic(message)) if message.contains("memory violation"))
        );
    }

    #[test]
    fn test_configuration() {
        let mut host = SimHost::new(None, None, None);
        // Test setting contract ID (dummy hash)
        let hash = Hash([0u8; 32]);
        host.set_contract_id(hash);

        host.set_fn_name("add")
            .expect("failed to set function name");
    }

    #[test]
    fn test_simple_value_handling() {
        let host = SimHost::new(None, None, None);

        let val_a = host.val_from_u32(10);
        let val_b = host.val_from_u32(20);

        let res_a = host.val_to_u32(val_a).expect("conversion failed");
        let res_b = host.val_to_u32(val_b).expect("conversion failed");

        assert_eq!(res_a + res_b, 30);
    }

    #[test]
    fn test_restore_from_snapshot_replaces_mutated_storage_and_clears_host_events() {
        let mut host = SimHost::new(None, None, None);
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
}
