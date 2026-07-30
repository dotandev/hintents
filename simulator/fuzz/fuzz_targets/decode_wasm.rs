#![no_main]
use libfuzzer_sys::fuzz_target;

// Fuzz target for the WASM payload decoder (TypeSection::parse).
// This parses the WASM module bytes and extracts the type section signatures, catching panics on malformed modules.
fuzz_target!(|data: &[u8]| {
    let _ = erst_sim::wasm_types::TypeSection::parse(data);
});
