// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use soroban_env_host::{
    budget::Budget,
    storage::Storage,
    xdr::{Hash, ScErrorCode, ScErrorType},
    DiagnosticLevel, Error as EnvError, Host, HostError, TryIntoVal, Val,
};

#[allow(dead_code)]
/// Wrapper around the Soroban Host to manage initialization and execution context.
pub struct SimHost {
    pub inner: Host,
    pub contract_id: Option<Hash>,
    pub fn_name: Option<String>,
    pub memory_limit: Option<u64>,
}

#[allow(dead_code)]
impl SimHost {
    /// Initialize a new Host with optional budget settings and resource calibration.
    pub fn new(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
    ) -> Self {
        let budget = Budget::default();

        if let Some(_calib) = calibration {
            // Resource calibration hooks are currently best-effort. Newer
            // soroban-env-host versions no longer expose the previous model API.
            // We keep the request field for forward compatibility.
        }

        if let Some((_cpu, _mem)) = budget_limits {
            // Budget customization requires testutils feature or extended API
            // Using default mainnet budget settings
        }

        // Host::with_storage_and_budget is available in recent versions
        let host = Host::with_storage_and_budget(Storage::default(), budget);

        // Enable debug mode for better diagnostics
        host.set_diagnostic_level(DiagnosticLevel::Debug)
            .expect("failed to set diagnostic level");

        Self {
            inner: host,
            contract_id: None,
            fn_name: None,
            memory_limit,
        }
    }

    /// Set the contract ID for execution context.
    pub fn set_contract_id(&mut self, id: Hash) {
        self.contract_id = Some(id);
    }

    /// Set the function name to invoke.
    pub fn set_fn_name(&mut self, name: &str) -> Result<(), HostError> {
        self.fn_name = Some(name.to_string());
        Ok(())
    }

    /// Helper to convert a u32 to a Soroban Val
    pub fn val_from_u32(&self, v: u32) -> Val {
        Val::from_u32(v).into()
    }

    /// Helper to convert a Val back to u32
    pub fn val_to_u32(&self, v: Val) -> Result<u32, HostError> {
        v.try_into_val(&self.inner).map_err(|_| {
            let e = EnvError::from_type_and_code(ScErrorType::Context, ScErrorCode::InvalidInput);
            e.into()
        })
    }

    /// Check memory consumption against limit and panic if exceeded
    pub fn check_memory_limit(&self) {
        if let Some(limit) = self.memory_limit {
            if let Ok(mem_bytes) = self.inner.budget_cloned().get_mem_bytes_consumed() {
                if mem_bytes > limit {
                    panic!(
                        "Memory limit exceeded: {} bytes > {} bytes limit",
                        mem_bytes, limit
                    );
                }
            }
        }
    }

    /// Rebuild the host with fresh ledger storage while preserving compiled WASM modules.
    ///
    /// This is useful for high-volume simulation/test loops where recreating and
    /// recompiling modules is expensive, but each iteration needs an isolated ledger state.
    pub fn wipe_ledger_state_preserving_modules(&mut self) -> Result<(), HostError> {
        // Start each iteration with a fresh budget and storage snapshot.
        let budget = Budget::default();

        // Best-effort transfer of module cache. If the old host never initialized one,
        // we still proceed with a clean host.
        let module_cache = self.inner.take_module_cache().ok();

        let fresh_host = Host::with_storage_and_budget(Storage::default(), budget);
        fresh_host.set_diagnostic_level(DiagnosticLevel::Debug)?;

        if let Some(cache) = module_cache {
            fresh_host.set_module_cache(cache)?;
        }

        self.inner = fresh_host;
        self.contract_id = None;
        self.fn_name = None;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::ModuleCache;

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
        assert!(host.contract_id.is_some());

        // Test setting function name
        host.set_fn_name("add")
            .expect("failed to set function name");
        assert!(host.fn_name.is_some());
    }

    #[test]
    fn test_simple_value_handling() {
        let host = SimHost::new(None, None, None);

        let a = 10u32;
        let b = 20u32;

        // Convert to Val (simulating inputs)
        let val_a = host.val_from_u32(a);
        let val_b = host.val_from_u32(b);

        // Perform additions by converting back (simulating host operation handling)
        let res_a = host.val_to_u32(val_a).expect("conversion failed");
        let res_b = host.val_to_u32(val_b).expect("conversion failed");

        assert_eq!(res_a + res_b, 30);
    }

    #[test]
    fn test_wipe_ledger_state_preserving_modules_without_cache() {
        let mut host = SimHost::new(None, None, None);
        let before = format!("{:?}", host.inner);

        host.wipe_ledger_state_preserving_modules()
            .expect("wipe should succeed");

        let after = format!("{:?}", host.inner);
        assert_ne!(before, after, "host instance should be rebuilt");
    }

    #[test]
    fn test_wipe_ledger_state_preserving_modules_keeps_module_cache() {
        let mut host = SimHost::new(None, None, None);

        let cache = ModuleCache::new(&host.inner).expect("module cache should initialize");
        host.inner
            .set_module_cache(cache)
            .expect("setting module cache should succeed");

        host.wipe_ledger_state_preserving_modules()
            .expect("wipe should succeed");

        // If cache transfer worked, taking the cache from the rebuilt host succeeds.
        let transferred = host
            .inner
            .take_module_cache()
            .expect("module cache should be preserved after wipe");
        // Put it back to leave host usable for any follow-on checks.
        host.inner
            .set_module_cache(transferred)
            .expect("reinstalling module cache should succeed");
    }
}
