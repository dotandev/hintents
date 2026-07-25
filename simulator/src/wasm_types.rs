// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! WebAssembly type parsing and signature analysis for enhanced trap diagnostics.
//!
//! This module provides utilities to parse WebAssembly type sections and function tables,
//! enabling detailed error messages when call_indirect traps occur.
//!
//! # Performance notes
//!
//! `TypeSection::parse` performs zero-copy decoding of the WASM binary:
//! - It stops iterating as soon as the TypeSection payload has been consumed,
//!   avoiding a full scan of the remaining sections (code, data, etc.).
//! - `valtype_to_value_type` is `#[inline]` and branch-free; it never allocates.
//! - Parameter and result `Vec`s are populated with `push` rather than
//!   `collect` to avoid a second allocation per function type.
//! - The outer `types` vec is pre-allocated when `wasmparser` exposes a count hint.

#![allow(dead_code)]

use std::fmt::Write as _;

use serde::Serialize;
use wasmparser::{Parser, Payload, ValType};

/// WebAssembly value type representation
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub enum ValueType {
    I32,
    I64,
    F32,
    F64,
    V128,
    FuncRef,
    ExternRef,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TypeError {
    UnsupportedValType(String),
}

impl std::fmt::Display for TypeError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            TypeError::UnsupportedValType(s) => write!(f, "Unsupported value type: {}", s),
        }
    }
}

impl std::error::Error for TypeError {}

/// Zero-copy, allocation-free conversion from a [`ValType`] to a [`ValueType`].
///
/// Returns `None` for reference types that are neither `funcref` nor `externref`
/// rather than allocating a formatted error string.  Callers that need a
/// structured error can convert the `None` case themselves.
#[inline]
fn valtype_to_value_type(vt: ValType) -> Option<ValueType> {
    match vt {
        ValType::I32 => Some(ValueType::I32),
        ValType::I64 => Some(ValueType::I64),
        ValType::F32 => Some(ValueType::F32),
        ValType::F64 => Some(ValueType::F64),
        ValType::V128 => Some(ValueType::V128),
        ValType::Ref(rt) if rt.is_func_ref() => Some(ValueType::FuncRef),
        ValType::Ref(rt) if rt.is_extern_ref() => Some(ValueType::ExternRef),
        _ => None,
    }
}

/// Kept for API compatibility with code that uses `TryFrom<ValType>`.
impl TryFrom<ValType> for ValueType {
    type Error = TypeError;

    #[inline]
    fn try_from(vt: ValType) -> Result<Self, Self::Error> {
        valtype_to_value_type(vt)
            .ok_or_else(|| TypeError::UnsupportedValType(format!("{:?}", vt)))
    }
}

impl std::fmt::Display for ValueType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ValueType::I32 => write!(f, "i32"),
            ValueType::I64 => write!(f, "i64"),
            ValueType::F32 => write!(f, "f32"),
            ValueType::F64 => write!(f, "f64"),
            ValueType::V128 => write!(f, "v128"),
            ValueType::FuncRef => write!(f, "funcref"),
            ValueType::ExternRef => write!(f, "externref"),
        }
    }
}

/// Function signature with parameters and return types
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct FunctionSignature {
    pub params: Vec<ValueType>,
    pub results: Vec<ValueType>,
}

impl FunctionSignature {
    /// Create a new function signature
    pub fn new(params: Vec<ValueType>, results: Vec<ValueType>) -> Self {
        Self { params, results }
    }

    /// Format the signature in human-readable form: `(params) -> (results)`.
    ///
    /// Writes directly into a single pre-allocated `String` to avoid the
    /// intermediate `Vec<String>` allocations that `join` would require.
    pub fn format(&self) -> String {
        // Rough capacity estimate: 6 chars per type + separators + wrapper chars.
        let est = (self.params.len() + self.results.len()) * 6 + 8;
        let mut out = String::with_capacity(est);

        out.push('(');
        for (i, t) in self.params.iter().enumerate() {
            if i > 0 {
                out.push_str(", ");
            }
            // `write!` into a `String` never fails.
            let _ = write!(out, "{}", t);
        }
        out.push_str(") -> (");
        for (i, t) in self.results.iter().enumerate() {
            if i > 0 {
                out.push_str(", ");
            }
            let _ = write!(out, "{}", t);
        }
        out.push(')');
        out
    }

    /// Compare this signature with another and return detailed differences
    pub fn compare(&self, other: &FunctionSignature) -> SignatureDiff {
        let param_count_match = self.params.len() == other.params.len();
        let result_count_match = self.results.len() == other.results.len();

        let mut param_mismatches = Vec::new();
        let mut result_mismatches = Vec::new();

        // Compare parameters
        let min_params = self.params.len().min(other.params.len());
        for i in 0..min_params {
            if self.params[i] != other.params[i] {
                param_mismatches.push((i, self.params[i].clone(), other.params[i].clone()));
            }
        }

        // Compare results
        let min_results = self.results.len().min(other.results.len());
        for i in 0..min_results {
            if self.results[i] != other.results[i] {
                result_mismatches.push((i, self.results[i].clone(), other.results[i].clone()));
            }
        }

        SignatureDiff {
            param_count_match,
            result_count_match,
            param_mismatches,
            result_mismatches,
        }
    }
}

/// Detailed comparison between two function signatures
#[derive(Debug, Clone, Serialize)]
pub struct SignatureDiff {
    pub param_count_match: bool,
    pub result_count_match: bool,
    /// (index, expected_type, actual_type)
    pub param_mismatches: Vec<(usize, ValueType, ValueType)>,
    /// (index, expected_type, actual_type)
    pub result_mismatches: Vec<(usize, ValueType, ValueType)>,
}

impl SignatureDiff {
    /// Check if signatures are identical
    pub fn is_match(&self) -> bool {
        self.param_count_match
            && self.result_count_match
            && self.param_mismatches.is_empty()
            && self.result_mismatches.is_empty()
    }
}

/// Parsed type section containing function signatures
#[derive(Debug, Clone)]
pub struct TypeSection {
    types: Vec<FunctionSignature>,
}

impl TypeSection {
    /// Parse the type section from WebAssembly module bytes.
    ///
    /// # Performance
    /// - Stops iterating payloads as soon as the TypeSection has been consumed;
    ///   subsequent sections (imports, code, data …) are never visited.
    /// - Uses `valtype_to_value_type` which is `#[inline]` and never allocates.
    /// - Pushes directly into pre-allocated `Vec`s instead of `collect`ing
    ///   through an intermediate iterator.
    pub fn parse(wasm_bytes: &[u8]) -> Result<Self, String> {
        let mut types: Vec<FunctionSignature> = Vec::new();

        for payload in Parser::new(0).parse_all(wasm_bytes) {
            let payload = payload.map_err(|e| format!("Failed to parse WASM: {}", e))?;

            match payload {
                Payload::TypeSection(type_reader) => {
                    // Pre-allocate based on the count hint exposed by wasmparser.
                    types.reserve(type_reader.count() as usize);

                    for rec_group in type_reader {
                        let rec_group =
                            rec_group.map_err(|e| format!("Failed to read type: {}", e))?;

                        for sub_type in rec_group.types() {
                            let func_type = sub_type.composite_type.unwrap_func();

                            // Build params — push directly, no intermediate collect.
                            let mut params = Vec::with_capacity(func_type.params().len());
                            for &vt in func_type.params().iter() {
                                let value_type = valtype_to_value_type(vt).ok_or_else(|| {
                                    format!("Unsupported param type: {:?}", vt)
                                })?;
                                params.push(value_type);
                            }

                            // Build results — same approach.
                            let mut results = Vec::with_capacity(func_type.results().len());
                            for &vt in func_type.results().iter() {
                                let value_type = valtype_to_value_type(vt).ok_or_else(|| {
                                    format!("Unsupported result type: {:?}", vt)
                                })?;
                                results.push(value_type);
                            }

                            types.push(FunctionSignature::new(params, results));
                        }
                    }

                    // TypeSection always appears before Code/Data sections in a
                    // valid WASM binary — no need to scan the rest of the file.
                    break;
                }

                // Skip every other payload without allocating.
                Payload::End(_) => break,
                _ => {}
            }
        }

        Ok(TypeSection { types })
    }

    /// Get a function signature by type index
    pub fn get_signature(&self, type_index: u32) -> Option<&FunctionSignature> {
        self.types.get(type_index as usize)
    }

    /// Get the number of types in this section
    pub fn len(&self) -> usize {
        self.types.len()
    }

    /// Check if the type section is empty
    #[allow(dead_code)]
    pub fn is_empty(&self) -> bool {
        self.types.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_value_type_display() {
        assert_eq!(ValueType::I32.to_string(), "i32");
        assert_eq!(ValueType::I64.to_string(), "i64");
        assert_eq!(ValueType::F32.to_string(), "f32");
        assert_eq!(ValueType::F64.to_string(), "f64");
        assert_eq!(ValueType::V128.to_string(), "v128");
        assert_eq!(ValueType::FuncRef.to_string(), "funcref");
        assert_eq!(ValueType::ExternRef.to_string(), "externref");
    }

    #[test]
    fn test_signature_format_empty() {
        let sig = FunctionSignature::new(vec![], vec![]);
        assert_eq!(sig.format(), "() -> ()");
    }

    #[test]
    fn test_signature_format_single_param() {
        let sig = FunctionSignature::new(vec![ValueType::I32], vec![]);
        assert_eq!(sig.format(), "(i32) -> ()");
    }

    #[test]
    fn test_signature_format_multiple_params() {
        let sig = FunctionSignature::new(
            vec![ValueType::I32, ValueType::I64, ValueType::F32],
            vec![ValueType::I64],
        );
        assert_eq!(sig.format(), "(i32, i64, f32) -> (i64)");
    }

    #[test]
    fn test_signature_format_multiple_results() {
        let sig =
            FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I32, ValueType::I64]);
        assert_eq!(sig.format(), "(i32) -> (i32, i64)");
    }

    #[test]
    fn test_signature_compare_identical() {
        let sig1 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let sig2 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let diff = sig1.compare(&sig2);
        assert!(diff.is_match());
        assert!(diff.param_count_match);
        assert!(diff.result_count_match);
        assert!(diff.param_mismatches.is_empty());
        assert!(diff.result_mismatches.is_empty());
    }

    #[test]
    fn test_signature_compare_different_param_count() {
        let sig1 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let sig2 =
            FunctionSignature::new(vec![ValueType::I32, ValueType::I32], vec![ValueType::I64]);
        let diff = sig1.compare(&sig2);
        assert!(!diff.is_match());
        assert!(!diff.param_count_match);
        assert!(diff.result_count_match);
    }

    #[test]
    fn test_signature_compare_different_result_count() {
        let sig1 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let sig2 =
            FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64, ValueType::I32]);
        let diff = sig1.compare(&sig2);
        assert!(!diff.is_match());
        assert!(diff.param_count_match);
        assert!(!diff.result_count_match);
    }

    #[test]
    fn test_signature_compare_different_param_types() {
        let sig1 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let sig2 = FunctionSignature::new(vec![ValueType::I64], vec![ValueType::I64]);
        let diff = sig1.compare(&sig2);
        assert!(!diff.is_match());
        assert_eq!(diff.param_mismatches.len(), 1);
        assert_eq!(diff.param_mismatches[0].0, 0);
        assert_eq!(diff.param_mismatches[0].1, ValueType::I32);
        assert_eq!(diff.param_mismatches[0].2, ValueType::I64);
    }

    #[test]
    fn test_signature_compare_different_result_types() {
        let sig1 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I64]);
        let sig2 = FunctionSignature::new(vec![ValueType::I32], vec![ValueType::I32]);
        let diff = sig1.compare(&sig2);
        assert!(!diff.is_match());
        assert_eq!(diff.result_mismatches.len(), 1);
        assert_eq!(diff.result_mismatches[0].0, 0);
        assert_eq!(diff.result_mismatches[0].1, ValueType::I64);
        assert_eq!(diff.result_mismatches[0].2, ValueType::I32);
    }

    #[test]
    fn test_type_section_parse_simple_module() {
        // Simple WAT: (module (func (param i32) (result i64)))
        let wasm = wat::parse_str(r#"(module (func (param i32) (result i64)))"#).unwrap();
        let type_section = TypeSection::parse(&wasm).unwrap();
        assert_eq!(type_section.len(), 1);
        let sig = type_section.get_signature(0).unwrap();
        assert_eq!(sig.params, vec![ValueType::I32]);
        assert_eq!(sig.results, vec![ValueType::I64]);
    }

    #[test]
    fn test_type_section_parse_multiple_types() {
        let wasm = wat::parse_str(
            r#"
            (module
                (func (param i32) (result i64))
                (func (param i64 i64) (result i32))
            )
            "#,
        )
        .unwrap();
        let type_section = TypeSection::parse(&wasm).unwrap();
        assert_eq!(type_section.len(), 2);

        let sig0 = type_section.get_signature(0).unwrap();
        assert_eq!(sig0.params, vec![ValueType::I32]);
        assert_eq!(sig0.results, vec![ValueType::I64]);

        let sig1 = type_section.get_signature(1).unwrap();
        assert_eq!(sig1.params, vec![ValueType::I64, ValueType::I64]);
        assert_eq!(sig1.results, vec![ValueType::I32]);
    }

    #[test]
    fn test_type_section_get_signature_out_of_bounds() {
        let wasm = wat::parse_str(r#"(module (func (param i32)))"#).unwrap();
        let type_section = TypeSection::parse(&wasm).unwrap();
        assert!(type_section.get_signature(10).is_none());
    }

    /// Verify that modules without a TypeSection parse cleanly to an empty section.
    #[test]
    fn test_type_section_parse_empty_module() {
        let wasm = wat::parse_str(r#"(module)"#).unwrap();
        let type_section = TypeSection::parse(&wasm).unwrap();
        assert_eq!(type_section.len(), 0);
        assert!(type_section.is_empty());
    }

    /// Ensure the inline `valtype_to_value_type` helper covers all supported types.
    #[test]
    fn test_valtype_to_value_type_all_primitives() {
        assert_eq!(valtype_to_value_type(ValType::I32), Some(ValueType::I32));
        assert_eq!(valtype_to_value_type(ValType::I64), Some(ValueType::I64));
        assert_eq!(valtype_to_value_type(ValType::F32), Some(ValueType::F32));
        assert_eq!(valtype_to_value_type(ValType::F64), Some(ValueType::F64));
        assert_eq!(valtype_to_value_type(ValType::V128), Some(ValueType::V128));
    }
}
