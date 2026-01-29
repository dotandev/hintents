// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::theme::load_theme;
use crate::theme::ansi::apply;

pub fn render_trace() {
    let theme = load_theme();

    println!(
        "{} {}",
        apply(&theme.span, "SPAN"),
        apply(&theme.event, "User logged in")
    );

    println!(
        "{} {}",
        apply(&theme.error, "ERROR"),
        apply(&theme.error, "Connection failed")
    );
}
