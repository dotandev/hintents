// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::collections::HashMap;
use std::fs;
use std::path::Path;
use std::sync::Mutex;

/// Cumulative gas metrics attributed to a single named function.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct GasSample {
    /// Soroban CPU instructions consumed while the function executed.
    pub cpu_insns: u64,
    /// Soroban memory bytes consumed while the function executed.
    pub memory_bytes: u64,
    /// Number of times the function was recorded.
    pub samples: u64,
}

/// Thread-safe registry of gas consumption attributed to named functions.
///
/// The simulation loop records per-function CPU/memory deltas as the Soroban
/// budget meters each Wasm call, then folds the result into the profiler
/// output via [`PprofGuard::stop_with_gas`] so developers can see which
/// functions burned the most gas when optimizing their contracts.
#[derive(Debug, Default)]
pub struct GasTracker {
    inner: Mutex<HashMap<String, GasSample>>,
}

impl GasTracker {
    /// Creates an empty gas tracker.
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    /// Accumulates `cpu_insns` and `memory_bytes` against `function`.
    ///
    /// Subsequent calls for the same function name sum their deltas and bump
    /// the sample counter, so callers can record every metered call boundary
    /// without pre-aggregating.
    pub fn record(&self, function: &str, cpu_insns: u64, memory_bytes: u64) {
        let mut inner = self
            .inner
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        let sample = inner.entry(function.to_string()).or_default();
        sample.cpu_insns = sample.cpu_insns.saturating_add(cpu_insns);
        sample.memory_bytes = sample.memory_bytes.saturating_add(memory_bytes);
        sample.samples = sample.samples.saturating_add(1);
    }

    /// Returns the tracked functions sorted by CPU consumption (descending),
    /// with ties broken by function name for deterministic output.
    #[must_use]
    pub fn snapshot(&self) -> Vec<(String, GasSample)> {
        let mut entries: Vec<(String, GasSample)> = self
            .inner
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
            .iter()
            .map(|(name, sample)| (name.clone(), *sample))
            .collect();
        entries.sort_by(|a, b| {
            b.1.cpu_insns
                .cmp(&a.1.cpu_insns)
                .then_with(|| a.0.cmp(&b.0))
        });
        entries
    }
}

#[cfg(unix)]
mod imp {
    use pprof::protos::{
        Function as PprofFunction, Line as PprofLine, Location as PprofLocation,
        Mapping as PprofMapping, Profile as PprofProfile, Sample as PprofSample,
        ValueType as PprofValueType, Message as _,
    };

    use super::GasTracker;

    /// Wraps `pprof::ProfilerGuard` to produce pprof-compatible CPU profiles.
    pub struct PprofGuard<'a> {
        inner: Option<pprof::ProfilerGuard<'a>>,
    }

    impl<'a> PprofGuard<'a> {
        /// Start profiling at the given sampling frequency (Hz).
        pub fn start(frequency: i32) -> Self {
            let guard = pprof::ProfilerGuardBuilder::default()
                .frequency(frequency)
                .blocklist(&["libc", "libgcc", "pthread", "vdso"])
                .build()
                .ok();
            Self { inner: guard }
        }

        /// Stop profiling and fold the tracked gas metrics into the returned
        /// pprof profile as `gas` (CPU instructions) and `mem` (memory bytes)
        /// sample types, so `go tool pprof` can rank functions by gas cost.
        pub fn stop_with_gas(&mut self, gas: &GasTracker) -> Option<Vec<u8>> {
            let guard = self.inner.take()?;
            let report = guard.report().build().ok()?;
            let mut profile = report.pprof().ok()?;
            append_gas_samples(&mut profile, gas);
            let mut buf = Vec::new();
            profile.encode(&mut buf).ok()?;
            Some(buf)
        }
    }

    impl<'a> Drop for PprofGuard<'a> {
        fn drop(&mut self) {
            // Consume the guard even if stop_with_gas() was never called so the
            // underlying sampler stops before the process exits.
            let _ = self.inner.take();
        }
    }

    /// Sample type names and units embedded in the gas profile.
    const GAS: &str = "gas";
    const MEM: &str = "mem";
    const INSTRUCTIONS: &str = "instructions";
    const BYTES: &str = "bytes";

    /// Folds the tracked gas metrics into an existing pprof profile.
    ///
    /// The CPU profile's `sample_type` is extended with `gas` and `mem`
    /// entries, every existing sample is padded to the new value count, and a
    /// fresh sample is appended per tracked function. The profile's default
    /// sample type is set to `gas` so plain `go tool pprof` invocations rank by
    /// gas cost.
    pub(super) fn append_gas_samples(profile: &mut PprofProfile, tracker: &GasTracker) {
        // pprof requires string_table[0] to be the empty string.
        if profile.string_table.first().is_none_or(|s| !s.is_empty()) {
            profile.string_table.insert(0, String::new());
        }

        let gas_type = PprofValueType {
            ty: intern_string(&mut profile.string_table, GAS),
            unit: intern_string(&mut profile.string_table, INSTRUCTIONS),
        };
        let mem_type = PprofValueType {
            ty: intern_string(&mut profile.string_table, MEM),
            unit: intern_string(&mut profile.string_table, BYTES),
        };

        let gas_index = append_sample_type(profile, &gas_type);
        let mem_index = append_sample_type(profile, &mem_type);

        // Every sample must carry exactly one value per sample type.
        let value_count = profile.sample_type.len();
        for sample in &mut profile.sample {
            sample.value.resize(value_count, 0);
        }

        let mapping_id = ensure_mapping(profile);
        let mut next_function_id = profile.function.iter().map(PprofFunction::id).max().unwrap_or(0);
        let mut next_location_id = profile.location.iter().map(PprofLocation::id).max().unwrap_or(0);

        let mut new_functions = Vec::new();
        let mut new_locations = Vec::new();
        let mut new_samples = Vec::new();

        for (function, sample) in tracker.snapshot() {
            next_function_id += 1;
            next_location_id += 1;
            let name_index = intern_string(&mut profile.string_table, &function);

            new_functions.push(PprofFunction {
                id: next_function_id,
                name: name_index,
                system_name: name_index,
                filename: 0,
                start_line: 0,
            });
            new_locations.push(PprofLocation {
                id: next_location_id,
                mapping_id,
                address: 0,
                line: vec![PprofLine {
                    function_id: next_function_id,
                    line: 0,
                }],
                is_folded: false,
            });

            let mut values = vec![0i64; value_count];
            values[gas_index] = to_i64(sample.cpu_insns);
            values[mem_index] = to_i64(sample.memory_bytes);
            new_samples.push(PprofSample {
                location_id: vec![next_location_id],
                value: values,
                label: Vec::new(),
            });
        }

        profile.function.extend(new_functions);
        profile.location.extend(new_locations);
        profile.sample.extend(new_samples);
        profile.default_sample_type = i64::try_from(gas_index).unwrap_or(i64::MAX);
    }

    /// Appends `sample_type` unless a matching entry already exists, returning
    /// its index into `profile.sample_type`.
    fn append_sample_type(
        profile: &mut PprofProfile,
        sample_type: &PprofValueType,
    ) -> usize {
        if let Some(index) = profile
            .sample_type
            .iter()
            .position(|existing| existing.ty == sample_type.ty && existing.unit == sample_type.unit)
        {
            return index;
        }
        profile.sample_type.push(sample_type.clone());
        profile.sample_type.len() - 1
    }

    /// Returns the id of the profile's mapping, creating a synthetic mapping
    /// when the profile carries none.
    fn ensure_mapping(profile: &mut PprofProfile) -> u64 {
        if let Some(mapping) = profile.mapping.first() {
            return mapping.id;
        }
        profile.mapping.push(PprofMapping {
            id: 1,
            ..PprofMapping::default()
        });
        1
    }

    /// Interns `value` into `table`, returning its index.
    fn intern_string(table: &mut Vec<String>, value: &str) -> i64 {
        if let Some(index) = table.iter().position(|entry| entry == value) {
            return i64::try_from(index).unwrap_or(i64::MAX);
        }
        table.push(value.to_string());
        i64::try_from(table.len() - 1).unwrap_or(i64::MAX)
    }

    /// Converts a gas counter to a pprof int64 value, saturating on overflow.
    fn to_i64(value: u64) -> i64 {
        i64::try_from(value).unwrap_or(i64::MAX)
    }
}

#[cfg(not(unix))]
mod imp {
    use super::{gas_metrics_json, GasTracker};

    /// PprofGuard is a no-op on platforms where the `pprof` crate is not
    /// available (e.g. Windows).
    pub struct PprofGuard;

    impl PprofGuard {
        /// No-op constructor kept for API compatibility.
        pub fn start(_frequency: i32) -> Self {
            Self
        }

        /// On platforms without `pprof`, the tracked gas metrics are returned
        /// as a JSON document so the profiler output contract stays uniform.
        pub fn stop_with_gas(&mut self, gas: &GasTracker) -> Option<Vec<u8>> {
            Some(gas_metrics_json(gas).into_bytes())
        }
    }
}

pub use imp::PprofGuard;

/// Renders the tracked gas metrics as a self-describing JSON document.
///
/// Used on platforms where the `pprof` crate is unavailable so the profiler
/// output contract stays uniform. On Unix the same data is embedded directly
/// into the pprof profile instead.
#[cfg(not(unix))]
pub fn gas_metrics_json(tracker: &GasTracker) -> String {
    let entries: Vec<serde_json::Value> = tracker
        .snapshot()
        .into_iter()
        .map(|(function, sample)| {
            serde_json::json!({
                "function": function,
                "cpu_insns": sample.cpu_insns,
                "memory_bytes": sample.memory_bytes,
                "samples": sample.samples,
            })
        })
        .collect();
    serde_json::json!({ "gas_metrics": entries }).to_string()
}

/// Write raw profiler protobuf bytes to a file.
pub fn write_file(bytes: &[u8], path: &Path) -> Result<(), String> {
    fs::write(path, bytes)
        .map_err(|e| format!("Failed to write pprof file {}: {}", path.display(), e))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn gas_tracker_accumulates_per_function() {
        let tracker = GasTracker::new();
        tracker.record("transfer", 100, 10);
        tracker.record("transfer", 50, 5);
        tracker.record("swap", 200, 20);

        let snapshot = tracker.snapshot();
        assert_eq!(snapshot.len(), 2);

        assert_eq!(snapshot[0].0, "swap");
        assert_eq!(snapshot[0].1.cpu_insns, 200);
        assert_eq!(snapshot[0].1.memory_bytes, 20);
        assert_eq!(snapshot[0].1.samples, 1);

        assert_eq!(snapshot[1].0, "transfer");
        assert_eq!(snapshot[1].1.cpu_insns, 150);
        assert_eq!(snapshot[1].1.memory_bytes, 15);
        assert_eq!(snapshot[1].1.samples, 2);
    }

    #[test]
    fn gas_tracker_saturates_on_overflow() {
        let tracker = GasTracker::new();
        tracker.record("hot", u64::MAX, u64::MAX);
        tracker.record("hot", 1, 1);

        let snapshot = tracker.snapshot();
        assert_eq!(snapshot[0].1.cpu_insns, u64::MAX);
        assert_eq!(snapshot[0].1.memory_bytes, u64::MAX);
        assert_eq!(snapshot[0].1.samples, 2);
    }

    #[test]
    fn gas_tracker_orders_by_cpu_desc_then_name() {
        let tracker = GasTracker::new();
        tracker.record("b", 1, 0);
        tracker.record("a", 1, 0);

        let snapshot = tracker.snapshot();
        assert_eq!(snapshot[0].0, "a");
        assert_eq!(snapshot[1].0, "b");
    }

    #[test]
    fn empty_tracker_has_no_snapshots() {
        let tracker = GasTracker::new();
        assert!(tracker.snapshot().is_empty());
    }

    #[cfg(not(unix))]
    #[test]
    fn gas_metrics_json_renders_all_entries() {
        let tracker = GasTracker::new();
        tracker.record("transfer", 100, 10);
        tracker.record("swap", 200, 20);

        let json = gas_metrics_json(&tracker);
        assert!(json.contains("transfer"));
        assert!(json.contains("swap"));
        assert!(json.contains("cpu_insns"));
        assert!(json.contains("memory_bytes"));
    }

    #[cfg(unix)]
    #[test]
    fn append_gas_samples_embeds_gas_types() {
        use pprof::protos::{Profile as PprofProfile, ValueType as PprofValueType};

        let tracker = GasTracker::new();
        tracker.record("transfer", 300, 30);

        let mut profile = PprofProfile {
            sample_type: vec![PprofValueType { ty: 1, unit: 2 }],
            string_table: vec![
                String::new(),
                "cpu".to_string(),
                "nanoseconds".to_string(),
            ],
            ..PprofProfile::default()
        };
        imp::append_gas_samples(&mut profile, &tracker);

        let types: Vec<&str> = profile
            .sample_type
            .iter()
            .map(|value_type| profile.string_table[value_type.ty as usize].as_str())
            .collect();
        assert!(types.contains(&"gas"));
        assert!(types.contains(&"mem"));

        assert_eq!(profile.sample.len(), 1);
        let sample = &profile.sample[0];
        assert_eq!(sample.value.len(), profile.sample_type.len());
        assert!(sample.value.iter().any(|&value| value == 300));
    }
}
