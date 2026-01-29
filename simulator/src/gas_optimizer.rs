// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct BudgetMetrics {
    pub cpu_instructions: u64,
    pub memory_bytes: u64,
    pub total_operations: usize,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct OptimizationTip {
    pub category: String,
    pub severity: String, // "high", "medium", "low"
    pub message: String,
    pub estimated_savings: String,
    pub code_location: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OptimizationReport {
    pub overall_efficiency: f64, // 0-100 score
    pub tips: Vec<OptimizationTip>,
    pub budget_breakdown: HashMap<String, f64>,
    pub comparison_to_baseline: String,
}

pub struct GasOptimizationAdvisor {
    // Baseline metrics for common operations
    baseline_cpu_per_op: u64,
    baseline_memory_per_op: u64,
}

impl GasOptimizationAdvisor {
    pub fn new() -> Self {
        Self {
            baseline_cpu_per_op: 1000,
            baseline_memory_per_op: 500,
        }
    }

    /// Analyze budget metrics and generate optimization suggestions
    pub fn analyze(&self, metrics: &BudgetMetrics) -> OptimizationReport {
        let mut tips = Vec::new();
        let mut budget_breakdown = HashMap::new();

        let cpu_per_op = if metrics.total_operations > 0 {
            metrics.cpu_instructions / metrics.total_operations as u64
        } else {
            0
        };

        let memory_per_op = if metrics.total_operations > 0 {
            metrics.memory_bytes / metrics.total_operations as u64
        } else {
            0
        };

        let cpu_percentage = (metrics.cpu_instructions as f64 / 100_000_000.0) * 100.0;
        let memory_percentage = (metrics.memory_bytes as f64 / 50_000_000.0) * 100.0;

        budget_breakdown.insert("cpu_usage_percent".to_string(), cpu_percentage);
        budget_breakdown.insert("memory_usage_percent".to_string(), memory_percentage);
        budget_breakdown.insert("cpu_per_operation".to_string(), cpu_per_op as f64);
        budget_breakdown.insert("memory_per_operation".to_string(), memory_per_op as f64);

        // Analyze CPU usage
        if cpu_per_op > self.baseline_cpu_per_op * 2 {
            tips.push(OptimizationTip {
                category: "CPU Usage".to_string(),
                severity: "high".to_string(),
                message: format!(
                    "CPU consumption is {}x higher than baseline. Consider optimizing loops and reducing computational complexity.",
                    cpu_per_op / self.baseline_cpu_per_op
                ),
                estimated_savings: format!("~{}% reduction possible", 
                    ((cpu_per_op - self.baseline_cpu_per_op) as f64 / cpu_per_op as f64 * 100.0) as u32),
                code_location: Some("Loop operations".to_string()),
            });
        } else if cpu_per_op > self.baseline_cpu_per_op {
            tips.push(OptimizationTip {
                category: "CPU Usage".to_string(),
                severity: "medium".to_string(),
                message: format!(
                    "CPU usage is {}x baseline. Review computational operations for optimization opportunities.",
                    cpu_per_op / self.baseline_cpu_per_op
                ),
                estimated_savings: format!("~{}% reduction possible",
                    ((cpu_per_op - self.baseline_cpu_per_op) as f64 / cpu_per_op as f64 * 100.0) as u32),
                code_location: None,
            });
        }

        // Analyze Memory usage
        if memory_per_op > self.baseline_memory_per_op * 2 {
            tips.push(OptimizationTip {
                category: "Memory Usage".to_string(),
                severity: "high".to_string(),
                message: format!(
                    "Memory consumption is {}x higher than baseline. Consider using more efficient data structures or reducing allocations.",
                    memory_per_op / self.baseline_memory_per_op
                ),
                estimated_savings: format!("~{}% reduction possible",
                    ((memory_per_op - self.baseline_memory_per_op) as f64 / memory_per_op as f64 * 100.0) as u32),
                code_location: Some("Data storage operations".to_string()),
            });
        } else if memory_per_op > self.baseline_memory_per_op {
            tips.push(OptimizationTip {
                category: "Memory Usage".to_string(),
                severity: "medium".to_string(),
                message: "Memory usage is above baseline. Review data structure choices."
                    .to_string(),
                estimated_savings: format!(
                    "~{}% reduction possible",
                    ((memory_per_op - self.baseline_memory_per_op) as f64 / memory_per_op as f64
                        * 100.0) as u32
                ),
                code_location: None,
            });
        }

        // High CPU percentage warning
        if cpu_percentage > 40.0 {
            tips.push(OptimizationTip {
                category: "Budget Allocation".to_string(),
                severity: "high".to_string(),
                message: format!(
                    "This operation consumes {:.1}% of the CPU budget; consider batching multiple operations or caching results.",
                    cpu_percentage
                ),
                estimated_savings: "20-40% with batching".to_string(),
                code_location: Some("Contract invocation".to_string()),
            });
        }

        // Memory optimization tips
        if memory_percentage > 30.0 {
            tips.push(OptimizationTip {
                category: "Memory Efficiency".to_string(),
                severity: "medium".to_string(),
                message: format!(
                    "Memory usage is {:.1}% of budget. Consider using references instead of cloning data.",
                    memory_percentage
                ),
                estimated_savings: "10-25% with better memory management".to_string(),
                code_location: None,
            });
        }

        // General best practices
        if tips.is_empty() {
            tips.push(OptimizationTip {
                category: "General".to_string(),
                severity: "low".to_string(),
                message: "Contract execution is efficient. Consider testing with larger datasets to ensure scalability.".to_string(),
                estimated_savings: "N/A".to_string(),
                code_location: None,
            });
        }

        // Calculate overall efficiency score (0-100)
        let cpu_efficiency = if cpu_per_op > 0 {
            (self.baseline_cpu_per_op as f64 / cpu_per_op as f64 * 100.0).min(100.0)
        } else {
            100.0
        };

        let memory_efficiency = if memory_per_op > 0 {
            (self.baseline_memory_per_op as f64 / memory_per_op as f64 * 100.0).min(100.0)
        } else {
            100.0
        };

        let overall_efficiency = (cpu_efficiency + memory_efficiency) / 2.0;

        // Comparison summary
        let comparison = if overall_efficiency >= 90.0 {
            "Excellent - performing within best practice guidelines".to_string()
        } else if overall_efficiency >= 70.0 {
            "Good - minor optimizations possible".to_string()
        } else if overall_efficiency >= 50.0 {
            "Fair - significant optimization opportunities exist".to_string()
        } else {
            "Poor - contract requires substantial optimization".to_string()
        };

        OptimizationReport {
            overall_efficiency,
            tips,
            budget_breakdown,
            comparison_to_baseline: comparison,
        }
    }

    /// Analyze specific operation patterns
    #[allow(dead_code)]
    pub fn analyze_operation_pattern(
        &self,
        operation_type: &str,
        count: usize,
        cpu_cost: u64,
    ) -> Option<OptimizationTip> {
        match operation_type {
            "loop" if count > 100 => Some(OptimizationTip {
                category: "Loop Optimization".to_string(),
                severity: "high".to_string(),
                message: format!(
                    "Loop executes {} times consuming {} CPU instructions. Consider batching or reducing iterations.",
                    count, cpu_cost
                ),
                estimated_savings: "30-50% with batching".to_string(),
                code_location: Some("Loop body".to_string()),
            }),
            "storage_read" if count > 50 => Some(OptimizationTip {
                category: "Storage Access".to_string(),
                severity: "medium".to_string(),
                message: format!(
                    "{} storage reads detected. Cache frequently accessed values.",
                    count
                ),
                estimated_savings: "15-30% with caching".to_string(),
                code_location: Some("Storage operations".to_string()),
            }),
            "storage_write" if count > 20 => Some(OptimizationTip {
                category: "Storage Access".to_string(),
                severity: "high".to_string(),
                message: format!(
                    "{} storage writes detected. Batch writes or use temporary variables.",
                    count
                ),
                estimated_savings: "25-40% with batching".to_string(),
                code_location: Some("Storage operations".to_string()),
            }),
            _ => None,
        }
    }
}

impl Default for GasOptimizationAdvisor {
    fn default() -> Self {
        Self::new()
    }
}
