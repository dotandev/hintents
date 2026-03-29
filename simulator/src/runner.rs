// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use soroban_env_host::{
    budget::Budget,
    storage::Storage,
    xdr::{Hash, ScErrorCode, ScErrorType},
    DiagnosticLevel, Error as EnvError, Host, HostError, TryIntoVal, Val,
};

/// Wrapper around the Soroban Host to manage initialization and execution context.
pub struct SimHost {
    pub inner: Host,
    /// Events buffered since the last call to `_drain_events_for_snapshot`.
    _pending_events: Vec<String>,
}

impl SimHost {
    /// Initialize a new Host with optional budget settings and resource calibration.
    pub fn new(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        _memory_limit: Option<u64>,
    ) -> Self {
        let budget = Budget::default();

        if let Some(_calib) = calibration {
            // Note: In newer versions of soroban_env_host, the Budget interface
            // no longer uses set_model() or CostModel directly like this.
            // Resource calibration settings from the request are ignored
            // in this simulator version to maintain compatibility with the SDK.
        }

        if let Some((_cpu, _mem)) = budget_limits {
            // Budget customization requires testutils feature or extended API
            // Using default mainnet budget settings
        }

        // Host::with_storage_and_budget is available in recent versions
        let host = Host::with_storage_and_budget(Storage::default(), budget);

        host.set_diagnostic_level(DiagnosticLevel::Debug)
            .expect("failed to set diagnostic level");

        Self {
            inner: host,
            _pending_events: Vec::new(),
        }
    }

    /// Set the contract ID for execution context.
    pub fn _set_contract_id(&mut self, _id: Hash) {}

    /// Set the function name to invoke.
    pub fn _set_fn_name(&mut self, _name: &str) -> Result<(), HostError> {
        Ok(())
    }

    /// Convert a u32 to a Soroban Val.
    pub fn _val_from_u32(&self, v: u32) -> Val {
        Val::from_u32(v).into()
    }

    /// Convert a Val back to u32.
    pub fn _val_to_u32(&self, v: Val) -> Result<u32, HostError> {
        v.try_into_val(&self.inner).map_err(|_| {
            EnvError::from_type_and_code(ScErrorType::Context, ScErrorCode::InvalidInput).into()
        })
    }

    /// Buffer a contract event for inclusion in the next snapshot.
    ///
    /// Call this from the simulation loop each time an event is emitted so that
    /// `_drain_events_for_snapshot` can associate the right events with each
    /// snapshot window.
    pub fn _push_event(&mut self, event: String) {
        self._pending_events.push(event);
    }

    /// Return all events buffered since the last snapshot and clear the buffer.
    ///
    /// The returned `Vec` is moved into the `events` field of the `StateSnapshot`
    /// being constructed.  After this call the buffer is empty and ready for the
    /// next snapshot window.
    pub fn _drain_events_for_snapshot(&mut self) -> Vec<String> {
        std::mem::take(&mut self._pending_events)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
        host._set_contract_id(hash);

        host._set_fn_name("add")
            .expect("failed to set function name");
    }

    #[test]
    fn test_simple_value_handling() {
        let host = SimHost::new(None, None, None);

        let val_a = host._val_from_u32(10);
        let val_b = host._val_from_u32(20);

        let res_a = host._val_to_u32(val_a).expect("conversion failed");
        let res_b = host._val_to_u32(val_b).expect("conversion failed");

        assert_eq!(res_a + res_b, 30);
    }

    #[test]
    fn test_drain_events_for_snapshot_returns_buffered_events() {
        let mut host = SimHost::new(None, None, None);
        host._push_event("event_a".to_string());
        host._push_event("event_b".to_string());

        let drained = host._drain_events_for_snapshot();
        assert_eq!(drained, vec!["event_a", "event_b"]);
    }

    #[test]
    fn test_drain_events_for_snapshot_clears_buffer() {
        let mut host = SimHost::new(None, None, None);
        host._push_event("event_a".to_string());
        let _ = host._drain_events_for_snapshot();

        let second_drain = host._drain_events_for_snapshot();
        assert!(second_drain.is_empty());
    }

    #[test]
    fn test_drain_events_for_snapshot_empty_buffer() {
        let mut host = SimHost::new(None, None, None);
        let drained = host._drain_events_for_snapshot();
        assert!(drained.is_empty());
    }
}
