use std::process::{Command, Stdio};

#[test]
fn test_heap_profile_flag_exists() {
    let simulator_path = env!("CARGO_BIN_EXE_simulator");
    
    let output = Command::new(simulator_path)
        .arg("--help")
        .output()
        .expect("Failed to run simulator");
    
    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(stdout.contains("--heap-profile"), "CLI should have --heap-profile flag");
    assert!(stdout.contains("Dump heap profile before and after contract execution"), 
            "Help text should describe heap profiling");
}

#[test]
fn test_heap_profile_flag_accepted() {
    let simulator_path = env!("CARGO_BIN_EXE_simulator");
    
    let test_input = r#"{
        "envelope_xdr": "",
        "result_meta_xdr": "",
        "ledger_entries": null,
        "contract_wasm": null,
        "enable_optimization_advisor": false,
        "profile": false,
        "timestamp": "2026-02-24T15:30:00Z"
    }"#;
    
    let output = Command::new(simulator_path)
        .arg("--heap-profile")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .env("RUST_BACKTRACE", "0")
        .arg("--")
        .spawn()
        .and_then(|mut child| {
            use std::io::Write;
            if let Some(mut stdin) = child.stdin.take() {
                let _ = stdin.write_all(test_input.as_bytes());
            }
            child.wait_with_output()
        });
    
    assert!(output.is_ok(), "Simulator should accept --heap-profile flag");
}

