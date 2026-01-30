// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
