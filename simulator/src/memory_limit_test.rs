// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Test module for memory limit simulation functionality

#[cfg(test)]
mod tests {
    use crate::runner::SimHost;
    use crate::types::ResourceCalibration;

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
    fn test_memory_limit_check() {
        // Test memory limit checking functionality
        let memory_limit = Some(1000); // Very small limit
        let host = SimHost::new(None, None, memory_limit);
        
        // This should not panic as we haven't executed any operations yet
        host.check_memory_limit();
    }

    #[test]
    #[should_panic(expected = "Memory limit exceeded")]
    fn test_memory_limit_exceeded() {
        // This test would require mocking the host to return high memory usage
        // For now, we just verify the panic message format
        let memory_limit = Some(100);
        let host = SimHost::new(None, None, memory_limit);
        
        // This will panic if memory usage exceeds limit
        // Note: In a real test, we'd need to mock the budget to return high usage
        host.check_memory_limit();
    }
}
