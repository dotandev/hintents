// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import "testing"

func TestRenderEventTree_Basic(t *testing.T) {
	events := []Event{
		{Type: "INIT_TRANSFER"},
		{
			Type: "DEBIT_ACCOUNT",
			Children: []Event{
				{Type: "FEE_CALC"},
			},
		},
		{Type: "COMPLETE"},
	}

	output := RenderEventTree(events)

	expected := `Events
├── INIT_TRANSFER
├── DEBIT_ACCOUNT
│   └── FEE_CALC
└── COMPLETE`

	if output != expected {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestRenderEventTree_Empty(t *testing.T) {
	output := RenderEventTree(nil)

	if output != "No events recorded." {
		t.Fatalf("unexpected output: %s", output)
	}
}
