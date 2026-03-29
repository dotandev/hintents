// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0


let cli = Cli::parse();

let trace_json = generate_trace();

if cli.share {
    let uploader = GistUploader::new(token);
    let url = uploader.upload(&trace_json, cli.public)?;
    println!("Shared: {}", url);
}
let cli = Cli::parse();

let trace_json = generate_trace();

if cli.share {
    let uploader = GistUploader::new(token);
    let url = uploader.upload(&trace_json, cli.public)?;
    println!("Shared: {}", url);
}
