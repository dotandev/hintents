// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use soroban_env_host::xdr::{LedgerEntry, LedgerEntryChange};
use std::collections::HashMap;

fn merge_storage_state(before: &[LedgerEntry], changes: &[LedgerEntryChange]) -> Vec<LedgerEntry> {
    let mut state: HashMap<String, LedgerEntry> = HashMap::new();

    // Load BEFORE state
    for entry in before {
        state.insert(format!("{:?}", entry.data), entry.clone());
    }

    // Apply ResultMeta changes
    for change in changes {
        match change {
            LedgerEntryChange::Created(e) | LedgerEntryChange::Updated(e) => {
                state.insert(format!("{:?}", e.data), e.clone());
            }
            LedgerEntryChange::Removed(key) => {
                state.remove(&format!("{:?}", key));
            }
            _ => {}
        }
    }

    state.into_values().collect()
}
