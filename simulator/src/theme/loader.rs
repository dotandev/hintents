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

use serde::Deserialize;
use std::fs;

use super::types::Theme;
use crate::config::paths::theme_path;

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
struct ThemeConfig {
    span: Option<String>,
    event: Option<String>,
    error: Option<String>,
    warning: Option<String>,
    info: Option<String>,
    dim: Option<String>,
    highlight: Option<String>,
}

#[allow(dead_code)]
pub fn load_theme() -> Theme {
    let default = Theme::default();

    let content = fs::read_to_string(theme_path());
    let Ok(content) = content else {
        return default;
    };

    let Ok(config) = serde_json::from_str::<ThemeConfig>(&content) else {
        return default;
    };

    Theme {
        span: config.span.unwrap_or(default.span),
        event: config.event.unwrap_or(default.event),
        error: config.error.unwrap_or(default.error),
        warning: config.warning.unwrap_or(default.warning),
        info: config.info.unwrap_or(default.info),
        dim: config.dim.unwrap_or(default.dim),
        highlight: config.highlight.unwrap_or(default.highlight),
    }
}
