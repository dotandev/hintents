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

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct Theme {
    pub span: String,
    pub event: String,
    pub error: String,
    pub warning: String,
    pub info: String,
    pub dim: String,
    pub highlight: String,
}

impl Default for Theme {
    fn default() -> Self {
        Self {
            span: "blue".into(),
            event: "cyan".into(),
            error: "red".into(),
            warning: "yellow".into(),
            info: "green".into(),
            dim: "bright_black".into(),
            highlight: "magenta".into(),
        }
    }
}
