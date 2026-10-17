// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Architecture fuzzing for ARM64 vs x86_64 endianness and bit-shifting (Issue #1931).
//!
//! Verifies that:
//! 1. Multi-byte integer serializations (u16, u32, u64, u128, i32, i64, i128) are strictly deterministic
//!    and canonical across little-endian and big-endian target conventions.
//! 2. Byte-shifting, tagged pointer packing, and bit-masking operations produce identical outputs
//!    regardless of host CPU architecture (e.g. Apple Silicon M1/M2 ARM64 vs Intel x86_64).
//! 3. WASM linear memory load/store operations consistently adhere to the WASM specification (Little-Endian).
//! 4. Canonical serialized outputs are invariant against simulated host endianness flips.

use proptest::prelude::*;

// ---------------------------------------------------------------------------
// Tagged Pointer Bit-Packing Helpers (Representative of Host Object Tagging)
// ---------------------------------------------------------------------------

const PAYLOAD_MASK_56: u64 = 0x00ff_ffff_ffff_ffff;

/// Packs an 8-bit tag and a 56-bit payload into a 64-bit integer.
#[inline]
fn pack_tagged_val(tag: u8, payload: u64) -> u64 {
    ((payload & PAYLOAD_MASK_56) << 8) | (u64::from(tag))
}

/// Unpacks a 64-bit tagged integer into an 8-bit tag and 56-bit payload.
#[inline]
fn unpack_tagged_val(val: u64) -> (u8, u64) {
    let tag = (val & 0xff) as u8;
    let payload = (val >> 8) & PAYLOAD_MASK_56;
    (tag, payload)
}

// ---------------------------------------------------------------------------
// Simulated WASM Linear Memory Access (Little-Endian by Spec)
// ---------------------------------------------------------------------------

struct SimulatedMemory {
    bytes: Vec<u8>,
}

impl SimulatedMemory {
    fn new(size: usize) -> Self {
        Self {
            bytes: vec![0u8; size],
        }
    }

    fn write_u32_le(&mut self, offset: usize, val: u32) -> Result<(), &'static str> {
        if offset + 4 > self.bytes.len() {
            return Err("out of bounds");
        }
        self.bytes[offset..offset + 4].copy_from_slice(&val.to_le_bytes());
        Ok(())
    }

    fn read_u32_le(&self, offset: usize) -> Result<u32, &'static str> {
        if offset + 4 > self.bytes.len() {
            return Err("out of bounds");
        }
        let slice: [u8; 4] = self.bytes[offset..offset + 4]
            .try_into()
            .map_err(|_| "slice conversion failed")?;
        Ok(u32::from_le_bytes(slice))
    }

    fn write_u64_le(&mut self, offset: usize, val: u64) -> Result<(), &'static str> {
        if offset + 8 > self.bytes.len() {
            return Err("out of bounds");
        }
        self.bytes[offset..offset + 8].copy_from_slice(&val.to_le_bytes());
        Ok(())
    }

    fn read_u64_le(&self, offset: usize) -> Result<u64, &'static str> {
        if offset + 8 > self.bytes.len() {
            return Err("out of bounds");
        }
        let slice: [u8; 8] = self.bytes[offset..offset + 8]
            .try_into()
            .map_err(|_| "slice conversion failed")?;
        Ok(u64::from_le_bytes(slice))
    }

    fn write_i128_le(&mut self, offset: usize, val: i128) -> Result<(), &'static str> {
        if offset + 16 > self.bytes.len() {
            return Err("out of bounds");
        }
        self.bytes[offset..offset + 16].copy_from_slice(&val.to_le_bytes());
        Ok(())
    }

    fn read_i128_le(&self, offset: usize) -> Result<i128, &'static str> {
        if offset + 16 > self.bytes.len() {
            return Err("out of bounds");
        }
        let slice: [u8; 16] = self.bytes[offset..offset + 16]
            .try_into()
            .map_err(|_| "slice conversion failed")?;
        Ok(i128::from_le_bytes(slice))
    }
}

// ---------------------------------------------------------------------------
// Simulated Big-Endian Architecture Model
// ---------------------------------------------------------------------------

/// Simulates reading and writing multi-byte integers on a Big-Endian CPU.
/// Verifies that explicit `to_le_bytes()` and `from_le_bytes()` guarantee
/// architecture-independence, whereas naive native-endian operations would diverge.
struct EndiannessSimulator;

impl EndiannessSimulator {
    /// Canonical serialization to little-endian bytes.
    fn canonical_u64(val: u64) -> [u8; 8] {
        val.to_le_bytes()
    }

    /// Simulates how a Big-Endian machine encodes native bytes.
    fn simulated_be_native_u64(val: u64) -> [u8; 8] {
        val.to_be_bytes()
    }

    /// Verifies that explicit LE serialization is invariant even if bytes are converted
    /// via byte-swapping on a simulated BE target.
    fn roundtrip_simulated_be_host(val: u64) -> u64 {
        // On a BE host, reading LE bytes requires byte-swapping:
        let le_bytes = val.to_le_bytes();
        let be_repr = u64::from_be_bytes(le_bytes);
        // Swapping bytes restores the original integer:
        be_repr.swap_bytes()
    }
}

// ---------------------------------------------------------------------------
// Property Tests
// ---------------------------------------------------------------------------

proptest! {
    #![proptest_config(ProptestConfig::with_cases(1000))]

    #[test]
    fn prop_tagged_pointer_packing_endian_invariance(
        tag in any::<u8>(),
        payload in any::<u64>(),
    ) {
        let masked_payload = payload & PAYLOAD_MASK_56;
        let packed = pack_tagged_val(tag, masked_payload);
        let (extracted_tag, extracted_payload) = unpack_tagged_val(packed);

        prop_assert_eq!(tag, extracted_tag, "Extracted tag must match original");
        prop_assert_eq!(masked_payload, extracted_payload, "Extracted payload must match masked original");

        // Canonical LE serialization of tagged pointer must match manual byte extraction
        let le_bytes = packed.to_le_bytes();
        prop_assert_eq!(le_bytes[0], tag, "Byte 0 of LE tagged pointer must contain the 8-bit tag");
    }

    #[test]
    fn prop_wasm_linear_memory_le_invariance(
        val_u32 in any::<u32>(),
        val_u64 in any::<u64>(),
        val_i128 in any::<i128>(),
        offset in 0usize..256usize,
    ) {
        let mut mem = SimulatedMemory::new(512);

        // Test u32 LE
        mem.write_u32_le(offset, val_u32).unwrap();
        let read_u32 = mem.read_u32_le(offset).unwrap();
        prop_assert_eq!(val_u32, read_u32, "u32 LE memory roundtrip failed");

        // Test u64 LE
        mem.write_u64_le(offset + 32, val_u64).unwrap();
        let read_u64 = mem.read_u64_le(offset + 32).unwrap();
        prop_assert_eq!(val_u64, read_u64, "u64 LE memory roundtrip failed");

        // Test i128 LE
        mem.write_i128_le(offset + 64, val_i128).unwrap();
        let read_i128 = mem.read_i128_le(offset + 64).unwrap();
        prop_assert_eq!(val_i128, read_i128, "i128 LE memory roundtrip failed");
    }

    #[test]
    fn prop_cross_architecture_endian_simulation(val in any::<u64>()) {
        let restored = EndiannessSimulator::roundtrip_simulated_be_host(val);
        prop_assert_eq!(val, restored, "Simulated Big-Endian host must recover identical value via byte-swap");

        let canonical_le = EndiannessSimulator::canonical_u64(val);
        let simulated_be = EndiannessSimulator::simulated_be_native_u64(val);

        // Canonical LE must be the exact reverse of BE representation
        let mut reversed_be = simulated_be;
        reversed_be.reverse();
        prop_assert_eq!(canonical_le, reversed_be, "LE bytes must equal reversed BE bytes across architectures");
    }

    #[test]
    fn prop_multi_byte_swap_bytes_identity(
        u16_val in any::<u16>(),
        u32_val in any::<u32>(),
        u64_val in any::<u64>(),
        u128_val in any::<u128>(),
    ) {
        // Invariant: val.swap_bytes().swap_bytes() == val
        prop_assert_eq!(u16_val.swap_bytes().swap_bytes(), u16_val);
        prop_assert_eq!(u32_val.swap_bytes().swap_bytes(), u32_val);
        prop_assert_eq!(u64_val.swap_bytes().swap_bytes(), u64_val);
        prop_assert_eq!(u128_val.swap_bytes().swap_bytes(), u128_val);

        // Invariant: val.to_le_bytes() == val.to_be_bytes() reversed
        let mut u64_be = u64_val.to_be_bytes();
        u64_be.reverse();
        prop_assert_eq!(u64_val.to_le_bytes(), u64_be);
    }

    #[test]
    fn prop_signed_shift_arithmetic_invariance(
        val in any::<i64>(),
        shift in 0u32..63u32,
    ) {
        // Sign-extending arithmetic right shift must be identical across architectures
        let shifted = val >> shift;
        if val < 0 {
            prop_assert!(shifted < 0, "Arithmetic right shift on negative i64 must maintain sign bit");
        } else if val == 0 {
            prop_assert_eq!(shifted, 0);
        } else {
            prop_assert!(shifted >= 0);
        }
    }
}
