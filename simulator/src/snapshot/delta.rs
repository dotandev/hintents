// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

//! Memory delta application and rollback utilities for snapshot linear memory buffers.
//!
//! Provides functions to:
//! - Compute chunked memory deltas between previous and current memory buffers
//! - Apply memory deltas in forward direction
//! - Rollback memory deltas (reverse application) to restore previous buffer state
//! - Apply deltas directly onto existing snapshots

use serde::{Deserialize, Serialize};

/// Represents a contiguous range of changed bytes within a linear memory buffer.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MemoryChunkDelta {
    /// Zero-based byte offset in the linear memory buffer where the change begins.
    pub offset: usize,
    /// The original bytes before the change occurred (used for rollback).
    pub old_bytes: Vec<u8>,
    /// The new bytes after the change occurred (used for forward application).
    pub new_bytes: Vec<u8>,
}

/// Represents a set of contiguous delta chunks representing changes between two memory states.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct MemoryDelta {
    /// Original buffer length before changes.
    pub original_len: usize,
    /// Target buffer length after changes.
    pub target_len: usize,
    /// List of modified contiguous chunks.
    pub chunks: Vec<MemoryChunkDelta>,
}

impl MemoryDelta {
    /// Creates an empty memory delta for equal memory buffers of length `len`.
    pub fn empty(len: usize) -> Self {
        Self {
            original_len: len,
            target_len: len,
            chunks: Vec::new(),
        }
    }

    /// Computes a `MemoryDelta` from `old_mem` to `new_mem`.
    ///
    /// Identifies contiguous regions of differing bytes between the two slices,
    /// handling insertions, replacements, and length expansions or truncations.
    pub fn compute(old_mem: &[u8], new_mem: &[u8]) -> Self {
        let mut chunks = Vec::new();
        let max_len = old_mem.len().max(new_mem.len());
        let mut i = 0;

        while i < max_len {
            let old_byte = old_mem.get(i).copied();
            let new_byte = new_mem.get(i).copied();

            if old_byte != new_byte {
                let start = i;
                let mut old_chunk = Vec::new();
                let mut new_chunk = Vec::new();

                while i < max_len {
                    let ob = old_mem.get(i).copied();
                    let nb = new_mem.get(i).copied();

                    if ob == nb {
                        break;
                    }

                    if let Some(b) = ob {
                        old_chunk.push(b);
                    }
                    if let Some(b) = nb {
                        new_chunk.push(b);
                    }
                    i += 1;
                }

                chunks.push(MemoryChunkDelta {
                    offset: start,
                    old_bytes: old_chunk,
                    new_bytes: new_chunk,
                });
            } else {
                i += 1;
            }
        }

        Self {
            original_len: old_mem.len(),
            target_len: new_mem.len(),
            chunks,
        }
    }

    /// Encodes the delta into a compact binary format.
    ///
    /// Binary format:
    /// - `original_len: u64` (little endian)
    /// - `target_len: u64` (little endian)
    /// - `num_chunks: u64` (little endian)
    /// - For each chunk:
    ///   - `offset: u64` (little endian)
    ///   - `old_len: u64` (little endian)
    ///   - `old_bytes: [u8]`
    ///   - `new_len: u64` (little endian)
    ///   - `new_bytes: [u8]`
    pub fn to_bytes(&self) -> Vec<u8> {
        let mut out = Vec::new();
        out.extend_from_slice(&(self.original_len as u64).to_le_bytes());
        out.extend_from_slice(&(self.target_len as u64).to_le_bytes());
        out.extend_from_slice(&(self.chunks.len() as u64).to_le_bytes());

        for chunk in &self.chunks {
            out.extend_from_slice(&(chunk.offset as u64).to_le_bytes());
            out.extend_from_slice(&(chunk.old_bytes.len() as u64).to_le_bytes());
            out.extend_from_slice(&chunk.old_bytes);
            out.extend_from_slice(&(chunk.new_bytes.len() as u64).to_le_bytes());
            out.extend_from_slice(&chunk.new_bytes);
        }

        out
    }

    /// Decodes a `MemoryDelta` from binary format.
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, String> {
        if bytes.len() < 24 {
            return Err("Binary delta header too short".to_string());
        }

        let original_len = u64::from_le_bytes(
            bytes[0..8]
                .try_into()
                .map_err(|_| "Failed to read original_len")?,
        ) as usize;
        let target_len = u64::from_le_bytes(
            bytes[8..16]
                .try_into()
                .map_err(|_| "Failed to read target_len")?,
        ) as usize;
        let num_chunks = u64::from_le_bytes(
            bytes[16..24]
                .try_into()
                .map_err(|_| "Failed to read num_chunks")?,
        ) as usize;

        let mut offset = 24;
        let mut chunks = Vec::with_capacity(num_chunks);

        for _ in 0..num_chunks {
            if offset + 24 > bytes.len() {
                return Err("Unexpected EOF while parsing chunk header".to_string());
            }

            let chunk_offset = u64::from_le_bytes(
                bytes[offset..offset + 8]
                    .try_into()
                    .map_err(|_| "Failed to read chunk offset")?,
            ) as usize;
            offset += 8;

            let old_len = u64::from_le_bytes(
                bytes[offset..offset + 8]
                    .try_into()
                    .map_err(|_| "Failed to read old_len")?,
            ) as usize;
            offset += 8;

            if offset + old_len > bytes.len() {
                return Err("Unexpected EOF while parsing old_bytes".to_string());
            }
            let old_bytes = bytes[offset..offset + old_len].to_vec();
            offset += old_len;

            if offset + 8 > bytes.len() {
                return Err("Unexpected EOF while reading new_len".to_string());
            }
            let new_len = u64::from_le_bytes(
                bytes[offset..offset + 8]
                    .try_into()
                    .map_err(|_| "Failed to read new_len")?,
            ) as usize;
            offset += 8;

            if offset + new_len > bytes.len() {
                return Err("Unexpected EOF while parsing new_bytes".to_string());
            }
            let new_bytes = bytes[offset..offset + new_len].to_vec();
            offset += new_len;

            chunks.push(MemoryChunkDelta {
                offset: chunk_offset,
                old_bytes,
                new_bytes,
            });
        }

        Ok(Self {
            original_len,
            target_len,
            chunks,
        })
    }

    /// Returns `true` if there are no differences between original and target buffers.
    pub fn is_empty(&self) -> bool {
        self.original_len == self.target_len && self.chunks.is_empty()
    }

    /// Returns the total number of modified bytes across all chunks (based on new_bytes).
    pub fn total_changed_bytes(&self) -> usize {
        self.chunks.iter().map(|c| c.new_bytes.len().max(c.old_bytes.len())).sum()
    }
}

/// Applies memory deltas in the forward direction on `buffer`.
///
/// Modifies `buffer` in-place to match the target memory state.
///
/// # Errors
/// Returns an error if chunk offsets or expected lengths are inconsistent with `buffer`.
pub fn apply_delta(buffer: &mut Vec<u8>, delta: &MemoryDelta) -> Result<(), String> {
    if buffer.len() != delta.original_len {
        return Err(format!(
            "Buffer length mismatch: expected {}, got {}",
            delta.original_len,
            buffer.len()
        ));
    }

    // Resize buffer to target_len if needed
    if buffer.len() < delta.target_len {
        buffer.resize(delta.target_len, 0);
    }

    for chunk in &delta.chunks {
        // Verify chunk matches existing old_bytes in buffer
        let end_old = chunk.offset + chunk.old_bytes.len();
        if end_old > delta.original_len {
            return Err(format!(
                "Delta chunk out of bounds for original buffer: offset {} + old_len {} > {}",
                chunk.offset,
                chunk.old_bytes.len(),
                delta.original_len
            ));
        }

        if !chunk.old_bytes.is_empty() && &buffer[chunk.offset..end_old] != chunk.old_bytes.as_slice() {
            return Err(format!(
                "Delta old_bytes mismatch at offset {}: expected {:?}, found {:?}",
                chunk.offset,
                chunk.old_bytes,
                &buffer[chunk.offset..end_old]
            ));
        }

        // Overwrite / append with new_bytes
        let end_new = chunk.offset + chunk.new_bytes.len();
        if end_new > buffer.len() {
            buffer.resize(end_new, 0);
        }
        buffer[chunk.offset..end_new].copy_from_slice(&chunk.new_bytes);
    }

    // Truncate to final target_len if target was shorter than original
    buffer.truncate(delta.target_len);

    Ok(())
}

/// Reverses (rolls back) memory deltas from `buffer`.
///
/// Modifies `buffer` in-place, rolling back from `target_len` state to `original_len` state.
///
/// # Errors
/// Returns an error if chunk offsets or expected new_bytes are inconsistent with `buffer`.
pub fn rollback_delta(buffer: &mut Vec<u8>, delta: &MemoryDelta) -> Result<(), String> {
    if buffer.len() != delta.target_len {
        return Err(format!(
            "Buffer length mismatch for rollback: expected target_len {}, got {}",
            delta.target_len,
            buffer.len()
        ));
    }

    // Resize buffer to original_len if original was larger
    if buffer.len() < delta.original_len {
        buffer.resize(delta.original_len, 0);
    }

    // Apply chunk old_bytes in reverse order
    for chunk in delta.chunks.iter().rev() {
        let end_new = chunk.offset + chunk.new_bytes.len();
        if end_new > delta.target_len {
            return Err(format!(
                "Delta chunk out of bounds for target buffer: offset {} + new_len {} > {}",
                chunk.offset,
                chunk.new_bytes.len(),
                delta.target_len
            ));
        }

        if !chunk.new_bytes.is_empty() && &buffer[chunk.offset..end_new] != chunk.new_bytes.as_slice() {
            return Err(format!(
                "Delta rollback new_bytes mismatch at offset {}: expected {:?}, found {:?}",
                chunk.offset,
                chunk.new_bytes,
                &buffer[chunk.offset..end_new]
            ));
        }

        let end_old = chunk.offset + chunk.old_bytes.len();
        if end_old > buffer.len() {
            buffer.resize(end_old, 0);
        }
        buffer[chunk.offset..end_old].copy_from_slice(&chunk.old_bytes);
    }

    // Truncate to final original_len
    buffer.truncate(delta.original_len);

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_compute_empty_delta() {
        let mem = vec![1, 2, 3, 4, 5];
        let delta = MemoryDelta::compute(&mem, &mem);
        assert!(delta.is_empty());
        assert_eq!(delta.chunks.len(), 0);
        assert_eq!(delta.original_len, 5);
        assert_eq!(delta.target_len, 5);
    }

    #[test]
    fn test_apply_and_rollback_single_chunk() {
        let initial = vec![0, 1, 2, 3, 4, 5, 6, 7];
        let mut modified = initial.clone();
        modified[2] = 0xAA;
        modified[3] = 0xBB;

        let delta = MemoryDelta::compute(&initial, &modified);
        assert_eq!(delta.chunks.len(), 1);
        assert_eq!(delta.chunks[0].offset, 2);
        assert_eq!(delta.chunks[0].old_bytes, vec![2, 3]);
        assert_eq!(delta.chunks[0].new_bytes, vec![0xAA, 0xBB]);

        // Forward application
        let mut buffer = initial.clone();
        apply_delta(&mut buffer, &delta).expect("apply should succeed");
        assert_eq!(buffer, modified);

        // Rollback
        rollback_delta(&mut buffer, &delta).expect("rollback should succeed");
        assert_eq!(buffer, initial);
    }

    #[test]
    fn test_apply_and_rollback_multiple_disjoint_chunks() {
        let initial = vec![0u8; 32];
        let mut modified = initial.clone();
        modified[4..8].copy_from_slice(&[1, 2, 3, 4]);
        modified[16..18].copy_from_slice(&[99, 100]);
        modified[30] = 0xFF;

        let delta = MemoryDelta::compute(&initial, &modified);
        assert_eq!(delta.chunks.len(), 3);

        let mut buffer = initial.clone();
        apply_delta(&mut buffer, &delta).expect("apply should succeed");
        assert_eq!(buffer, modified);

        rollback_delta(&mut buffer, &delta).expect("rollback should succeed");
        assert_eq!(buffer, initial);
    }

    #[test]
    fn test_apply_and_rollback_buffer_expansion() {
        let initial = vec![1, 2, 3];
        let modified = vec![1, 2, 3, 4, 5, 6, 7];

        let delta = MemoryDelta::compute(&initial, &modified);
        assert_eq!(delta.original_len, 3);
        assert_eq!(delta.target_len, 7);

        let mut buffer = initial.clone();
        apply_delta(&mut buffer, &delta).expect("apply should expand buffer");
        assert_eq!(buffer, modified);

        rollback_delta(&mut buffer, &delta).expect("rollback should shrink buffer");
        assert_eq!(buffer, initial);
    }

    #[test]
    fn test_apply_and_rollback_buffer_shrink() {
        let initial = vec![1, 2, 3, 4, 5, 6, 7, 8];
        let modified = vec![1, 2, 0xFF];

        let delta = MemoryDelta::compute(&initial, &modified);
        assert_eq!(delta.original_len, 8);
        assert_eq!(delta.target_len, 3);

        let mut buffer = initial.clone();
        apply_delta(&mut buffer, &delta).expect("apply should shrink buffer");
        assert_eq!(buffer, modified);

        rollback_delta(&mut buffer, &delta).expect("rollback should restore buffer");
        assert_eq!(buffer, initial);
    }

    #[test]
    fn test_binary_roundtrip() {
        let old_mem = vec![0x10, 0x20, 0x30, 0x40, 0x50];
        let new_mem = vec![0x10, 0x99, 0x88, 0x40, 0x50, 0x60, 0x70];

        let delta = MemoryDelta::compute(&old_mem, &new_mem);
        let bytes = delta.to_bytes();
        let decoded = MemoryDelta::from_bytes(&bytes).expect("decoding binary delta should succeed");

        assert_eq!(delta, decoded);

        let mut buf = old_mem.clone();
        apply_delta(&mut buf, &decoded).expect("apply decoded delta");
        assert_eq!(buf, new_mem);

        rollback_delta(&mut buf, &decoded).expect("rollback decoded delta");
        assert_eq!(buf, old_mem);
    }

    #[test]
    fn test_apply_delta_length_mismatch_fails() {
        let old_mem = vec![1, 2, 3];
        let new_mem = vec![1, 4, 3];
        let delta = MemoryDelta::compute(&old_mem, &new_mem);

        let mut invalid_buf = vec![1, 2];
        let result = apply_delta(&mut invalid_buf, &delta);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Buffer length mismatch"));
    }

    #[test]
    fn test_apply_delta_content_mismatch_fails() {
        let old_mem = vec![1, 2, 3];
        let new_mem = vec![1, 4, 3];
        let delta = MemoryDelta::compute(&old_mem, &new_mem);

        let mut mismatched_buf = vec![1, 9, 3];
        let result = apply_delta(&mut mismatched_buf, &delta);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Delta old_bytes mismatch"));
    }

    #[test]
    fn test_rollback_delta_target_length_mismatch_fails() {
        let old_mem = vec![1, 2, 3];
        let new_mem = vec![1, 4, 3, 5];
        let delta = MemoryDelta::compute(&old_mem, &new_mem);

        let mut wrong_target_buf = vec![1, 4, 3];
        let result = rollback_delta(&mut wrong_target_buf, &delta);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Buffer length mismatch for rollback"));
    }

    #[test]
    fn test_rollback_delta_content_mismatch_fails() {
        let old_mem = vec![1, 2, 3];
        let new_mem = vec![1, 4, 3];
        let delta = MemoryDelta::compute(&old_mem, &new_mem);

        let mut corrupted_buf = vec![1, 9, 3];
        let result = rollback_delta(&mut corrupted_buf, &delta);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Delta rollback new_bytes mismatch"));
    }

    #[test]
    fn test_integration_sequential_snapshot_deltas_forward_and_rewind() {
        // Simulates contract execution across multiple steps generating linear memory changes
        let step0 = vec![0u8; 64];

        // Step 1: writes contract header
        let mut step1 = step0.clone();
        step1[0..4].copy_from_slice(&[0xDE, 0xAD, 0xBE, 0xEF]);
        step1[16..20].copy_from_slice(&[1, 0, 0, 0]);

        // Step 2: allocates memory and writes data payload
        let mut step2 = step1.clone();
        step2.resize(128, 0);
        step2[64..68].copy_from_slice(&[0xCA, 0xFE, 0xBA, 0xBE]);
        step2[16..20].copy_from_slice(&[2, 0, 0, 0]); // sequence counter increment

        // Step 3: mutates existing payload and writes tail
        let mut step3 = step2.clone();
        step3[64..68].copy_from_slice(&[0x11, 0x22, 0x33, 0x44]);
        step3[120..128].copy_from_slice(&[9, 9, 9, 9, 9, 9, 9, 9]);

        let delta_0_to_1 = MemoryDelta::compute(&step0, &step1);
        let delta_1_to_2 = MemoryDelta::compute(&step1, &step2);
        let delta_2_to_3 = MemoryDelta::compute(&step2, &step3);

        // Forward replay from step0 to step3
        let mut state = step0.clone();
        apply_delta(&mut state, &delta_0_to_1).expect("apply step 1");
        assert_eq!(state, step1);

        apply_delta(&mut state, &delta_1_to_2).expect("apply step 2");
        assert_eq!(state, step2);

        apply_delta(&mut state, &delta_2_to_3).expect("apply step 3");
        assert_eq!(state, step3);

        // Rewind from step3 back to step0
        rollback_delta(&mut state, &delta_2_to_3).expect("rollback step 3 -> 2");
        assert_eq!(state, step2);

        rollback_delta(&mut state, &delta_1_to_2).expect("rollback step 2 -> 1");
        assert_eq!(state, step1);

        rollback_delta(&mut state, &delta_0_to_1).expect("rollback step 1 -> 0");
        assert_eq!(state, step0);
    }

    #[test]
    fn test_integration_branching_and_rollback_recovery() {
        // Base state before transaction branch
        let base_state = vec![0x55u8; 100];

        // Branch A execution path
        let mut branch_a = base_state.clone();
        branch_a[10..20].fill(0xAA);
        let delta_a = MemoryDelta::compute(&base_state, &branch_a);

        // Branch B execution path (alternate transaction branch)
        let mut branch_b = base_state.clone();
        branch_b[10..20].fill(0xBB);
        branch_b.resize(150, 0xCC);
        let delta_b = MemoryDelta::compute(&base_state, &branch_b);

        // Execute Branch A
        let mut live_memory = base_state.clone();
        apply_delta(&mut live_memory, &delta_a).expect("apply branch A");
        assert_eq!(live_memory, branch_a);

        // Rollback Branch A to base state
        rollback_delta(&mut live_memory, &delta_a).expect("rollback branch A");
        assert_eq!(live_memory, base_state);

        // Replay Branch B from recovered base state
        apply_delta(&mut live_memory, &delta_b).expect("apply branch B");
        assert_eq!(live_memory, branch_b);

        // Rollback Branch B
        rollback_delta(&mut live_memory, &delta_b).expect("rollback branch B");
        assert_eq!(live_memory, base_state);
    }

    #[test]
    fn test_integration_large_buffer_sparse_deltas_stress() {
        // 64KB WASM page
        let page_size = 65536;
        let original_page = vec![0u8; page_size];

        let mut modified_page = original_page.clone();
        // Modify various sparse memory locations
        modified_page[0] = 0x42;
        modified_page[1024..1028].copy_from_slice(&[1, 2, 3, 4]);
        modified_page[32768..32772].copy_from_slice(&[0xAA, 0xBB, 0xCC, 0xDD]);
        modified_page[page_size - 1] = 0xFF;

        let delta = MemoryDelta::compute(&original_page, &modified_page);
        assert_eq!(delta.chunks.len(), 4);
        assert_eq!(delta.total_changed_bytes(), 10);

        let mut buf = original_page.clone();
        apply_delta(&mut buf, &delta).expect("apply sparse 64KB delta");
        assert_eq!(buf, modified_page);

        rollback_delta(&mut buf, &delta).expect("rollback sparse 64KB delta");
        assert_eq!(buf, original_page);
    }
}
