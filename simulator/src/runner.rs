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
}

#[allow(dead_code)]
impl SimHost {
    /// Initialize a new Host with optional budget settings and resource calibration.
    pub fn new(budget_limits: Option<(u64, u64)>, calibration: Option<crate::types::ResourceCalibration>) -> Self {
        let budget = Budget::default();
        
        if let Some(calib) = calibration {
            use soroban_env_host::budget::CostModel;
            use soroban_env_host::xdr::ContractCostType;

            // SHA256
            let sha256_model = CostModel {
                const_term: calib.sha256_fixed as i64,
                linear_term: calib.sha256_per_byte as i64,
            };
            let _ = budget.set_model(ContractCostType::ComputeSha256Hash, sha256_model);

            // Keccak256
            let keccak256_model = CostModel {
                const_term: calib.keccak256_fixed as i64,
                linear_term: calib.keccak256_per_byte as i64,
            };
            let _ = budget.set_model(ContractCostType::ComputeKeccak256Hash, keccak256_model);

            // Ed25519
            let ed25519_model = CostModel {
                const_term: calib.ed25519_fixed as i64,
                linear_term: 0,
            };
            let _ = budget.set_model(ContractCostType::VerifyEd25519Sig, ed25519_model);
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
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_host_initialization() {
        let host = SimHost::new(None, None);
        // Basic assertion that host is functional
        assert!(host.inner.budget_cloned().get_cpu_insns_consumed().is_ok());
    }

    #[test]
    fn test_configuration() {
        let mut host = SimHost::new(None, None);
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
        let host = SimHost::new(None, None);

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
}
