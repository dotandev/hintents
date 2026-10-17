// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatFDLimitWarning(t *testing.T) {
	currentLimit := uint64(256)
	minLimit := uint64(4096)

	msg := FormatFDLimitWarning(currentLimit, minLimit)

	require.NotEmpty(t, msg)
	assert.Contains(t, msg, "WARNING:")
	assert.Contains(t, msg, "Current file descriptor limit (256)")
	assert.Contains(t, msg, "recommended minimum (4096)")
	assert.Contains(t, msg, "ulimit -n 4096")
	assert.Contains(t, msg, "socket exhaustion")
}

func TestFormatFDLimitWarning_Thresholds(t *testing.T) {
	testCases := []struct {
		limit    uint64
		minLimit uint64
	}{
		{limit: 128, minLimit: 4096},
		{limit: 256, minLimit: 4096},
		{limit: 1024, minLimit: 4096},
		{limit: 2048, minLimit: 4096},
		{limit: 4095, minLimit: 4096},
	}

	for _, tc := range testCases {
		msg := FormatFDLimitWarning(tc.limit, tc.minLimit)
		assert.Contains(t, msg, "ulimit -n 4096")
		assert.True(t, strings.HasPrefix(msg, "\033[1;33m⚠️  WARNING:\033[0m"))
	}
}

func TestWarnIfLowFDLimit_Behavior(t *testing.T) {
	var buf bytes.Buffer
	warned := WarnIfLowFDLimit(&buf)

	if runtime.GOOS == "windows" {
		assert.False(t, warned)
		assert.Empty(t, buf.String())
	} else {
		// On macOS/Linux, if the test runner has limit < 4096, warning must be emitted;
		// if >= 4096, it remains silent.
		limit, err := getSystemFDLimit()
		require.NoError(t, err)

		if limit < MinRecommendedFDLimit {
			assert.True(t, warned)
			assert.Contains(t, buf.String(), "ulimit -n 4096")
		} else {
			assert.False(t, warned)
			assert.Empty(t, buf.String())
		}
	}
}
