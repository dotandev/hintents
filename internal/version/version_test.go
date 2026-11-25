// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package version

import "testing"

func TestDevelopmentDefaults(t *testing.T) {
	if Version != "0.0.0-dev" {
		t.Errorf("Version = %q, want %q", Version, "0.0.0-dev")
	}
	if CommitSHA != "unknown" {
		t.Errorf("CommitSHA = %q, want %q", CommitSHA, "unknown")
	}
	if BuildDate != "unknown" {
		t.Errorf("BuildDate = %q, want %q", BuildDate, "unknown")
	}
}
