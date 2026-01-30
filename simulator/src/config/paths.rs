// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

use std::path::PathBuf;

#[allow(dead_code)]
pub fn theme_path() -> PathBuf {
    let mut path = std::env::var("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| PathBuf::from("."));
    path.push(".erst");
    path.push("theme.json");
    path
}
