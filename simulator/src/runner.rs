// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine;
use soroban_env_host::{
    budget::Budget,
    events::{Events, HostEvent},
    storage::{AccessType, Footprint, FootprintMap, Storage, StorageMap},
    xdr::{Hash, Limits, ScErrorCode, ScErrorType, WriteXdr},
    DiagnosticLevel, Error as EnvError, Host, HostError, TryIntoVal, Val,
};
use std::rc::Rc;
use std::sync::atomic::{AtomicU64, Ordering};

use crate::snapshot::{LedgerSnapshot, SnapshotError};

/// Process-wide count of memory (in bytes) currently reserved by active
/// [`SimHost`] instances that were constructed with an explicit
/// `memory_limit`.
///
/// Each [`SimHost`]'s own `Budget` already enforces that *that one*
/// simulation stays within its own `memory_limit`, but that check is purely
/// per-instance: if several simulations run concurrently (e.g. one per
/// simulation thread/connection), each can individually be within its own
/// budget while their combined memory footprint still overwhelms the
/// process. This atomic counter tracks that aggregate so a global ceiling
/// (see [`set_global_memory_ceiling`]) can be enforced across concurrently
/// active hosts, regardless of which thread constructs or drops them.
static GLOBAL_RESERVED_MEMORY: AtomicU64 = AtomicU64::new(0);

/// Process-wide memory ceiling (in bytes) shared across all concurrently
/// active [`SimHost`] instances. `u64::MAX` (the default) means no global
/// ceiling is enforced, only each host's own `memory_limit`.
static GLOBAL_MEMORY_CEILING: AtomicU64 = AtomicU64::new(u64::MAX);

/// Sets the process-wide memory ceiling (in bytes) shared across all
/// concurrently active [`SimHost`] instances. Pass `None` to disable the
/// global ceiling (the default), leaving only each host's own
/// `memory_limit` in effect.
#[allow(dead_code)]
pub fn set_global_memory_ceiling(ceiling: Option<u64>) {
    GLOBAL_MEMORY_CEILING.store(ceiling.unwrap_or(u64::MAX), Ordering::SeqCst);
}

/// Returns the total memory (bytes) currently reserved across all active
/// [`SimHost`] instances that were constructed with a `memory_limit`.
///
/// This is a snapshot: under concurrent construction/drop of hosts on other
/// threads it can change immediately after being read, same as any other
/// atomic counter used for monitoring rather than mutual exclusion.
#[allow(dead_code)]
pub fn global_reserved_memory() -> u64 {
    GLOBAL_RESERVED_MEMORY.load(Ordering::SeqCst)
}

/// Attempts to reserve `bytes` against the global memory ceiling.
///
/// On success, `GLOBAL_RESERVED_MEMORY` has been incremented by `bytes` and
/// the caller is responsible for releasing it later (see
/// [`release_global_memory_reservation`]), typically via `Drop`. On failure,
/// no reservation is retained: the attempted increment is rolled back before
/// returning, so a rejected host doesn't leave a phantom reservation behind.
fn try_reserve_global_memory(bytes: u64) -> Result<(), (u64, u64)> {
    let reserved_after = GLOBAL_RESERVED_MEMORY.fetch_add(bytes, Ordering::SeqCst) + bytes;
    let ceiling = GLOBAL_MEMORY_CEILING.load(Ordering::SeqCst);
    if reserved_after > ceiling {
        GLOBAL_RESERVED_MEMORY.fetch_sub(bytes, Ordering::SeqCst);
        return Err((reserved_after, ceiling));
    }
    Ok(())
}

/// Releases a previously successful reservation made via
/// [`try_reserve_global_memory`].
fn release_global_memory_reservation(bytes: u64) {
    GLOBAL_RESERVED_MEMORY.fetch_sub(bytes, Ordering::SeqCst);
}

#[derive(Debug, thiserror::Error)]
pub enum SimHostError {
    #[error(transparent)]
    Host(#[from] HostError),
    #[error(transparent)]
    Snapshot(#[from] SnapshotError),
    #[error(
        "global memory limit exceeded: reserving this host would bring concurrently \
         reserved memory to {reserved} bytes, exceeding the {ceiling} byte ceiling"
    )]
    GlobalMemoryLimitExceeded { reserved: u64, ceiling: u64 },
}

pub struct SimHost {
    pub inner: Host,
    ledger_snapshot: LedgerSnapshot,
    budget_limits: Option<(u64, u64)>,
    calibration: Option<crate::types::ResourceCalibration>,
    pub(crate) memory_limit: Option<u64>,
    pending_events: Vec<String>,
}

impl SimHost {
    /// Initialize a new Host with optional budget settings and resource calibration.
    pub fn new(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
    ) -> Self {
        if let Some(mem) = memory_limit {
            if let Err((reserved, ceiling)) = try_reserve_global_memory(mem) {
                panic!(
                    "ERR_GLOBAL_MEMORY_LIMIT_EXCEEDED: reserving this host would bring \
                     concurrently reserved memory to {reserved} bytes, exceeding the \
                     {ceiling} byte ceiling"
                );
            }
        }

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
    pub fn from_snapshot(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
        snapshot: &LedgerSnapshot,
    ) -> Result<Self, SimHostError> {
        if let Some(mem) = memory_limit {
            if let Err((reserved, ceiling)) = try_reserve_global_memory(mem) {
                return Err(SimHostError::GlobalMemoryLimitExceeded { reserved, ceiling });
            }
        }

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

        let storage = match Self::storage_from_snapshot(snapshot, &budget) {
            Ok(storage) => storage,
            Err(e) => {
                if let Some(mem) = memory_limit {
                    release_global_memory_reservation(mem);
                }
                return Err(e);
            }
        };
        let host = Host::with_storage_and_budget(storage, budget);
        if let Err(e) = host.set_diagnostic_level(DiagnosticLevel::Debug) {
            if let Some(mem) = memory_limit {
                release_global_memory_reservation(mem);
            }
            return Err(e.into());
        }

        Ok(Self {
            inner: host,
            ledger_snapshot: snapshot.fork(),
            budget_limits,
            calibration,
            memory_limit,
            pending_events: Vec::new(),
        })
    }

    /// Replaces the current host with a freshly initialized host loaded from the snapshot.
    pub fn restore_from_snapshot(&mut self, snapshot: &LedgerSnapshot) -> Result<(), SimHostError> {
        let restored = Self::from_snapshot(
            self.budget_limits,
            self.calibration.clone(),
            self.memory_limit,
            snapshot,
        )?;
        *self = restored;
        Ok(())
    }

    /// Captures the current host storage as a reusable ledger snapshot.
    pub fn capture_snapshot(&self) -> Result<LedgerSnapshot, SimHostError> {
        Ok(self.ledger_snapshot.fork())
    }

    /// Returns the host events that have been emitted so far.
    pub fn events(&self) -> Result<Events, SimHostError> {
        Ok(self.inner.get_events()?)
    }

    /// Returns the host events as a cloned vector for external history tracking.
    pub fn event_log(&self) -> Result<Vec<HostEvent>, SimHostError> {
        Ok(self.events()?.0)
    }

    /// Stores or replaces a ledger entry by rebuilding the host from the updated snapshot.
    pub fn set_ledger_entry(
        &mut self,
        key: soroban_env_host::xdr::LedgerKey,
        entry: soroban_env_host::xdr::LedgerEntry,
    ) -> Result<(), SimHostError> {
        let key_bytes = key
            .to_xdr(Limits::none())
            .map_err(|e| SnapshotError::XdrEncoding(format!("Failed to encode key: {e}")))?;
        self.ledger_snapshot.insert(key_bytes, entry);
        let snapshot = self.ledger_snapshot.fork();
        self.restore_from_snapshot(&snapshot)
    }

    fn storage_from_snapshot(
        snapshot: &LedgerSnapshot,
        budget: &Budget,
    ) -> Result<Storage, SimHostError> {
        let mut footprint_map = FootprintMap::new();
        let mut storage_map = StorageMap::new();

        for (key_bytes, entry) in snapshot.iter() {
            let key = Rc::new(crate::snapshot::decode_ledger_key(
                &base64::engine::general_purpose::STANDARD.encode(key_bytes),
            )?);
            footprint_map = footprint_map.insert(Rc::clone(&key), AccessType::ReadWrite, budget)?;
            storage_map = storage_map.insert(key, Some((Rc::new(entry.clone()), None)), budget)?;
        }

        Ok(Storage::with_enforcing_footprint_and_map(
            Footprint(footprint_map),
            storage_map,
        ))
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
        self.pending_events.push(event);
    }

    /// Return all events buffered since the last snapshot and clear the buffer.
    ///
    /// The returned `Vec` is moved into the `events` field of the `StateSnapshot`
    /// being constructed.  After this call the buffer is empty and ready for the
    /// next snapshot window.
    #[allow(dead_code)]
    pub fn drain_events_for_snapshot(&mut self) -> Vec<String> {
        std::mem::take(&mut self.pending_events)
    }

    /// Checks this host's own memory consumption against its per-instance
    /// `memory_limit`, panicking if it has been exceeded.
    ///
    /// This only enforces *this* host's individual budget. It does not by
    /// itself account for other concurrently active hosts - that aggregate
    /// tracking happens separately via the global reservation taken in
    /// [`SimHost::new`]/[`SimHost::from_snapshot`] and released on
    /// [`Drop`].
    #[allow(dead_code)]
    pub fn check_memory_limit(&self) {
        if let Some(limit) = self.memory_limit {
            if let Ok(mem_bytes) = self.inner.budget_cloned().get_mem_bytes_consumed() {
                if mem_bytes > limit {
                    panic!(
                        "ERR_MEMORY_LIMIT_EXCEEDED: consumed {mem_bytes} bytes, limit {limit} bytes"
                    );
                }
            }
        }
    }
}

impl Drop for SimHost {
    fn drop(&mut self) {
        if let Some(mem) = self.memory_limit {
            release_global_memory_reservation(mem);
        }
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

    // Tests below manipulate the process-wide GLOBAL_MEMORY_CEILING, so they
    // serialize against each other via this lock. Tests elsewhere in this
    // module (and other test modules) that never touch the global ceiling
    // are unaffected and continue to run in parallel as normal.
    static GLOBAL_CEILING_TEST_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());

    #[test]
    fn global_memory_reservation_tracks_construction_and_drop() {
        let _guard = GLOBAL_CEILING_TEST_LOCK.lock().unwrap();
        set_global_memory_ceiling(None);

        let before = global_reserved_memory();
        {
            let _host = SimHost::new(None, None, Some(4096));
            assert_eq!(global_reserved_memory(), before + 4096);
        }
        // The host has been dropped; its reservation should be released.
        assert_eq!(global_reserved_memory(), before);
    }

    #[test]
    fn global_memory_ceiling_rejects_from_snapshot_when_exceeded() {
        let _guard = GLOBAL_CEILING_TEST_LOCK.lock().unwrap();
        let current = global_reserved_memory();
        set_global_memory_ceiling(Some(current + 10));

        let snapshot = LedgerSnapshot::new();
        let result = SimHost::from_snapshot(None, None, Some(1_000_000), &snapshot);

        set_global_memory_ceiling(None);

        assert!(matches!(
            result,
            Err(SimHostError::GlobalMemoryLimitExceeded { .. })
        ));
        // A rejected reservation must not leak: total reserved memory
        // should be unchanged from before the attempt.
        assert_eq!(global_reserved_memory(), current);
    }

    #[test]
    fn global_memory_ceiling_panics_in_new_when_exceeded() {
        let _guard = GLOBAL_CEILING_TEST_LOCK.lock().unwrap();
        let current = global_reserved_memory();
        set_global_memory_ceiling(Some(current + 10));

        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            SimHost::new(None, None, Some(1_000_000))
        }));

        set_global_memory_ceiling(None);

        assert!(
            result.is_err(),
            "expected SimHost::new to panic when the global ceiling would be exceeded"
        );
        // The panicking construction must not leave a phantom reservation behind.
        assert_eq!(global_reserved_memory(), current);
    }

    #[test]
    fn concurrent_hosts_share_the_global_reservation_counter() {
        let _guard = GLOBAL_CEILING_TEST_LOCK.lock().unwrap();
        set_global_memory_ceiling(None);

        let before = global_reserved_memory();
        let per_host_bytes: u64 = 2048;
        let thread_count = 8;

        let handles: Vec<_> = (0..thread_count)
            .map(|_| {
                std::thread::spawn(move || {
                    let host = SimHost::new(None, None, Some(per_host_bytes));
                    // Hold the host briefly so all threads' reservations
                    // are simultaneously live at some point.
                    std::thread::sleep(std::time::Duration::from_millis(20));
                    drop(host);
                })
            })
            .collect();

        for handle in handles {
            handle.join().expect("host thread should not panic");
        }

        // All threads have finished and dropped their hosts, so the
        // aggregate reservation should be back to exactly where it
        // started, regardless of how the increments/decrements from each
        // thread interleaved.
        assert_eq!(global_reserved_memory(), before);
    }
}
