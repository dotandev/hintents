// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostFunctionHoverContent(t *testing.T) {
	content := HostFunctionHoverContent("require_auth")
	assert.Contains(t, content, "**require_auth**")
	assert.Contains(t, content, "ensures the given account")

	unknown := HostFunctionHoverContent("unknown_fn")
	assert.Contains(t, unknown, "**unknown_fn**")
	assert.Contains(t, unknown, "host function")
}
