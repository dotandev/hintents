// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::types::{WasmLocation};
use wasmparser::{NameSectionReader, Operator, Parser, Payload, BinaryReader, TypeRef, Import};

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
    let name = format!("{:?}", op);
    name.contains("F32") || name.contains("F64")
}

/// Helper to resolve function names and offsets from WASM bytes using wasmparser.
pub struct WasmModule {
    wasm: Vec<u8>,
    function_names: std::collections::HashMap<u32, String>,
}

impl WasmModule {
    pub fn new(wasm: Vec<u8>) -> Self {
        let mut function_names = std::collections::HashMap::new();

        // Parse name section to get function names
        let parser = Parser::new(0);
        for payload in parser.parse_all(&wasm) {
            if let Ok(Payload::CustomSection(section)) = payload {
                if section.name() == "name" {
                    let reader = NameSectionReader::new(BinaryReader::new(section.data(), section.range().start));
                    for name in reader {
                        if let Ok(wasmparser::Name::Function(func_map)) = name {
                            for naming in func_map {
                                if let Ok(naming) = naming {
                                    function_names.insert(naming.index, naming.name.to_string());
                                }
                            }
                        }
                    }
                }
            }
        }

        Self { wasm, function_names }
    }

    pub fn get_function_name(&self, func_index: u32) -> String {
        self.function_names
            .get(&func_index)
            .cloned()
            .unwrap_or_else(|| format!("func[{}]", func_index))
    }

    /// Finds the function index containing the given offset.
    pub fn find_function_at_offset(&self, offset: u64) -> Option<u32> {
        let mut current_func_index = 0;
        let parser = Parser::new(0);
        for payload in parser.parse_all(&self.wasm) {
            match payload {
                Ok(Payload::ImportSection(reader)) => {
                    for import in reader {
                        if let Ok(imp) = import {
                            // Use Debug representation to identify function imports if field names vary
                            let debug = format!("{:?}", imp);
                            if debug.contains("Func") {
                                current_func_index += 1;
                            }
                        }
                    }
                }
                Ok(Payload::FunctionSection(_reader)) => {
                    // This section just gives us the count/sigs, actual code is in CodeSection
                }
                Ok(Payload::CodeSectionEntry(body)) => {
                    let range = body.range();
                    if offset >= range.start as u64 && offset < range.end as u64 {
                        return Some(current_func_index);
                    }
                    current_func_index += 1;
                }
                _ => {}
            }
        }
        None
    }

    pub fn resolve_location(&self, offset: u64) -> Option<WasmLocation> {
        self.find_function_at_offset(offset).map(|idx| WasmLocation {
            function: self.get_function_name(idx),
            offset,
        })
    }
}
