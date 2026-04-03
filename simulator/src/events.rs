// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::types::SnapshotMetadata;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

static SNAPSHOT_COUNTER: AtomicU64 = AtomicU64::new(1);

pub fn generate_snapshot_id() -> String {
    let ts_micros = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_micros() as u64)
        .unwrap_or(0);
    let counter = SNAPSHOT_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("snap-{ts_micros:016x}-{counter:08x}")
}

pub fn build_snapshot_metadata(gas_consumed: u64, call_stack_depth: u32) -> SnapshotMetadata {
    SnapshotMetadata {
        id: generate_snapshot_id(),
        gas_consumed,
        call_stack_depth,
    }
}
