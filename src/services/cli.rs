// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use clap::Parser;

#[derive(Parser, Debug)]
pub struct Cli {
    #[arg(long)]
    pub share: bool,

    #[arg(long)]
    pub public: bool,
}
