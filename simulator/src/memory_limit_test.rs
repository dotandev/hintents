// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Test module for memory limit simulation functionality

#[cfg(test)]
mod tests {
    use crate::runner::SimHost;
    use crate::types::ResourceCalibration;
    use std::panic;

    #[test]
    fn test_memory_limit_field() {
        // Test that SimHost can be created with a memory limit
        let memory_limit = Some(1000000); // 1MB limit
        let host = SimHost::new(None, None, memory_limit);

        assert_eq!(host.memory_limit, memory_limit);
    }

    #[test]
    fn test_no_memory_limit() {
        // Test that SimHost can be created without memory limit
        let host = SimHost::new(None, None, None);

        assert_eq!(host.memory_limit, None);
    }

    #[test]
    fn test_memory_limit_check_no_panic() {
        // Test memory limit checking functionality when within limits
        let memory_limit = Some(1000); // Very small limit
        let host = SimHost::new(None, None, memory_limit);

        // This should not panic as we haven't executed any operations yet
        host.check_memory_limit();
    }

    #[test]
    fn test_memory_limit_exceeded_does_not_propagate_panic() {
        // Use catch_unwind to ensure panics from check_memory_limit do not
        // abort the entire test process.
        let memory_limit = Some(100);
        let host = SimHost::new(None, None, memory_limit);

        let result = panic::catch_unwind(|| {
            host.check_memory_limit();
        });

        // We only assert that a panic is observed here, without relying on
        // the exact panic message format.
        assert!(result.is_err());
    }
}
