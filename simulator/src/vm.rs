// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use wasmparser::{Operator, Parser, Payload};

pub fn enforce_soroban_compatibility(wasm: &[u8]) -> Result<(), String> {
    for payload in Parser::new(0).parse_all(wasm) {
        let payload = payload.map_err(|e| format!("[VM] Wasm parsing: {e}"))?;
        if let Payload::CodeSectionEntry(body) = payload {
            let mut ops = body
                .get_operators_reader()
                .map_err(|e| format!("[VM] Operator reader init: {e}"))?;
            let mut offset: usize = 0;
            while !ops.eof() {
                let op = ops
                    .read()
                    .map_err(|e| format!("[VM] Instruction read at offset {offset}: {e}"))?;
                if is_float_op(&op) {
                    return Err(format!(
                        "[VM] Soroban compatibility check at instruction offset {offset}: \
                         floating-point instructions are not allowed"
                    ));
                }
                offset += 1;
            }
        }
    }
    Ok(())
}

fn is_float_op<'a>(op: &Operator<'a>) -> bool {
    // Many of the `Operator` variants are prefixed with `F32` or `F64` when
    // they perform floating-point operations. To avoid having to keep an
    // exhaustive list in sync with whatever version of `wasmparser` is pulled
    // in, simply look at the debug representation and check for the prefix.
    //
    // This is slightly less strict than matching individual variants, but it's
    // good enough for our compatibility check: any float-related opcode will
    // trigger the `starts_with` condition.
    let name = format!("{:?}", op);
    name.contains("F32") || name.contains("F64")
}
