// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use wasmparser::{Operator, Parser, Payload};

pub fn enforce_soroban_compatibility(wasm: &[u8]) -> Result<(), String> {
    for payload in Parser::new(0).parse_all(wasm) {
        let payload = payload.map_err(|e| e.to_string())?;
        if let Payload::CodeSectionEntry(body) = payload {
            let mut ops = body.get_operators_reader().map_err(|e| e.to_string())?;
            while !ops.eof() {
                let op = ops.read().map_err(|e| e.to_string())?;
                if is_float_op(&op) {
                    return Err(
                        "floating-point instructions are not allowed under strict Soroban compatibility"
                            .to_string(),
                    );
                }
            }
        }
    }
    Ok(())
}

fn is_float_op(op: &Operator) -> bool {
    use Operator::*;
    matches!(
        op,
        F32Abs
            | F32Neg
            | F32Ceil
            | F32Floor
            | F32Trunc
            | F32Nearest
            | F32Sqrt
            | F32Add
            | F32Sub
            | F32Mul
            | F32Div
            | F32Min
            | F32Max
            | F32Copysign
            | F32Eq
            | F32Ne
            | F32Lt
            | F32Gt
            | F32Le
            | F32Ge
            | F32ConvertI32S
            | F32ConvertI32U
            | F32ConvertI64S
            | F32ConvertI64U
            | F32DemoteF64
            | F32ReinterpretI32
            | F32x4Splat
            | F32x4Eq
            | F32x4Ne
            | F32x4Lt
            | F32x4Gt
            | F32x4Le
            | F32x4Ge
            | F32Const { .. }
            | F64Abs
            | F64Neg
            | F64Ceil
            | F64Floor
            | F64Trunc
            | F64Nearest
            | F64Sqrt
            | F64Add
            | F64Sub
            | F64Mul
            | F64Div
            | F64Min
            | F64Max
            | F64Copysign
            | F64Eq
            | F64Ne
            | F64Lt
            | F64Gt
            | F64Le
            | F64Ge
            | F64ConvertI32S
            | F64ConvertI32U
            | F64ConvertI64S
            | F64ConvertI64U
            | F64PromoteF32
            | F64ReinterpretI64
            | F64x2Splat
            | F64x2Eq
            | F64x2Ne
            | F64x2Lt
            | F64x2Gt
            | F64x2Le
            | F64x2Ge
            | F64Const { .. }
            | I32TruncF32S
            | I32TruncF32U
            | I32TruncF64S
            | I32TruncF64U
            | I64TruncF32S
            | I64TruncF32U
            | I64TruncF64S
            | I64TruncF64U
            | I32TruncSatF32S
            | I32TruncSatF32U
            | I32TruncSatF64S
            | I32TruncSatF64U
            | I64TruncSatF32S
            | I64TruncSatF32U
            | I64TruncSatF64S
            | I64TruncSatF64U
    )
}
