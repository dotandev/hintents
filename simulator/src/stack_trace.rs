// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Enhanced WASM stack trace generation.
//!
//! Exposes the Wasmi internal call stack directly on traps,
//! bypassing Soroban Host abstractions for low-level debugging.

#![allow(dead_code)]

use crate::source_mapper::{SourceLocation, SourceMapper};
use once_cell::sync::Lazy;
use regex::Regex;
use serde::Serialize;

/// A single frame in a WASM call stack.
#[derive(Debug, Clone, Serialize, PartialEq)]
pub struct StackFrame {
    pub index: usize,
    pub func_index: Option<u32>,
    pub func_name: Option<String>,
    pub wasm_offset: Option<u64>,
    pub module: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_location: Option<SourceLocation>,
}

/// Categorised trap reason extracted from a raw error string.
#[derive(Debug, Clone, Serialize, PartialEq)]
pub enum TrapKind {
    OutOfBoundsMemoryAccess,
    OutOfBoundsTableAccess,
    IntegerOverflow,
    IntegerDivisionByZero,
    InvalidConversionToInt,
    Unreachable,
    StackOverflow,
    IndirectCallTypeMismatch,
    UndefinedElement,
    HostError(String),
    Unknown(String),
}

/// Structured stack trace emitted on a WASM trap.
#[derive(Debug, Clone, Serialize)]
pub struct WasmStackTrace {
    pub trap_kind: TrapKind,
    pub raw_message: String,
    pub frames: Vec<StackFrame>,
    pub soroban_wrapped: bool,
}

// ---------------------------------------------------------------------------
// Compiled regexes (initialised once)
// ---------------------------------------------------------------------------

/// Matches a numbered frame line, e.g.:
///   `  0: func[42] @ 0xa3c`
///   `  #1: some::symbol @ 0xb20`
///   `  2: func[7]`
static RE_NUMBERED_FRAME: Lazy<Regex> = Lazy::new(|| {
    Regex::new(
        r"(?x)
        ^\s*\#?(?P<idx>\d+)\s*:\s*   # leading index
        (?P<body>[^\r\n]+?)           # frame body
        \s*$",
    )
    .unwrap()
});

/// Matches `func[N]` inside a frame body.
static RE_FUNC_INDEX: Lazy<Regex> = Lazy::new(|| Regex::new(r"^func\[(?P<idx>\d+)\]$").unwrap());

/// Matches a hex or decimal offset after ` @ `.
static RE_OFFSET: Lazy<Regex> =
    Lazy::new(|| Regex::new(r"@\s*(?:0x(?P<hex>[0-9a-fA-F]+)|(?P<dec>\d+))\s*$").unwrap());

/// Matches an explicit trapping instruction offset pattern, e.g.:
///   `trapping instruction code offset: 0xa3c`
///   `code offset 0x1000`
///   `trapped at offset 0x42`
///   `wasm offset: 0x123`
static RE_TRAPPING_OFFSET: Lazy<Regex> = Lazy::new(|| {
    Regex::new(
        r"(?ix)
        (?:
            trapping\s+(?:instruction\s+)?code\s+offset
          | code\s+offset
          | trapping\s+instruction\s+at
          | trapped\s+at(?:\s+offset)?
          | wasm\s+offset
          | (?:at\s+)?offset
        )\s*[:=]?\s*
        (?:0x(?P<hex>[0-9a-fA-F]+)|(?P<dec>\d+))
        ",
    )
    .unwrap()
});

/// Matches `@ 0xHEX` or `@ DEC` anywhere in a message or frame line.
static RE_AT_ANY_OFFSET: Lazy<Regex> =
    Lazy::new(|| Regex::new(r"@\s*(?:0x(?P<hex>[0-9a-fA-F]+)|(?P<dec>\d+))").unwrap());

/// Matches a Rust source location pattern in panic messages, e.g.:
///   `src/lib.rs:15:9`
///   `panicked at '...', tests/fixtures/contracts/panic.rust:39:9`
static RE_SOURCE_LOCATION_IN_MSG: Lazy<Regex> = Lazy::new(|| {
    Regex::new(
        r"(?x)
        (?:panicked\s+at\s+(?:'[^']*',\s+)?)?
        (?P<file>[a-zA-Z0-9_./\\-]+\.(?:rs|rust))
        :(?P<line>\d+)
        (?::(?P<col>\d+))?
        ",
    )
    .unwrap()
});

/// Extract the trapping instruction's code offset from an error or panic message.
pub fn extract_trapping_offset(msg: &str) -> Option<u64> {
    if let Some(caps) = RE_TRAPPING_OFFSET.captures(msg) {
        if let Some(hex) = caps.name("hex") {
            if let Ok(val) = u64::from_str_radix(hex.as_str(), 16) {
                return Some(val);
            }
        } else if let Some(dec) = caps.name("dec") {
            if let Ok(val) = dec.as_str().parse::<u64>() {
                return Some(val);
            }
        }
    }

    if let Some(caps) = RE_AT_ANY_OFFSET.captures(msg) {
        if let Some(hex) = caps.name("hex") {
            if let Ok(val) = u64::from_str_radix(hex.as_str(), 16) {
                return Some(val);
            }
        } else if let Some(dec) = caps.name("dec") {
            if let Ok(val) = dec.as_str().parse::<u64>() {
                return Some(val);
            }
        }
    }

    None
}

/// Extract a source location directly from a panic or error message if present (e.g. `src/lib.rs:12:4`).
pub fn extract_source_location_from_msg(msg: &str) -> Option<SourceLocation> {
    let caps = RE_SOURCE_LOCATION_IN_MSG.captures(msg)?;
    let file = caps.name("file")?.as_str().to_string();
    let line: u32 = caps.name("line")?.as_str().parse().ok()?;
    let column: Option<u32> = caps.name("col").and_then(|c| c.as_str().parse().ok());
    Some(SourceLocation {
        file,
        line,
        column,
        column_end: None,
        github_link: None,
    })
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

impl WasmStackTrace {
    /// Build a stack trace by parsing a raw HostError debug representation.
    pub fn from_host_error(error_debug: &str, mapper: Option<&SourceMapper>) -> Self {
        let trap_kind = classify_trap(error_debug);
        let mut frames = extract_frames(error_debug, mapper);
        let soroban_wrapped = error_debug.contains("HostError")
            || error_debug.contains("ScError")
            || error_debug.contains("Error(WasmVm");

        let trapping_offset = extract_trapping_offset(error_debug)
            .or_else(|| frames.first().and_then(|f| f.wasm_offset));

        if let Some(offset) = trapping_offset {
            if frames.is_empty() {
                let loc = mapper
                    .and_then(|m| m.map_wasm_offset_to_source(offset))
                    .or_else(|| extract_source_location_from_msg(error_debug));
                frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: Some(offset),
                    module: None,
                    source_location: loc,
                });
            } else {
                let top = &mut frames[0];
                if top.wasm_offset.is_none() {
                    top.wasm_offset = Some(offset);
                }
                if top.source_location.is_none() {
                    top.source_location = mapper
                        .and_then(|m| {
                            m.map_wasm_offset_to_source(top.wasm_offset.unwrap_or(offset))
                        })
                        .or_else(|| extract_source_location_from_msg(error_debug));
                }
            }
        } else if let Some(loc) = extract_source_location_from_msg(error_debug) {
            if frames.is_empty() {
                frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: None,
                    module: None,
                    source_location: Some(loc),
                });
            } else if frames[0].source_location.is_none() {
                frames[0].source_location = Some(loc);
            }
        }

        WasmStackTrace {
            trap_kind,
            raw_message: error_debug.to_string(),
            frames,
            soroban_wrapped,
        }
    }

    /// Build a trace from a panic payload.
    pub fn from_panic(message: &str) -> Self {
        Self::from_panic_with_mapper(message, None)
    }

    /// Build a trace from a panic payload and optional source mapper.
    pub fn from_panic_with_mapper(message: &str, mapper: Option<&SourceMapper>) -> Self {
        let trap_kind = classify_trap(message);
        let mut frames = extract_frames(message, mapper);
        let soroban_wrapped = message.contains("HostError")
            || message.contains("ScError")
            || message.contains("Error(WasmVm");

        let trapping_offset =
            extract_trapping_offset(message).or_else(|| frames.first().and_then(|f| f.wasm_offset));

        if let Some(offset) = trapping_offset {
            if frames.is_empty() {
                let loc = mapper
                    .and_then(|m| m.map_wasm_offset_to_source(offset))
                    .or_else(|| extract_source_location_from_msg(message));
                frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: Some(offset),
                    module: None,
                    source_location: loc,
                });
            } else {
                let top = &mut frames[0];
                if top.wasm_offset.is_none() {
                    top.wasm_offset = Some(offset);
                }
                if top.source_location.is_none() {
                    top.source_location = mapper
                        .and_then(|m| {
                            m.map_wasm_offset_to_source(top.wasm_offset.unwrap_or(offset))
                        })
                        .or_else(|| extract_source_location_from_msg(message));
                }
            }
        } else if let Some(loc) = extract_source_location_from_msg(message) {
            if frames.is_empty() {
                frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: None,
                    module: None,
                    source_location: Some(loc),
                });
            } else if frames[0].source_location.is_none() {
                frames[0].source_location = Some(loc);
            }
        }

        let mut trace = WasmStackTrace {
            trap_kind,
            raw_message: message.to_string(),
            frames,
            soroban_wrapped,
        };

        if let Some(m) = mapper {
            trace.resolve_sources(m);
        }

        trace
    }

    /// Populate `source_location` on each frame using the provided `SourceMapper`.
    pub fn resolve_sources(&mut self, mapper: &SourceMapper) {
        let trapping_offset = self
            .offset()
            .or_else(|| extract_trapping_offset(&self.raw_message));

        if self.frames.is_empty() {
            if let Some(offset) = trapping_offset {
                let loc = mapper
                    .map_wasm_offset_to_source(offset)
                    .or_else(|| extract_source_location_from_msg(&self.raw_message));
                self.frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: Some(offset),
                    module: None,
                    source_location: loc,
                });
            } else if let Some(loc) = extract_source_location_from_msg(&self.raw_message) {
                self.frames.push(StackFrame {
                    index: 0,
                    func_index: None,
                    func_name: None,
                    wasm_offset: None,
                    module: None,
                    source_location: Some(loc),
                });
            }
        } else {
            if let Some(offset) = trapping_offset {
                if self.frames[0].wasm_offset.is_none() {
                    self.frames[0].wasm_offset = Some(offset);
                }
                if self.frames[0].source_location.is_none() {
                    self.frames[0].source_location = mapper
                        .map_wasm_offset_to_source(self.frames[0].wasm_offset.unwrap_or(offset))
                        .or_else(|| extract_source_location_from_msg(&self.raw_message));
                }
            }

            for frame in &mut self.frames {
                if frame.source_location.is_none() {
                    if let Some(offset) = frame.wasm_offset {
                        frame.source_location = mapper.map_wasm_offset_to_source(offset);
                    }
                }
            }
        }
    }

    /// Format the trace as a human-readable string.
    pub fn display(&self) -> String {
        let mut out = String::new();
        out.push_str(&format!("Trap: {}\n", self.trap_kind_label()));

        if self.soroban_wrapped {
            out.push_str("  (error passed through Soroban Host layer)\n");
        }

        if self.frames.is_empty() {
            out.push_str("  <no frames captured>\n");
        } else {
            out.push_str("  Call stack (most recent call last):\n");
            for frame in &self.frames {
                out.push_str(&format!("    #{}: ", frame.index));

                if let Some(ref name) = frame.func_name {
                    out.push_str(name);
                } else if let Some(idx) = frame.func_index {
                    out.push_str(&format!("func[{}]", idx));
                } else {
                    out.push_str("<unknown>");
                }

                if let Some(ref module) = frame.module {
                    out.push_str(&format!(" in {}", module));
                }

                if let Some(ref loc) = frame.source_location {
                    out.push_str(&format!(" ({})", loc));
                } else if let Some(offset) = frame.wasm_offset {
                    out.push_str(&format!(" @ 0x{:x}", offset));
                }

                out.push('\n');
            }
        }

        out
    }

    fn trap_kind_label(&self) -> &str {
        match &self.trap_kind {
            TrapKind::OutOfBoundsMemoryAccess => "out of bounds memory access",
            TrapKind::OutOfBoundsTableAccess => "out of bounds table access",
            TrapKind::IntegerOverflow => "integer overflow",
            TrapKind::IntegerDivisionByZero => "integer division by zero",
            TrapKind::InvalidConversionToInt => "invalid conversion to integer",
            TrapKind::Unreachable => "unreachable instruction executed",
            TrapKind::StackOverflow => "stack overflow",
            TrapKind::IndirectCallTypeMismatch => "indirect call type mismatch",
            TrapKind::UndefinedElement => "undefined table element",
            TrapKind::HostError(_) => "host error",
            TrapKind::Unknown(_) => "unknown trap",
        }
    }

    /// Get the WASM offset of the most recent frame (the trap site), if known.
    pub fn offset(&self) -> Option<u64> {
        self.frames.first().and_then(|f| f.wasm_offset)
    }
}

// ---------------------------------------------------------------------------
// Trap classification
// ---------------------------------------------------------------------------

/// Classify a raw error string into a known trap kind.
fn classify_trap(msg: &str) -> TrapKind {
    let lower = msg.to_lowercase();

    if lower.contains("out of bounds memory") {
        TrapKind::OutOfBoundsMemoryAccess
    } else if lower.contains("out of bounds table") {
        TrapKind::OutOfBoundsTableAccess
    } else if lower.contains("integer overflow") {
        TrapKind::IntegerOverflow
    } else if lower.contains("integer division by zero") || lower.contains("division by zero") {
        TrapKind::IntegerDivisionByZero
    } else if lower.contains("invalid conversion to int") {
        TrapKind::InvalidConversionToInt
    } else if lower.contains("unreachable") {
        TrapKind::Unreachable
    } else if lower.contains("call stack exhausted") || lower.contains("stack overflow") {
        TrapKind::StackOverflow
    } else if lower.contains("indirect call type mismatch") {
        TrapKind::IndirectCallTypeMismatch
    } else if lower.contains("undefined element") || lower.contains("uninitialized element") {
        TrapKind::UndefinedElement
    } else if lower.contains("hosterror") || lower.contains("host error") {
        TrapKind::HostError(msg.to_string())
    } else {
        TrapKind::Unknown(msg.to_string())
    }
}

// ---------------------------------------------------------------------------
// Frame extraction  regex-based, replaces brittle string splitting
// ---------------------------------------------------------------------------

/// Extract call stack frames from a Wasmi/Soroban error string using
/// anchored regex patterns instead of fragile line-splitting heuristics.
fn extract_frames(error_debug: &str, mapper: Option<&SourceMapper>) -> Vec<StackFrame> {
    let mut frames = Vec::new();

    for line in error_debug.lines() {
        if let Some(frame) = try_parse_numbered_frame(line, mapper) {
            frames.push(frame);
        } else {
            let trimmed = line.trim();
            if trimmed.starts_with("func[") || trimmed.starts_with('<') || trimmed.contains(" @ ") {
                if let Some(frame) = try_parse_bare_frame(trimmed, frames.len(), mapper) {
                    frames.push(frame);
                }
            }
        }
    }

    frames
}

/// Parse a frame line that begins with an index: `0: func[42] @ 0xa3c`.
fn try_parse_numbered_frame(line: &str, mapper: Option<&SourceMapper>) -> Option<StackFrame> {
    let caps = RE_NUMBERED_FRAME.captures(line)?;
    let index: usize = caps["idx"].parse().ok()?;
    let body = caps["body"].trim();

    let (func_name, func_index, wasm_offset) = parse_frame_body(body);
    let source_location =
        wasm_offset.and_then(|o| mapper.and_then(|m| m.map_wasm_offset_to_source(o)));

    Some(StackFrame {
        index,
        func_index,
        func_name,
        wasm_offset,
        module: None,
        source_location,
    })
}

/// Parse a bare frame line without a leading index.
fn try_parse_bare_frame(
    line: &str,
    index: usize,
    mapper: Option<&SourceMapper>,
) -> Option<StackFrame> {
    let (func_name, func_index, wasm_offset) = parse_frame_body(line);

    if func_name.is_some() || func_index.is_some() {
        let source_location =
            wasm_offset.and_then(|o| mapper.and_then(|m| m.map_wasm_offset_to_source(o)));

        Some(StackFrame {
            index,
            func_index,
            func_name,
            wasm_offset,
            module: None,
            source_location,
        })
    } else {
        None
    }
}

/// Parse the body of a frame line into (func_name, func_index, wasm_offset).
///
/// Uses compiled regexes instead of manual string splitting.
pub fn parse_frame_body(body: &str) -> (Option<String>, Option<u32>, Option<u64>) {
    if body.is_empty() {
        return (None, None, None);
    }

    // Extract offset with regex  handles both `@ 0xABC` and `@ 1234`.
    let wasm_offset = RE_OFFSET.captures(body).and_then(|c| {
        if let Some(hex) = c.name("hex") {
            u64::from_str_radix(hex.as_str(), 16).ok()
        } else {
            c.name("dec").and_then(|d| d.as_str().parse().ok())
        }
    });

    // Strip the offset portion to get the name part.
    let name_part = if let Some(at) = body.find(" @ ") {
        body[..at].trim()
    } else {
        body.trim()
    };

    // Detect `func[N]` pattern with regex.
    if let Some(caps) = RE_FUNC_INDEX.captures(name_part) {
        if let Ok(idx) = caps["idx"].parse::<u32>() {
            return (None, Some(idx), wasm_offset);
        }
        // Parse failure: fall through and treat the body as a symbol name.
    }

    // Otherwise treat the name part as a symbol name.
    let func_name = if name_part.is_empty() {
        None
    } else {
        Some(name_part.to_string())
    };

    (func_name, None, wasm_offset)
}

/// Decode a raw error string into a human-readable description.
#[allow(dead_code)]
pub fn decode_error(msg: &str) -> String {
    let trace = WasmStackTrace::from_host_error(msg, None);
    let label = trace.trap_kind_label();

    if label != "unknown trap" {
        format!("VM Trap: {} -- {}", capitalise_first(label), msg)
    } else {
        format!("Error: {}", msg)
    }
}

#[allow(dead_code)]
fn capitalise_first(s: &str) -> String {
    let mut chars = s.chars();
    match chars.next() {
        None => String::new(),
        Some(c) => c.to_uppercase().to_string() + chars.as_str(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_classify_oob_memory() {
        let kind = classify_trap("Error: Wasm Trap: out of bounds memory access");
        assert_eq!(kind, TrapKind::OutOfBoundsMemoryAccess);
    }

    #[test]
    fn test_classify_unreachable() {
        let kind = classify_trap("wasm trap: unreachable");
        assert_eq!(kind, TrapKind::Unreachable);
    }

    #[test]
    fn test_classify_stack_overflow() {
        let kind = classify_trap("call stack exhausted");
        assert_eq!(kind, TrapKind::StackOverflow);
    }

    #[test]
    fn test_classify_division_by_zero() {
        let kind = classify_trap("integer division by zero");
        assert_eq!(kind, TrapKind::IntegerDivisionByZero);
    }

    #[test]
    fn test_classify_host_error() {
        let kind = classify_trap("HostError: contract call failed");
        assert!(matches!(kind, TrapKind::HostError(_)));
    }

    #[test]
    fn test_classify_unknown() {
        let kind = classify_trap("something completely unexpected");
        assert!(matches!(kind, TrapKind::Unknown(_)));
    }

    #[test]
    fn test_extract_numbered_frames() {
        let input = "wasm backtrace:\n  0: func[42] @ 0xa3c\n  1: func[7] @ 0xb20";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 2);
        assert_eq!(frames[0].index, 0);
        assert_eq!(frames[0].func_index, Some(42));
        assert_eq!(frames[0].wasm_offset, Some(0xa3c));
        assert_eq!(frames[1].index, 1);
        assert_eq!(frames[1].func_index, Some(7));
        assert_eq!(frames[1].wasm_offset, Some(0xb20));
    }

    #[test]
    fn test_extract_named_frames() {
        let input =
            "trace:\n  0: soroban_token::transfer @ 0x100\n  1: soroban_sdk::invoke @ 0x200";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 2);
        assert_eq!(
            frames[0].func_name,
            Some("soroban_token::transfer".to_string())
        );
        assert_eq!(frames[0].wasm_offset, Some(0x100));
    }

    #[test]
    fn test_extract_no_frames() {
        let input = "simple error message without any stack frames";
        let frames = extract_frames(input, None);
        assert!(frames.is_empty());
    }

    #[test]
    fn test_from_host_error_soroban_wrapped() {
        let trace = WasmStackTrace::from_host_error(
            "HostError: Error(WasmVm, InternalError)\n  0: func[5] @ 0x42",
            None,
        );
        assert!(trace.soroban_wrapped);
        assert_eq!(trace.frames.len(), 1);
        assert_eq!(trace.frames[0].func_index, Some(5));
    }

    #[test]
    fn test_from_host_error_not_soroban_wrapped() {
        let trace = WasmStackTrace::from_host_error("wasm trap: unreachable\n  0: func[10]", None);
        assert!(!trace.soroban_wrapped);
        assert_eq!(trace.trap_kind, TrapKind::Unreachable);
    }

    #[test]
    fn test_from_panic() {
        let trace = WasmStackTrace::from_panic("assertion failed");
        assert!(trace.frames.is_empty());
        assert!(!trace.soroban_wrapped);
        assert!(matches!(trace.trap_kind, TrapKind::Unknown(_)));
    }

    #[test]
    fn test_display_with_frames() {
        let trace = WasmStackTrace {
            trap_kind: TrapKind::OutOfBoundsMemoryAccess,
            raw_message: "test".to_string(),
            frames: vec![
                StackFrame {
                    index: 0,
                    func_index: Some(42),
                    func_name: None,
                    wasm_offset: Some(0xa3c),
                    module: None,
                    source_location: None,
                },
                StackFrame {
                    index: 1,
                    func_index: None,
                    func_name: Some("my_contract::transfer".to_string()),
                    wasm_offset: Some(0xb20),
                    module: Some("token".to_string()),
                    source_location: None,
                },
            ],
            soroban_wrapped: false,
        };
        let output = trace.display();
        assert!(output.contains("out of bounds memory access"));
        assert!(output.contains("func[42]"));
        assert!(output.contains("0xa3c"));
        assert!(output.contains("my_contract::transfer"));
        assert!(output.contains("in token"));
    }

    #[test]
    fn test_display_empty_frames() {
        let trace = WasmStackTrace::from_panic("boom");
        let output = trace.display();
        assert!(output.contains("<no frames captured>"));
    }

    #[test]
    fn test_display_soroban_wrapped() {
        let trace = WasmStackTrace::from_host_error("HostError: something", None);
        let output = trace.display();
        assert!(output.contains("Soroban Host layer"));
    }

    #[test]
    fn test_decode_error_known_trap() {
        let msg = decode_error("Error: Wasm Trap: out of bounds memory access");
        assert!(msg.contains("VM Trap: Out of bounds memory access"));
    }

    #[test]
    fn test_decode_error_unknown() {
        let msg = decode_error("some random error");
        assert!(msg.starts_with("Error:"));
    }

    #[test]
    fn test_frame_with_offset_no_hex_prefix() {
        let input = "  0: func[1] @ 1234";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 1);
        assert_eq!(frames[0].wasm_offset, Some(1234));
    }

    #[test]
    fn test_parse_frame_body_empty() {
        let (name, index, offset) = parse_frame_body("");
        assert!(name.is_none());
        assert!(index.is_none());
        assert!(offset.is_none());
    }

    #[test]
    fn test_classify_table_access() {
        assert_eq!(
            classify_trap("out of bounds table access"),
            TrapKind::OutOfBoundsTableAccess
        );
    }

    #[test]
    fn test_classify_indirect_call_mismatch() {
        assert_eq!(
            classify_trap("indirect call type mismatch"),
            TrapKind::IndirectCallTypeMismatch
        );
    }

    #[test]
    fn test_capitalise_first() {
        assert_eq!(capitalise_first("hello"), "Hello");
        assert_eq!(capitalise_first(""), "");
        assert_eq!(capitalise_first("a"), "A");
    }

    // --- new property-based style tests for the regex engine ---

    #[test]
    fn test_regex_tolerates_extra_whitespace() {
        let input = "   0  :  func[99]  @  0xff  ";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 1);
        assert_eq!(frames[0].func_index, Some(99));
        assert_eq!(frames[0].wasm_offset, Some(0xff));
    }

    #[test]
    fn test_regex_hash_prefix_index() {
        let input = "  #3: func[2] @ 0x10";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 1);
        assert_eq!(frames[0].index, 3);
    }

    #[test]
    fn test_regex_named_frame_no_offset() {
        let input = "  0: my_contract::do_thing";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 1);
        assert_eq!(
            frames[0].func_name.as_deref(),
            Some("my_contract::do_thing")
        );
        assert!(frames[0].wasm_offset.is_none());
    }

    #[test]
    fn test_regex_garbage_lines_skipped() {
        let input = "random noise\n  0: func[1] @ 0x1\nmore noise";
        let frames = extract_frames(input, None);
        assert_eq!(frames.len(), 1);
    }

    #[test]
    fn test_extract_trapping_offset_formats() {
        assert_eq!(
            extract_trapping_offset("  0: func[42] @ 0xa3c"),
            Some(0xa3c)
        );
        assert_eq!(
            extract_trapping_offset("wasm trap: unreachable, code offset 0x1000"),
            Some(0x1000)
        );
        assert_eq!(
            extract_trapping_offset("trapping instruction code offset: 0x200"),
            Some(0x200)
        );
        assert_eq!(extract_trapping_offset("wasm offset: 0x50"), Some(0x50));
        assert_eq!(
            extract_trapping_offset("contract trapped at offset 0x30"),
            Some(0x30)
        );
        assert_eq!(extract_trapping_offset("offset: 0x40"), Some(0x40));
        assert_eq!(extract_trapping_offset("assertion failed"), None);
    }

    #[test]
    fn test_host_error_with_trapping_offset_resolves_dwarf_line() {
        let mapper = SourceMapper::with_mappings(vec![(
            0x1000,
            Some(0x1010),
            SourceLocation {
                file: "src/contract.rs".to_string(),
                line: 42,
                column: Some(10),
                column_end: None,
                github_link: None,
            },
        )]);

        let input = "HostError: Error(WasmVm, Trap(Unreachable))\n  0: my_contract::panic_with_snapshot\n  1: soroban_sdk::invoke @ 0x200\ntrapping instruction code offset: 0x1000";
        let trace = WasmStackTrace::from_host_error(input, Some(&mapper));

        assert_eq!(trace.frames.len(), 2);
        assert_eq!(trace.frames[0].index, 0);
        assert_eq!(
            trace.frames[0].func_name.as_deref(),
            Some("my_contract::panic_with_snapshot")
        );
        assert_eq!(trace.frames[0].wasm_offset, Some(0x1000));

        let loc = trace.frames[0]
            .source_location
            .as_ref()
            .expect("top frame must have source location");
        assert_eq!(loc.file, "src/contract.rs");
        assert_eq!(loc.line, 42);
        assert_eq!(loc.column, Some(10));

        let display = trace.display();
        assert!(display.contains("my_contract::panic_with_snapshot (src/contract.rs:42:10)"));
    }

    #[test]
    fn test_panic_with_trapping_offset_resolves_dwarf_line() {
        let mapper = SourceMapper::with_mappings(vec![(
            0x1000,
            Some(0x1010),
            SourceLocation {
                file: "src/panic.rs".to_string(),
                line: 39,
                column: Some(5),
                column_end: None,
                github_link: None,
            },
        )]);

        let panic_msg = "VM call trapped: UnreachableCodeReached, code offset 0x1000";
        let mut trace = WasmStackTrace::from_panic(panic_msg);
        assert_eq!(trace.frames.len(), 1);
        assert_eq!(trace.frames[0].wasm_offset, Some(0x1000));

        trace.resolve_sources(&mapper);

        let loc = trace.frames[0]
            .source_location
            .as_ref()
            .expect("top frame must have source location after resolve_sources");
        assert_eq!(loc.file, "src/panic.rs");
        assert_eq!(loc.line, 39);
        assert_eq!(loc.column, Some(5));

        let display = trace.display();
        assert!(display.contains("(src/panic.rs:39:5)"));
    }

    #[test]
    fn test_from_panic_with_mapper_directly() {
        let mapper = SourceMapper::with_mappings(vec![(
            0x500,
            Some(0x520),
            SourceLocation {
                file: "src/token.rs".to_string(),
                line: 100,
                column: Some(8),
                column_end: None,
                github_link: None,
            },
        )]);

        let trace = WasmStackTrace::from_panic_with_mapper(
            "contract execution trapped at offset 0x500",
            Some(&mapper),
        );
        assert_eq!(trace.frames.len(), 1);
        assert_eq!(trace.frames[0].wasm_offset, Some(0x500));
        let loc = trace.frames[0].source_location.as_ref().unwrap();
        assert_eq!(loc.file, "src/token.rs");
        assert_eq!(loc.line, 100);
        assert_eq!(loc.column, Some(8));
        assert!(trace.display().contains("(src/token.rs:100:8)"));
    }

    #[test]
    fn test_panic_with_direct_source_location_in_message() {
        let panic_msg = "panicked at 'explicit panic', src/lib.rs:15:9";
        let trace = WasmStackTrace::from_panic(panic_msg);
        assert_eq!(trace.frames.len(), 1);
        let loc = trace.frames[0].source_location.as_ref().unwrap();
        assert_eq!(loc.file, "src/lib.rs");
        assert_eq!(loc.line, 15);
        assert_eq!(loc.column, Some(9));
        assert!(trace.display().contains("(src/lib.rs:15:9)"));
    }

    #[test]
    fn test_display_with_and_without_column() {
        let trace_with_col = WasmStackTrace {
            trap_kind: TrapKind::Unreachable,
            raw_message: "test".to_string(),
            frames: vec![StackFrame {
                index: 0,
                func_index: None,
                func_name: Some("test_func".to_string()),
                wasm_offset: Some(0x10),
                module: None,
                source_location: Some(SourceLocation {
                    file: "test.rs".to_string(),
                    line: 20,
                    column: Some(3),
                    column_end: None,
                    github_link: None,
                }),
            }],
            soroban_wrapped: false,
        };
        assert!(trace_with_col
            .display()
            .contains("test_func (test.rs:20:3)"));

        let trace_without_col = WasmStackTrace {
            trap_kind: TrapKind::Unreachable,
            raw_message: "test".to_string(),
            frames: vec![StackFrame {
                index: 0,
                func_index: None,
                func_name: Some("test_func".to_string()),
                wasm_offset: Some(0x10),
                module: None,
                source_location: Some(SourceLocation {
                    file: "test.rs".to_string(),
                    line: 20,
                    column: None,
                    column_end: None,
                    github_link: None,
                }),
            }],
            soroban_wrapped: false,
        };
        assert!(trace_without_col
            .display()
            .contains("test_func (test.rs:20)"));
    }
}
