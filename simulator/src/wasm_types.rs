// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! WebAssembly type parsing and signature analysis for enhanced trap diagnostics.
//!
//! This module provides utilities to parse WebAssembly type sections and function tables,
//! enabling detailed error messages when call_indirect traps occur.

#![allow(dead_code)]

use serde::Serialize;
use wasmparser::{CompositeInnerType, Parser, Payload, ValType};

// ── Error types ──────────────────────────────────────────────────────────────

/// Errors that can arise when converting wasmparser types into this module's types.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum TypeConversionError {
    /// The composite type was not a function type (e.g. it was a struct or array from
    /// the GC proposal), so it cannot be represented as a `FunctionSignature`.
    #[error("expected a function type but found a non-function composite type")]
    NotAFunctionType,
}

/// Errors that can arise when parsing a WebAssembly module's type section.
#[derive(Debug, thiserror::Error)]
pub enum ParseError {
    /// wasmparser rejected the binary.
    #[error("failed to parse WASM module: {0}")]
    Wasm(String),

    /// A recursive-group entry was not a function type.
    #[error("type index {index}: {source}")]
    TypeConversion {
        index: usize,
        #[source]
        source: TypeConversionError,
    },
}

// ── Value type ────────────────────────────────────────────────────────────────

/// WebAssembly value type representation.
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

impl TryFrom<ValType> for ValueType {
    type Error = TypeConversionError;

    fn try_from(vt: ValType) -> Result<Self, Self::Error> {
        Ok(match vt {
            ValType::I32 => ValueType::I32,
            ValType::I64 => ValueType::I64,
            ValType::F32 => ValueType::F32,
            ValType::F64 => ValueType::F64,
            ValType::V128 => ValueType::V128,
            ValType::Ref(rt) => {
                if rt.is_func_ref() {
                    ValueType::FuncRef
                } else {
                    ValueType::ExternRef
                }
            }
        })
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

// ── Function signature ────────────────────────────────────────────────────────

/// Function signature with parameters and return types.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct FunctionSignature {
    pub params: Vec<ValueType>,
    pub results: Vec<ValueType>,
}

impl FunctionSignature {
    /// Create a new function signature.
    pub fn new(params: Vec<ValueType>, results: Vec<ValueType>) -> Self {
        Self { params, results }
    }

    /// Format the signature in human-readable form: `(params) -> (results)`.
    pub fn format(&self) -> String {
        let params = self
            .params
            .iter()
            .map(|t| t.to_string())
            .collect::<Vec<_>>()
            .join(", ");

        let results = self
            .results
            .iter()
            .map(|t| t.to_string())
            .collect::<Vec<_>>()
            .join(", ");

        format!("({}) -> ({})", params, results)
    }

    /// Compare this signature with another and return detailed differences.
    pub fn compare(&self, other: &FunctionSignature) -> SignatureDiff {
        let param_count_match = self.params.len() == other.params.len();
        let result_count_match = self.results.len() == other.results.len();

        let mut param_mismatches = Vec::new();
        let mut result_mismatches = Vec::new();

        let min_params = self.params.len().min(other.params.len());
        for i in 0..min_params {
            if self.params[i] != other.params[i] {
                param_mismatches.push((i, self.params[i].clone(), other.params[i].clone()));
            }
        }

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

/// Try to build a `FunctionSignature` from a wasmparser `CompositeInnerType`.
///
/// Returns `Err(TypeConversionError::NotAFunctionType)` if the composite type is
/// a struct, array (GC proposal), or continuation type (stack-switching proposal)
/// rather than a plain function type.
impl TryFrom<&CompositeInnerType> for FunctionSignature {
    type Error = TypeConversionError;

    fn try_from(inner: &CompositeInnerType) -> Result<Self, Self::Error> {
        match inner {
            CompositeInnerType::Func(func_type) => {
                let params = func_type
                    .params()
                    .iter()
                    .map(|vt| ValueType::try_from(*vt))
                    // ValType → ValueType is currently infallible, but we propagate
                    // the error so callers don't need to change if it gains new variants.
                    .collect::<Result<Vec<_>, _>>()?;

                let results = func_type
                    .results()
                    .iter()
                    .map(|vt| ValueType::try_from(*vt))
                    .collect::<Result<Vec<_>, _>>()?;

                Ok(FunctionSignature::new(params, results))
            }

            // Struct, array, and continuation types are not function types.
            CompositeInnerType::Array(_)
            | CompositeInnerType::Struct(_)
            | CompositeInnerType::Cont(_) => Err(TypeConversionError::NotAFunctionType),
        }
    }
}

// ── Signature diff ────────────────────────────────────────────────────────────

/// Detailed comparison between two function signatures.
#[derive(Debug, Clone, Serialize)]
pub struct SignatureDiff {
    pub param_count_match: bool,
    pub result_count_match: bool,
    /// `(index, expected_type, actual_type)`
    pub param_mismatches: Vec<(usize, ValueType, ValueType)>,
    /// `(index, expected_type, actual_type)`
    pub result_mismatches: Vec<(usize, ValueType, ValueType)>,
}

impl SignatureDiff {
    /// Returns `true` if the two signatures are identical.
    pub fn is_match(&self) -> bool {
        self.param_count_match
            && self.result_count_match
            && self.param_mismatches.is_empty()
            && self.result_mismatches.is_empty()
    }
}

// ── Type section ──────────────────────────────────────────────────────────────

/// Parsed type section containing function signatures.
///
/// Non-function composite types (structs, arrays from the GC proposal, continuations
/// from the stack-switching proposal) are silently skipped during parsing; only
/// function types are stored.
#[derive(Debug, Clone)]
pub struct TypeSection {
    types: Vec<FunctionSignature>,
}

impl TypeSection {
    /// Parse the type section from WebAssembly module bytes.
    ///
    /// Returns a `ParseError` if the binary is malformed or if any encountered
    /// composite type cannot be converted (non-function types are skipped, not
    /// treated as errors — see `TypeSection::parse_strict` for a stricter variant).
    pub fn parse(wasm_bytes: &[u8]) -> Result<Self, ParseError> {
        Self::parse_inner(wasm_bytes, false)
    }

    /// Like [`parse`](Self::parse), but returns an error instead of skipping
    /// non-function composite types (structs/arrays/continuations).
    pub fn parse_strict(wasm_bytes: &[u8]) -> Result<Self, ParseError> {
        Self::parse_inner(wasm_bytes, true)
    }

    fn parse_inner(wasm_bytes: &[u8], strict: bool) -> Result<Self, ParseError> {
        let mut types = Vec::new();
        let mut global_index: usize = 0;

        for payload in Parser::new(0).parse_all(wasm_bytes) {
            let payload = payload.map_err(|e| ParseError::Wasm(e.to_string()))?;

            if let Payload::TypeSection(type_reader) = payload {
                for rec_group in type_reader {
                    let rec_group = rec_group.map_err(|e| ParseError::Wasm(e.to_string()))?;

                    for sub_type in rec_group.types() {
                        let result = FunctionSignature::try_from(&sub_type.composite_type.inner);

                        match result {
                            Ok(sig) => types.push(sig),
                            Err(e) if strict => {
                                return Err(ParseError::TypeConversion {
                                    index: global_index,
                                    source: e,
                                });
                            }
                            Err(_) => { /* skip non-function types in lenient mode */ }
                        }

                        global_index += 1;
                    }
                }
            }
        }

        Ok(TypeSection { types })
    }

    /// Get a function signature by type index.
    pub fn get_signature(&self, type_index: u32) -> Option<&FunctionSignature> {
        self.types.get(type_index as usize)
    }

    /// Get the number of types in this section.
    pub fn len(&self) -> usize {
        self.types.len()
    }

    /// Check if the type section is empty.
    pub fn is_empty(&self) -> bool {
        self.types.is_empty()
    }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    // ── ValueType ──

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
    fn test_value_type_try_from_all_variants() {
        assert_eq!(ValueType::try_from(ValType::I32).unwrap(), ValueType::I32);
        assert_eq!(ValueType::try_from(ValType::I64).unwrap(), ValueType::I64);
        assert_eq!(ValueType::try_from(ValType::F32).unwrap(), ValueType::F32);
        assert_eq!(ValueType::try_from(ValType::F64).unwrap(), ValueType::F64);
        assert_eq!(ValueType::try_from(ValType::V128).unwrap(), ValueType::V128);
    }

    // ── FunctionSignature ──

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

    // ── SignatureDiff ──

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

    // ── TypeSection ──

    #[test]
    fn test_type_section_parse_simple_module() {
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

    #[test]
    fn test_type_section_parse_error_on_bad_bytes() {
        let result = TypeSection::parse(b"not wasm");
        assert!(matches!(result, Err(ParseError::Wasm(_))));
    }

    #[test]
    fn test_type_conversion_error_display() {
        let err = TypeConversionError::NotAFunctionType;
        assert!(!err.to_string().is_empty());
    }

    #[test]
    fn test_parse_error_display_variants() {
        let wasm_err = ParseError::Wasm("bad bytes".into());
        assert!(wasm_err.to_string().contains("bad bytes"));

        let conv_err = ParseError::TypeConversion {
            index: 3,
            source: TypeConversionError::NotAFunctionType,
        };
        assert!(conv_err.to_string().contains("3"));
    }
}