// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// AnomalyType represents the mathematical or logic violations detected
/// by the AssetTracker during simulation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum AnomalyType {
    /// Token was created out of thin air (without explicit mint authorization)
    UnauthorizedMint,
    /// Token balance decreased without a valid transfer or burn event
    BalanceLeak,
    /// Double-spending detected (spend exceeds tracked balance)
    DoubleSpend,
}

/// AssetAnomaly represents a Move-level safety violation detected in Rust code.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AssetAnomaly {
    pub anomaly_type: AnomalyType,
    pub contract_id: String,
    pub amount: i64,
    pub message: String,
}

/// AssetTracker simulates Move's "resource safety" linear types by intercepting
/// and balancing token flows mathematically.
pub struct AssetTracker {
    enabled: bool,
    /// Map of Address -> Balance
    balances: HashMap<String, i64>,
    anomalies: Vec<AssetAnomaly>,
}

impl AssetTracker {
    pub fn new(enabled: bool) -> Self {
        Self {
            enabled,
            balances: HashMap::new(),
            anomalies: Vec::new(),
        }
    }

    /// Record a transfer of assets.
    pub fn record_transfer(&mut self, from: &str, to: &str, amount: i64) {
        if !self.enabled {
            return;
        }

        let from_bal = self.balances.entry(from.to_string()).or_insert(0);
        if *from_bal < amount {
            self.anomalies.push(AssetAnomaly {
                anomaly_type: AnomalyType::DoubleSpend,
                contract_id: from.to_string(),
                amount,
                message: format!(
                    "Double spend detected: {} attempted to transfer {} but only had {}",
                    from, amount, from_bal
                ),
            });
        }
        *from_bal -= amount;

        let to_bal = self.balances.entry(to.to_string()).or_insert(0);
        *to_bal += amount;
    }

    /// Record a minting of assets.
    pub fn record_mint(&mut self, to: &str, amount: i64, authorized: bool) {
        if !self.enabled {
            return;
        }

        if !authorized {
            self.anomalies.push(AssetAnomaly {
                anomaly_type: AnomalyType::UnauthorizedMint,
                contract_id: to.to_string(),
                amount,
                message: format!(
                    "Unauthorized mint detected: {} received {} without valid authorization",
                    to, amount
                ),
            });
        }

        let to_bal = self.balances.entry(to.to_string()).or_insert(0);
        *to_bal += amount;
    }

    /// Record a burning of assets.
    pub fn record_burn(&mut self, from: &str, amount: i64) {
        if !self.enabled {
            return;
        }

        let from_bal = self.balances.entry(from.to_string()).or_insert(0);
        if *from_bal < amount {
            self.anomalies.push(AssetAnomaly {
                anomaly_type: AnomalyType::DoubleSpend,
                contract_id: from.to_string(),
                amount,
                message: format!(
                    "Burn exceeded balance: {} attempted to burn {} but only had {}",
                    from, amount, from_bal
                ),
            });
        }
        *from_bal -= amount;
    }

    /// Consolidate and return anomalies at the end of the simulation block.
    pub fn finalize(&mut self) -> Vec<AssetAnomaly> {
        self.anomalies.clone()
    }
}
