// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::path::PathBuf;

pub fn theme_path() -> PathBuf {
    let mut path = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
    path.push(".erst");
    path.push("theme.toml");
    path
}
