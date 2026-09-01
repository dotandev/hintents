#![no_main]

use libfuzzer_sys::fuzz_target;
use wasmparser::{Parser, Payload};

fuzz_target!(|data: &[u8]| {
    // We fuzz the WASM payload decoder (instruction parsing) to catch panics on malformed modules.
    let _ = erst_sim::wasm_types::TypeSection::parse(data);

    // Replicate the instruction traversal done by the simulator to ensure wasmparser doesn't panic
    for payload in Parser::new(0).parse_all(data) {
        if let Ok(Payload::CodeSectionEntry(body)) = payload {
            if let Ok(mut ops) = body.get_operators_reader() {
                while let Ok(_op) = ops.read() {
                    // Just reading the op ensures we decode it
                }
            }
        }
    }
});
