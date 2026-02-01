// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import "testing"

// TestCIFailureDemo demonstrates CI failure detection
// Uncomment the t.Fail() line to test CI failure
func TestCIFailureDemo(t *testing.T) {
	// t.Fail() // Uncomment this line to intentionally break CI
	t.Log("CI test passing")
}
