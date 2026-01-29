// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package security_test

import (
	"fmt"

	"github.com/dotandev/hintents/internal/security"
)

// Example demonstrates basic usage of the security detector
func Example() {
	detector := security.NewDetector()

	// Simulate a transaction with security issues
	events := []string{
		"ContractEvent: transfer initiated",
		"PANIC: arithmetic overflow",
	}

	logs := []string{
		"Executing transfer function",
		"overflow detected in balance calculation",
	}

	findings := detector.Analyze("", "", events, logs)

	fmt.Printf("Found %d security issues:\n", len(findings))
	for i, finding := range findings {
		fmt.Printf("%d. [%s] %s - %s\n", i+1, finding.Type, finding.Severity, finding.Title)
	}

	// Output:
	// Found 2 security issues:
	// 1. [VERIFIED_RISK] HIGH - Integer Overflow/Underflow Detected
	// 2. [VERIFIED_RISK] HIGH - Contract Panic/Trap
}
