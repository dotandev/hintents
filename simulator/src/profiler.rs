// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

use std::fs;
use std::path::Path;

#[cfg(unix)]
mod imp {
    use prost::Message as _;

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

        /// Stop profiling and return the raw pprof protobuf bytes.
        pub fn stop(&mut self) -> Option<Vec<u8>> {
            let guard = self.inner.take()?;
            let report = guard.report().build().ok()?;
            let profile = report.pprof().ok()?;
            let mut buf = Vec::new();
            profile.encode(&mut buf).ok()?;
            Some(buf)
        }
    }

    impl<'a> Drop for PprofGuard<'a> {
        fn drop(&mut self) {
            // Ensure the guard is consumed even if stop() was never called.
            if self.inner.is_some() {
                self.stop();
            }
        }
    }
}

#[cfg(not(unix))]
mod imp {
    /// PprofGuard is a no-op on platforms where the `pprof` crate is not
    /// available (e.g. Windows).
    pub struct PprofGuard;

    impl PprofGuard {
        pub fn start(_frequency: i32) -> Self {
            Self
        }

        pub fn stop(&mut self) -> Option<Vec<u8>> {
            None
        }
    }
}

pub use imp::PprofGuard;

/// Write raw pprof protobuf bytes to a file.
pub fn write_file(bytes: &[u8], path: &Path) -> Result<(), String> {
    fs::write(path, bytes)
        .map_err(|e| format!("Failed to write pprof file {}: {}", path.display(), e))
}

/// Represents a single memory allocation sample along with its call stack.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MemoryAllocationSample {
    pub stack: Vec<String>,
    pub bytes: u64,
}

impl MemoryAllocationSample {
    /// Creates a new memory allocation sample.
    pub fn new<I, S>(stack: I, bytes: u64) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        Self {
            stack: stack.into_iter().map(Into::into).collect(),
            bytes,
        }
    }

    /// Formats the sample into folded stack format (`frame1;frame2;frame3 <bytes>`).
    pub fn format_folded(&self) -> String {
        if self.stack.is_empty() {
            format!("unknown {}", self.bytes)
        } else {
            format!("{} {}", self.stack.join(";"), self.bytes)
        }
    }
}

/// Formats a list of memory allocation samples into folded stack lines.
pub fn format_folded_memory_samples(samples: &[MemoryAllocationSample]) -> String {
    let mut folded = String::new();
    for sample in samples {
        folded.push_str(&sample.format_folded());
        folded.push('\n');
    }
    folded
}

/// Generates an SVG memory allocation flamegraph from folded stack data.
///
/// Uses the `Mem` color palette from `inferno` to visually highlight memory
/// consumption and track memory bloat across contract execution.
pub fn generate_memory_flamegraph(folded_data: &str) -> Result<String, String> {
    generate_memory_flamegraph_with_options(folded_data, None, None)
}

/// Generates an SVG memory allocation flamegraph with custom title and count unit label.
pub fn generate_memory_flamegraph_with_options(
    folded_data: &str,
    title: Option<&str>,
    count_name: Option<&str>,
) -> Result<String, String> {
    let mut options = inferno::flamegraph::Options::default();
    options.title = title.unwrap_or("Memory Allocation Flamegraph").to_string();
    options.count_name = count_name.unwrap_or("bytes").to_string();
    options.colors = inferno::flamegraph::color::Palette::Basic(
        inferno::flamegraph::color::BasicPalette::Mem,
    );

    let mut result_vec = Vec::new();
    inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result_vec)
        .map_err(|e| format!("Failed to generate memory flamegraph: {e}"))?;

    String::from_utf8(result_vec).map_err(|e| format!("Invalid UTF-8 in flamegraph output: {e}"))
}

/// Generates an SVG memory allocation flamegraph directly from memory allocation samples.
pub fn generate_memory_allocation_flamegraph(
    samples: &[MemoryAllocationSample],
) -> Result<String, String> {
    let folded = format_folded_memory_samples(samples);
    generate_memory_flamegraph(&folded)
}

/// Generates an SVG CPU execution flamegraph from folded stack data.
pub fn generate_cpu_flamegraph(folded_data: &str) -> Result<String, String> {
    let mut options = inferno::flamegraph::Options::default();
    options.title = "CPU Execution Flamegraph".to_string();
    options.count_name = "samples".to_string();

    let mut result_vec = Vec::new();
    inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result_vec)
        .map_err(|e| format!("Failed to generate CPU flamegraph: {e}"))?;

    String::from_utf8(result_vec).map_err(|e| format!("Invalid UTF-8 in flamegraph output: {e}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_allocation_sample_formatting() {
        let sample = MemoryAllocationSample::new(["ContractA", "mint", "host_alloc"], 1024);
        assert_eq!(sample.format_folded(), "ContractA;mint;host_alloc 1024");

        let empty_sample = MemoryAllocationSample::new(Vec::<String>::new(), 512);
        assert_eq!(empty_sample.format_folded(), "unknown 512");
    }

    #[test]
    fn test_format_folded_memory_samples() {
        let samples = vec![
            MemoryAllocationSample::new(["ContractA", "transfer"], 2048),
            MemoryAllocationSample::new(["ContractB", "verify"], 4096),
        ];
        let folded = format_folded_memory_samples(&samples);
        assert_eq!(
            folded,
            "ContractA;transfer 2048\nContractB;verify 4096\n"
        );
    }

    #[test]
    fn test_generate_memory_flamegraph() {
        let folded = "Root;Contract::init 1024\nRoot;Contract::execute 4096\n";
        let svg = generate_memory_flamegraph(folded).expect("failed to generate flamegraph");
        assert!(svg.contains("<svg"));
        assert!(svg.contains("</svg>"));
        assert!(svg.contains("Memory Allocation Flamegraph"));
    }

    #[test]
    fn test_generate_memory_flamegraph_with_options() {
        let folded = "Root;Contract::store 2048\n";
        let svg = generate_memory_flamegraph_with_options(
            folded,
            Some("Custom Memory Title"),
            Some("allocs"),
        )
        .expect("failed to generate custom flamegraph");
        assert!(svg.contains("<svg"));
        assert!(svg.contains("Custom Memory Title"));
    }

    #[test]
    fn test_generate_memory_allocation_flamegraph_from_samples() {
        let samples = vec![
            MemoryAllocationSample::new(["Host", "Contract::run", "alloc"], 8192),
            MemoryAllocationSample::new(["Host", "Contract::run", "buffer"], 16384),
        ];
        let svg = generate_memory_allocation_flamegraph(&samples)
            .expect("failed to generate flamegraph from samples");
        assert!(svg.contains("<svg"));
        assert!(svg.contains("</svg>"));
    }

    #[test]
    fn test_generate_cpu_flamegraph() {
        let folded = "Root;Compute::eval 500\nRoot;Compute::parse 300\n";
        let svg = generate_cpu_flamegraph(folded).expect("failed to generate cpu flamegraph");
        assert!(svg.contains("<svg"));
        assert!(svg.contains("CPU Execution Flamegraph"));
    }

    #[test]
    fn test_write_file() {
        let temp_dir = tempfile::tempdir().unwrap();
        let file_path = temp_dir.path().join("test_profile.pb");
        let data = b"test_pprof_bytes";
        let res = write_file(data, &file_path);
        assert!(res.is_ok());
        let read_back = fs::read(&file_path).unwrap();
        assert_eq!(read_back, data);
    }
}
