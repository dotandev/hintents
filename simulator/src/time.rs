// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Mock ledger clock for testing time-bound contracts.
//!
//! This module provides a [`ClockDrift`] system that allows tests to artificially
//! advance or manipulate the simulator's internal ledger clock, enabling tests of
//! contracts that depend on precise ledger timestamps.
//!
//! # Usage
//!
//! ```ignore
//! let mut host = SimHost::new(HostConfig::default());
//! host.advance_clock(3600); // Advance by 1 hour (3600 seconds)
//! host.set_clock(1700000000); // Set to absolute timestamp
//! let current = host.get_clock(); // Get current ledger timestamp
//! ```
//!
//! Clock state is preserved across snapshot capture and restore operations,
//! ensuring consistent time progression during rollback-and-resume scenarios.

use std::time::{SystemTime, UNIX_EPOCH};

/// Represents mock time state within the simulator.
///
/// `ClockDrift` tracks both an optional base timestamp and cumulative time
/// advances, allowing tests to control ledger time independently of wall-clock time.
///
/// # Snapshot Safety
///
/// When a [`crate::runner::SimHost`] is restored from a snapshot via
/// [`crate::runner::SimHost::restore_from_snapshot`], the `ClockDrift` is
/// NOT automatically reset—only the ledger state is restored. This allows
/// replaying from a checkpoint while maintaining clock continuity.
#[derive(Debug, Clone)]
pub struct ClockDrift {
    /// Optional base timestamp in seconds since Unix epoch.
    /// If `None`, the clock starts at the system time when the simulator was created.
    /// If `Some(ts)`, the clock starts at that timestamp.
    base_timestamp: Option<u64>,
    /// Cumulative seconds added to the base timestamp via [`advance_clock`].
    accumulated_drift: i64,
    /// If set, overrides both base and drift and returns this absolute timestamp.
    absolute_override: Option<u64>,
}

impl ClockDrift {
    /// Creates a new `ClockDrift` with no base set (defaults to system time when first queried).
    pub fn new() -> Self {
        Self {
            base_timestamp: None,
            accumulated_drift: 0,
            absolute_override: None,
        }
    }

    /// Creates a `ClockDrift` initialized to a specific timestamp.
    ///
    /// # Arguments
    /// * `timestamp` - Ledger timestamp in seconds since Unix epoch
    #[allow(dead_code)]
    pub fn with_timestamp(timestamp: u64) -> Self {
        Self {
            base_timestamp: Some(timestamp),
            accumulated_drift: 0,
            absolute_override: None,
        }
    }

    /// Advances the clock by the specified number of seconds.
    ///
    /// This adds to the accumulated drift, which is applied on top of the
    /// base timestamp (or system time if no base is set).
    ///
    /// # Arguments
    /// * `seconds` - Number of seconds to advance (can be negative to go backwards)
    #[allow(dead_code)]
    pub fn advance_clock(&mut self, seconds: i64) {
        self.absolute_override = None; // Clear any absolute override
        self.accumulated_drift = self.accumulated_drift.saturating_add(seconds);
    }

    /// Sets the clock to an absolute timestamp, overriding any drift.
    ///
    /// This takes precedence over base timestamp and accumulated drift.
    /// Useful for jump-to-time scenarios.
    ///
    /// # Arguments
    /// * `timestamp` - Absolute ledger timestamp in seconds since Unix epoch
    #[allow(dead_code)]
    pub fn set_clock(&mut self, timestamp: u64) {
        self.absolute_override = Some(timestamp);
        self.accumulated_drift = 0;
        self.base_timestamp = None;
    }

    /// Returns the current ledger timestamp in seconds since Unix epoch.
    ///
    /// Returns in priority order:
    /// 1. Absolute override (if set via [`set_clock`])
    /// 2. Base + accumulated drift (if base is set via [`with_timestamp`])
    /// 3. System time + accumulated drift (fallback)
    #[allow(dead_code)]
    pub fn get_clock(&self) -> u64 {
        if let Some(ts) = self.absolute_override {
            return ts;
        }

        let base = self.base_timestamp.unwrap_or_else(|| {
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_secs())
                .unwrap_or(0)
        });

        base.saturating_add_signed(self.accumulated_drift)
    }

    /// Resets the clock to its initial state (no drift, no override).
    #[allow(dead_code)]
    pub fn reset(&mut self) {
        self.base_timestamp = None;
        self.accumulated_drift = 0;
        self.absolute_override = None;
    }

    /// Returns the base timestamp if explicitly set.
    #[allow(dead_code)]
    pub fn base_timestamp(&self) -> Option<u64> {
        self.base_timestamp
    }

    /// Returns the accumulated drift in seconds.
    #[allow(dead_code)]
    pub fn accumulated_drift(&self) -> i64 {
        self.accumulated_drift
    }

    /// Returns the absolute override if set.
    #[allow(dead_code)]
    pub fn absolute_override(&self) -> Option<u64> {
        self.absolute_override
    }
}

impl Default for ClockDrift {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn clock_drift_new_defaults_to_no_base() {
        let clock = ClockDrift::new();
        assert_eq!(clock.base_timestamp(), None);
        assert_eq!(clock.accumulated_drift(), 0);
        assert_eq!(clock.absolute_override(), None);
    }

    #[test]
    fn clock_drift_with_timestamp_sets_base() {
        let clock = ClockDrift::with_timestamp(1700000000);
        assert_eq!(clock.base_timestamp(), Some(1700000000));
        assert_eq!(clock.get_clock(), 1700000000);
    }

    #[test]
    fn advance_clock_adds_to_accumulated_drift() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(3600);
        assert_eq!(clock.accumulated_drift(), 3600);
        assert_eq!(clock.get_clock(), 1700003600);
    }

    #[test]
    fn advance_clock_can_go_backwards() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(-1800);
        assert_eq!(clock.accumulated_drift(), -1800);
        assert_eq!(clock.get_clock(), 1699998200);
    }

    #[test]
    fn advance_clock_accumulates() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(1000);
        clock.advance_clock(2000);
        clock.advance_clock(500);
        assert_eq!(clock.accumulated_drift(), 3500);
        assert_eq!(clock.get_clock(), 1700003500);
    }

    #[test]
    fn set_clock_overrides_all_state() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(5000);
        assert_eq!(clock.get_clock(), 1700005000);

        clock.set_clock(1800000000);
        assert_eq!(clock.get_clock(), 1800000000);
        assert_eq!(clock.accumulated_drift(), 0);
        assert_eq!(clock.base_timestamp(), None);
        assert_eq!(clock.absolute_override(), Some(1800000000));
    }

    #[test]
    fn advance_after_set_clock_modifies_override() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.set_clock(1800000000);
        clock.advance_clock(100);
        // Advance after set_clock should clear the override and apply drift to base
        assert_eq!(clock.absolute_override(), None);
        assert_eq!(clock.accumulated_drift(), 100);
    }

    #[test]
    fn reset_clears_all_state() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(5000);
        clock.set_clock(1800000000);
        clock.advance_clock(100);

        clock.reset();
        assert_eq!(clock.base_timestamp(), None);
        assert_eq!(clock.accumulated_drift(), 0);
        assert_eq!(clock.absolute_override(), None);
    }

    #[test]
    fn get_clock_without_base_uses_system_time() {
        let clock = ClockDrift::new();
        let before = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        let current = clock.get_clock();
        let after = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();

        assert!(current >= before);
        assert!(current <= after + 1); // +1 to account for rounding
    }

    #[test]
    fn clock_drift_clone_preserves_state() {
        let mut clock = ClockDrift::with_timestamp(1700000000);
        clock.advance_clock(5000);
        let cloned = clock.clone();

        assert_eq!(cloned.get_clock(), clock.get_clock());
        assert_eq!(cloned.accumulated_drift(), clock.accumulated_drift());
    }

    #[test]
    fn saturating_add_prevents_overflow() {
        let mut clock = ClockDrift::with_timestamp(u64::MAX - 100);
        clock.advance_clock(1000);
        // Should saturate at u64::MAX, not wrap around
        assert_eq!(clock.get_clock(), u64::MAX);
    }

    #[test]
    fn saturating_sub_prevents_underflow() {
        let mut clock = ClockDrift::with_timestamp(100);
        clock.advance_clock(-200);
        // Saturating arithmetic should stop at 0, not wrap
        assert_eq!(clock.get_clock(), 0);
    }
}
