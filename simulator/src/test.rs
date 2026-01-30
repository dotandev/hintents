// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod contract_execution_tests {
    use crate::gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor};
    use crate::{execute_operations, StructuredError};

    // Mock helper to simulate HostError scenarios
    fn simulate_host_error() -> Result<Vec<String>, soroban_env_host::HostError> {
        // This would be a real HostError in actual implementation
        use soroban_env_host::HostError;
        Err(HostError::from(
            soroban_env_host::Error::from_type_and_code(
                soroban_env_host::xdr::ScErrorType::Budget,
                soroban_env_host::xdr::ScErrorCode::ExceededLimit,
            ),
        ))
    }

    #[test]
    fn test_host_error_propagation() {
        let result = simulate_host_error();
        assert!(result.is_err());

        if let Err(e) = result {
            let error_str = format!("{:?}", e);
            assert!(error_str.contains("Budget") || error_str.contains("ExceededLimit"));
        }
    }

    #[test]
    fn test_execute_operations_success_path() {
        use soroban_env_host::xdr::{Operation, VecM};

        // Create empty operations vector
        let operations: VecM<Operation, 100> = VecM::default();
        let host = soroban_env_host::Host::default();

        // Should succeed with empty operations
        let result = execute_operations(&host, &operations);
        assert!(result.is_ok());

        let logs = result.unwrap();
        assert_eq!(logs.len(), 0); // No operations = no logs
    }

    // ============================================================================
    // Panic Scenario Simulations
    // ============================================================================

    /// Test panic during division by zero
    #[test]
    fn test_division_by_zero_panic() {
        let result = std::panic::catch_unwind(|| {
            #[allow(unconditional_panic)]
            let _x = 1 / 0; // This will panic
        });

        assert!(result.is_err(), "Division by zero should panic");

        if let Err(panic_info) = result {
            let message = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown panic".to_string()
            };

            // The panic message should mention division or overflow
            println!("Panic message: {}", message);
            assert!(!message.is_empty());
        }
    }

    /// Test panic from assertion failure
    #[test]
    fn test_assertion_panic() {
        let result = std::panic::catch_unwind(|| {
            let balance = 100;
            let amount = 150;
            assert!(
                balance >= amount,
                "Insufficient balance: {} < {}",
                balance,
                amount
            );
        });

        assert!(result.is_err(), "Failed assertion should panic");

        if let Err(panic_info) = result {
            let message = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown".to_string()
            };

            assert!(message.contains("Insufficient balance") || message.contains("assertion"));
        }
    }

    /// Test panic from explicit panic! macro
    #[test]
    fn test_explicit_panic_macro() {
        let result = std::panic::catch_unwind(|| {
            panic!("Contract execution failed: invalid state");
        });

        assert!(result.is_err());

        if let Err(panic_info) = result {
            let message = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else {
                "Unknown".to_string()
            };

            assert_eq!(message, "Contract execution failed: invalid state");
        }
    }

    // ============================================================================
    // WASM Trap Simulations (these would be HostErrors in real execution)
    // ============================================================================

    #[test]
    fn test_out_of_gas_scenario() {
        // In a real scenario, this would be a HostError from budget exhaustion
        // For now, we simulate the error handling
        use soroban_env_host::HostError;

        let simulated_trap = HostError::from(soroban_env_host::Error::from_type_and_code(
            soroban_env_host::xdr::ScErrorType::Budget,
            soroban_env_host::xdr::ScErrorCode::ExceededLimit,
        ));

        let structured_error = StructuredError {
            error_type: "HostError".to_string(),
            message: format!("{:?}", simulated_trap),
            details: Some("Contract execution failed: out of gas".to_string()),
        };

        assert_eq!(structured_error.error_type, "HostError");
        assert!(structured_error.details.unwrap().contains("out of gas"));
    }

    #[test]
    fn test_invalid_operation_scenario() {
        // Simulate an invalid operation trap
        let structured_error = StructuredError {
            error_type: "HostError".to_string(),
            message: "Invalid operation".to_string(),
            details: Some("Contract attempted to perform an invalid operation".to_string()),
        };

        let json = serde_json::to_string(&structured_error).unwrap();
        assert!(json.contains("HostError"));
        assert!(json.contains("Invalid operation"));
    }

    // ============================================================================
    // State Preservation Tests
    // ============================================================================

    #[test]
    fn test_logs_preserved_before_panic() {
        let mut logs = vec![
            "Host initialized".to_string(),
            "Loaded 5 ledger entries".to_string(),
        ];

        // Create a closure that adds logs then panics
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            let mut inner_logs = logs.clone();
            inner_logs.push("Started contract execution".to_string());
            inner_logs.push("Function call: transfer".to_string());
            panic!("Contract panicked during transfer");
            #[allow(unreachable_code)]
            inner_logs
        }));

        // The panic should be caught
        assert!(result.is_err());

        // In the real simulator, logs collected before the panic boundary are preserved
        // Even though inner_logs are lost in this test, the outer logs remain
        assert_eq!(logs.len(), 2);

        // After catching the panic, we would add the panic message to logs
        logs.push("PANIC: Contract panicked during transfer".to_string());
        assert_eq!(logs.len(), 3);
    }

    #[test]
    fn test_partial_execution_state_captured() {
        // Simulate a scenario where some operations succeed before one panics
        let mut execution_logs: Vec<String> = Vec::new();

        for i in 0..5 {
            let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                if i == 3 {
                    panic!("Operation {} failed", i);
                }
                format!("Operation {} succeeded", i)
            }));

            match result {
                Ok(log) => execution_logs.push(log),
                Err(_) => {
                    execution_logs.push(format!("Operation {} panicked", i));
                    break; // Stop processing further operations
                }
            }
        }

        // Should have logs for operations 0, 1, 2, and the panic at 3
        assert_eq!(execution_logs.len(), 4);
        assert!(execution_logs[3].contains("panicked"));
    }

    // ============================================================================
    // Error Message Quality Tests
    // ============================================================================

    #[test]
    fn test_error_message_contains_useful_info() {
        let result = std::panic::catch_unwind(|| {
            panic!("Transfer failed: insufficient balance (have: 100, need: 150)");
        });

        if let Err(panic_info) = result {
            let message = panic_info
                .downcast_ref::<&str>()
                .map(|s| s.to_string())
                .unwrap_or_else(|| "Unknown".to_string());

            // Error message should contain actionable information
            assert!(message.contains("insufficient balance"));
            assert!(message.contains("100"));
            assert!(message.contains("150"));
        }
    }

    #[test]
    fn test_structured_error_provides_context() {
        let error = StructuredError {
            error_type: "Panic".to_string(),
            message: "Index out of bounds".to_string(),
            details: Some(
                "Attempted to access index 10 in array of length 5. \
                 This occurred in function 'get_user_data' at contract address 0x1234..."
                    .to_string(),
            ),
        };

        let json = serde_json::to_string(&error).unwrap();
        let parsed: StructuredError = serde_json::from_str(&json).unwrap();

        // Verify context is preserved
        assert!(parsed.details.is_some());
        let details = parsed.details.unwrap();
        assert!(details.contains("index 10"));
        assert!(details.contains("length 5"));
        assert!(details.contains("get_user_data"));
    }

    // ============================================================================
    // Recovery Tests
    // ============================================================================

    #[test]
    fn test_simulator_can_handle_subsequent_requests_after_panic() {
        // Simulate multiple requests, some panicking, some succeeding
        let requests = vec![
            ("request_1", false), // succeeds
            ("request_2", true),  // panics
            ("request_3", false), // succeeds
            ("request_4", true),  // panics
            ("request_5", false), // succeeds
        ];

        let mut results = Vec::new();

        for (name, should_panic) in requests {
            let result = std::panic::catch_unwind(|| {
                if should_panic {
                    panic!("Request {} panicked", name);
                }
                format!("Request {} succeeded", name)
            });

            match result {
                Ok(msg) => results.push(("success", msg)),
                Err(_) => results.push(("error", format!("Request {} panicked", name))),
            }
        }

        // All requests should be handled
        assert_eq!(results.len(), 5);

        // Verify success/error pattern
        assert_eq!(results[0].0, "success");
        assert_eq!(results[1].0, "error");
        assert_eq!(results[2].0, "success");
        assert_eq!(results[3].0, "error");
        assert_eq!(results[4].0, "success");
    }

    // ============================================================================
    // Performance Tests
    // ============================================================================

    #[test]
    fn test_panic_handling_overhead() {
        use std::time::Instant;

        // Measure overhead of catch_unwind on success path
        let iterations = 10000;

        // Without catch_unwind
        let start = Instant::now();
        for _ in 0..iterations {
            let _result: Result<(), ()> = Ok(());
        }
        let without_catch = start.elapsed();

        // With catch_unwind
        let start = Instant::now();
        for _ in 0..iterations {
            let _result = std::panic::catch_unwind(|| {
                // Empty operation
            });
        }
        let with_catch = start.elapsed();

        println!("Without catch_unwind: {:?}", without_catch);
        println!("With catch_unwind: {:?}", with_catch);

        // Overhead should be minimal (typically < 5% on modern systems)
        // This is informational, not a strict assertion
        let overhead_ratio = with_catch.as_nanos() as f64 / without_catch.as_nanos() as f64;
        println!("Overhead ratio: {:.2}x", overhead_ratio);
    }

    // ============================================================================
    // Test Gas Optimizer
    // ============================================================================

    #[test]
    fn test_efficient_contract_analysis() {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: 5000,
            memory_bytes: 2500,
            total_operations: 5,
        };

        let report = advisor.analyze(&metrics);

        // Should have high efficiency
        assert!(report.overall_efficiency >= 90.0);

        // Should have minimal warnings
        assert!(report.tips.iter().any(|t| t.severity == "low"));

        // Should have positive comparison
        assert!(report.comparison_to_baseline.contains("Excellent"));

        println!("Efficient Contract Report:");
        println!("Efficiency: {:.1}%", report.overall_efficiency);
        println!("Comparison: {}", report.comparison_to_baseline);
        for tip in &report.tips {
            println!("  - [{}] {}: {}", tip.severity, tip.category, tip.message);
        }
    }

    #[test]
    fn test_inefficient_contract_with_high_cpu() {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: 50_000_000, // 50M CPU (50% of typical budget)
            memory_bytes: 5_000_000,      // 5M Memory
            total_operations: 10,
        };

        let report = advisor.analyze(&metrics);

        assert!(report.overall_efficiency < 70.0);

        assert!(report.tips.iter().any(|t| t.severity == "high"));

        assert!(report
            .tips
            .iter()
            .any(|t| t.category.contains("CPU") || t.category.contains("Budget")));

        assert!(report
            .tips
            .iter()
            .any(|t| t.message.contains("50") && t.message.contains("%")));

        println!("
Inefficient Contract Report:");
        println!("Efficiency: {:.1}%", report.overall_efficiency);
        println!("Comparison: {}", report.comparison_to_baseline);
        for tip in &report.tips {
            println!("  - [{}] {}: {}", tip.severity, tip.category, tip.message);
            println!("    Savings: {}", tip.estimated_savings);
        }
    }

    #[test]
    fn test_high_memory_usage() {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: 10_000_000,
            memory_bytes: 20_000_000, // 20M Memory (40% of typical budget)
            total_operations: 5,
        };

        let report = advisor.analyze(&metrics);

        // Should have memory-related warnings
        assert!(report.tips.iter().any(|t| t.category.contains("Memory")));

        // Should warn about high memory percentage
        assert!(report
            .tips
            .iter()
            .any(|t| t.message.contains("Memory usage") && t.message.contains("%")));

        println!("
High Memory Usage Report:");
        for tip in &report.tips {
            println!("  - [{}] {}: {}", tip.severity, tip.category, tip.message);
        }
    }

    #[test]
    fn test_loop_optimization_detection() {
        let advisor = GasOptimizationAdvisor::new();

        // Test loop with many iterations
        let tip = advisor.analyze_operation_pattern("loop", 150, 100_000);
        assert!(tip.is_some());

        let tip = tip.unwrap();
        assert_eq!(tip.category, "Loop Optimization");
        assert_eq!(tip.severity, "high");
        assert!(tip.message.contains("150 times"));
        assert!(tip.estimated_savings.contains("30-50%"));

        println!("
Loop Optimization Tip:");
        println!("  {}", tip.message);
        println!("  Estimated Savings: {}", tip.estimated_savings);
    }

    #[test]
    fn test_storage_read_optimization() {
        let advisor = GasOptimizationAdvisor::new();

        // Test excessive storage reads
        let tip = advisor.analyze_operation_pattern("storage_read", 60, 75_000);
        assert!(tip.is_some());

        let tip = tip.unwrap();
        assert_eq!(tip.category, "Storage Access");
        assert_eq!(tip.severity, "medium");
        assert!(tip.message.contains("60 storage reads"));
        assert!(tip.message.contains("Cache"));

        println!("
Storage Read Optimization Tip:");
        println!("  {}", tip.message);
    }

    #[test]
    fn test_storage_write_optimization() {
        let advisor = GasOptimizationAdvisor::new();

        // Test excessive storage writes
        let tip = advisor.analyze_operation_pattern("storage_write", 25, 50_000);
        assert!(tip.is_some());

        let tip = tip.unwrap();
        assert_eq!(tip.category, "Storage Access");
        assert_eq!(tip.severity, "high");
        assert!(tip.message.contains("25 storage writes"));
        assert!(tip.message.contains("Batch"));

        println!("
Storage Write Optimization Tip:");
        println!("  {}", tip.message);
    }

    #[test]
    fn test_budget_breakdown() {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: 45_000_000,
            memory_bytes: 18_000_000,
            total_operations: 10,
        };

        let report = advisor.analyze(&metrics);

        // Check budget breakdown contains expected metrics
        assert!(report.budget_breakdown.contains_key("cpu_usage_percent"));
        assert!(report.budget_breakdown.contains_key("memory_usage_percent"));
        assert!(report.budget_breakdown.contains_key("cpu_per_operation"));
        assert!(report.budget_breakdown.contains_key("memory_per_operation"));

        // CPU should be ~45% of 100M budget
        let cpu_pct = report.budget_breakdown.get("cpu_usage_percent").unwrap();
        assert!(*cpu_pct > 40.0 && *cpu_pct < 50.0);

        // Memory should be ~36% of 50M budget
        let mem_pct = report.budget_breakdown.get("memory_usage_percent").unwrap();
        assert!(*mem_pct > 30.0 && *mem_pct < 40.0);

        println!("
Budget Breakdown:");
        for (key, value) in &report.budget_breakdown {
            println!("  {}: {:.2}", key, value);
        }
    }

    #[test]
    fn test_no_optimization_needed() {
        let advisor = GasOptimizationAdvisor::new();

        // Test operations that don't need optimization
        let tip1 = advisor.analyze_operation_pattern("loop", 50, 10_000);
        assert!(tip1.is_none());

        let tip2 = advisor.analyze_operation_pattern("storage_read", 30, 20_000);
        assert!(tip2.is_none());

        let tip3 = advisor.analyze_operation_pattern("storage_write", 10, 15_000);
        assert!(tip3.is_none());

        println!("
No optimization tips needed for efficient operations");
    }

    #[test]
    fn test_comprehensive_unoptimized_scenario() {
        let advisor = GasOptimizationAdvisor::new();

        // Simulate a really unoptimized contract
        let metrics = BudgetMetrics {
            cpu_instructions: 80_000_000, // 80% of budget
            memory_bytes: 40_000_000,     // 80% of budget
            total_operations: 20,
        };

        let report = advisor.analyze(&metrics);

        // Should have very low efficiency
        assert!(report.overall_efficiency < 50.0);

        // Should have multiple high severity tips
        let high_severity_count = report.tips.iter().filter(|t| t.severity == "high").count();
        assert!(high_severity_count >= 2);

        // Should recommend poor status
        assert!(report.comparison_to_baseline.contains("Poor"));

        println!("
Comprehensive Unoptimized Contract Report:");
        println!("Efficiency Score: {:.1}%", report.overall_efficiency);
        println!("Status: {}", report.comparison_to_baseline);
        println!("
Optimization Tips:");
        for (i, tip) in report.tips.iter().enumerate() {
            println!(
                "
{}. [{}] {}",
                i + 1,
                tip.severity.to_uppercase(),
                tip.category
            );
            println!("   {}", tip.message);
            println!("   Potential Savings: {}", tip.estimated_savings);
            if let Some(location) = &tip.code_location {
                println!("   Location: {}", location);
            }
        }
    }
}
