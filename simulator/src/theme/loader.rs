use serde::Deserialize;
use std::fs;

use super::theme::Theme;
use crate::config::paths::theme_path;

#[derive(Debug, Deserialize)]
struct ThemeConfig {
    span: Option<String>,
    event: Option<String>,
    error: Option<String>,
    warning: Option<String>,
    info: Option<String>,
    dim: Option<String>,
    highlight: Option<String>,
}

pub fn load_theme() -> Theme {
    let default = Theme::default();

    let content = fs::read_to_string(theme_path());
    let Ok(content) = content else {
        return default;
    };

    let Ok(config) = toml::from_str::<ThemeConfig>(&content) else {
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
