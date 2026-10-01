// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod tests {
    use rand::{Rng, SeedableRng, rngs::StdRng};

    #[test]
    fn test_endianness_consistency_fuzz() {
        let mut rng = StdRng::seed_from_u64(42);

        for _ in 0..10_000 {
            let val_u32: u32 = rng.gen();
            let val_u64: u64 = rng.gen();

            // Check to_le_bytes
            let le_u32 = val_u32.to_le_bytes();
            let expected_le_u32 = [
                (val_u32 & 0xFF) as u8,
                ((val_u32 >> 8) & 0xFF) as u8,
                ((val_u32 >> 16) & 0xFF) as u8,
                ((val_u32 >> 24) & 0xFF) as u8,
            ];
            assert_eq!(le_u32, expected_le_u32);

            let be_u32 = val_u32.to_be_bytes();
            let expected_be_u32 = [
                ((val_u32 >> 24) & 0xFF) as u8,
                ((val_u32 >> 16) & 0xFF) as u8,
                ((val_u32 >> 8) & 0xFF) as u8,
                (val_u32 & 0xFF) as u8,
            ];
            assert_eq!(be_u32, expected_be_u32);
            
            // Reconstruct and verify
            assert_eq!(u32::from_le_bytes(le_u32), val_u32);
            assert_eq!(u32::from_be_bytes(be_u32), val_u32);

            let le_u64 = val_u64.to_le_bytes();
            let expected_le_u64 = [
                (val_u64 & 0xFF) as u8,
                ((val_u64 >> 8) & 0xFF) as u8,
                ((val_u64 >> 16) & 0xFF) as u8,
                ((val_u64 >> 24) & 0xFF) as u8,
                ((val_u64 >> 32) & 0xFF) as u8,
                ((val_u64 >> 40) & 0xFF) as u8,
                ((val_u64 >> 48) & 0xFF) as u8,
                ((val_u64 >> 56) & 0xFF) as u8,
            ];
            assert_eq!(le_u64, expected_le_u64);

            let be_u64 = val_u64.to_be_bytes();
            let expected_be_u64 = [
                ((val_u64 >> 56) & 0xFF) as u8,
                ((val_u64 >> 48) & 0xFF) as u8,
                ((val_u64 >> 40) & 0xFF) as u8,
                ((val_u64 >> 32) & 0xFF) as u8,
                ((val_u64 >> 24) & 0xFF) as u8,
                ((val_u64 >> 16) & 0xFF) as u8,
                ((val_u64 >> 8) & 0xFF) as u8,
                (val_u64 & 0xFF) as u8,
            ];
            assert_eq!(be_u64, expected_be_u64);

            // Reconstruct and verify
            assert_eq!(u64::from_le_bytes(le_u64), val_u64);
            assert_eq!(u64::from_be_bytes(be_u64), val_u64);
        }
    }
}
