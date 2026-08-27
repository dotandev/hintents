// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use serde::{Deserialize, Serialize};
use soroban_env_host::{budget::Budget, xdr::ContractCostType, HostError};
use std::collections::HashMap;

/// Detailed breakdown of execution costs by operation type
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct CostMetrics {
    /// Total CPU instructions consumed for this operation
    pub cpu: u64,
    /// Total memory bytes allocated for this operation
    pub mem: u64,
    /// Number of times this operation was invoked
    pub iterations: u64,
}

/// A comprehensive collection of metrics for all recorded operations
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ExecutionMetrics {
    /// Distribution of instructions and costs per component
    pub cost_distribution: HashMap<String, CostMetrics>,

    /// Total CPU instructions consumed globally
    pub total_cpu_insns: u64,

    /// Total memory bytes consumed globally
    pub total_mem_bytes: u64,
}

pub fn extract_metrics(budget: &Budget) -> Result<ExecutionMetrics, HostError> {
    let mut cost_distribution = HashMap::new();

    for ct in ContractCostType::variants() {
        if let Ok(tracker) = budget.get_tracker(ct) {
            if tracker.iterations > 0 || tracker.cpu > 0 || tracker.mem > 0 {
                let name = format!("{:?}", ct);
                cost_distribution.insert(
                    name,
                    CostMetrics {
                        cpu: tracker.cpu,
                        mem: tracker.mem,
                        iterations: tracker.iterations,
                    },
                );
            }
        }
    }

    Ok(ExecutionMetrics {
        cost_distribution,
        total_cpu_insns: budget.get_cpu_insns_consumed()?,
        total_mem_bytes: budget.get_mem_bytes_consumed()?,
    })
}
