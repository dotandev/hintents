// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::path::PathBuf;

#[allow(dead_code)]
fn trace_viewer_temp_root() -> PathBuf {
    std::env::temp_dir().join("erst-trace-viewer")
}

#[allow(dead_code)]
pub fn trace_viewer_temp_path(file_name: &str) -> PathBuf {
    trace_viewer_temp_root().join(file_name)
}

#[allow(dead_code)]
pub fn render_trace() {
    tracing::info!(kind = "span", "User logged in");
    tracing::error!(kind = "error", "Connection failed");
}
