use serde::Deserialize;

use crate::error::AppError;

/// Client responsible for communicating with the Stellar Horizon API.
#[derive(Clone, Debug)]
pub struct HorizonClient {
    base_url: String,
}

impl HorizonClient {
    /// Create a new Horizon client.
    pub fn new(base_url: impl Into<String>) -> Self {
        Self {
            base_url: base_url.into(),
        }
    }

    /// Get the base URL of the Horizon server.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// Fetch the latest transaction from Horizon.
    ///
    /// NOTE:
    /// - Networking is added in a later issue
    /// - This stub prevents accidental Horizon usage early
    pub fn fetch_latest_transaction(
        &self,
    ) -> Result<HorizonTransaction, AppError> {
        Err(AppError::Network(
            "Horizon client not implemented yet".into(),
        ))
    }

    /// Fetch operations for a given transaction hash.
    pub fn fetch_operations(
        &self,
        _tx_hash: &str,
    ) -> Result<Vec<HorizonOperation>, AppError> {
        Err(AppError::Network(
            "Horizon client not implemented yet".into(),
        ))
    }
}

/// Represents a transaction returned by Horizon.
#[derive(Debug, Deserialize)]
pub struct HorizonTransaction {
    pub hash: String,
    pub successful: bool,
    pub fee_charged: String,
}

/// Represents an operation within a transaction.
#[derive(Debug, Deserialize)]
pub struct HorizonOperation {
    #[serde(rename = "type")]
    pub op_type: String,

    pub from: Option<String>,
    pub to: Option<String>,

    pub asset_type: Option<String>,
    pub asset_code: Option<String>,
    pub asset_issuer: Option<String>,

    pub amount: Option<String>,
}
