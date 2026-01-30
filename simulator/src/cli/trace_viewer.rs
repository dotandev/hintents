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

use crate::theme::ansi::apply;
use crate::theme::load_theme;

#[allow(dead_code)]
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
