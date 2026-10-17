// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errReader struct{}

func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read failure")
}

func TestResolveContractArgs_FlagPassthrough(t *testing.T) {
	raw := []string{"foo", "bar", "--flag=val"}
	res, err := ResolveContractArgs(raw, strings.NewReader("unexpected stdin"))
	require.NoError(t, err)
	assert.Equal(t, raw, res)
}

func TestResolveContractArgs_EmptyStdin(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n\t  \r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ResolveContractArgs([]string{"-"}, strings.NewReader(tt.input))
			require.NoError(t, err)
			assert.Empty(t, res)
		})
	}
}

func TestResolveContractArgs_JSONArrayStrings(t *testing.T) {
	jsonInput := `["alpha", "beta", "gamma 123"]`
	res, err := ResolveContractArgs([]string{"-"}, strings.NewReader(jsonInput))
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma 123"}, res)
}

func TestResolveContractArgs_JSONArrayGeneric(t *testing.T) {
	jsonInput := `["normal_str", 100, true, {"key": "val"}]`
	res, err := ResolveContractArgs([]string{"-"}, strings.NewReader(jsonInput))
	require.NoError(t, err)
	assert.Len(t, res, 4)
	assert.Equal(t, "normal_str", res[0])
	assert.Equal(t, "100", res[1])
	assert.Equal(t, "true", res[2])
	assert.JSONEq(t, `{"key": "val"}`, res[3])
}

func TestResolveContractArgs_NewlineSeparated(t *testing.T) {
	input := "arg1\narg2\n  arg3 with spaces  \n\narg4"
	res, err := ResolveContractArgs([]string{"-"}, strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []string{"arg1", "arg2", "arg3 with spaces", "arg4"}, res)
}

func TestResolveContractArgs_SingleRawText(t *testing.T) {
	input := "single_argument_value"
	res, err := ResolveContractArgs([]string{"-"}, strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []string{"single_argument_value"}, res)
}

func TestResolveContractArgs_ReaderError(t *testing.T) {
	_, err := ResolveContractArgs([]string{"-"}, &errReader{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read arguments from stdin")
}

func TestInvokeCmd_FlagsRegistration(t *testing.T) {
	cmd := invokeCmd
	assert.NotNil(t, cmd)
	assert.Equal(t, "invoke <contract-path-or-url>", cmd.Use)
	assert.Equal(t, "testing", cmd.GroupID)

	argsFlag := cmd.Flags().Lookup("args")
	require.NotNil(t, argsFlag)
	assert.Equal(t, "stringSlice", argsFlag.Value.Type())

	noCacheFlag := cmd.Flags().Lookup("no-cache")
	require.NotNil(t, noCacheFlag)
	assert.Equal(t, "bool", noCacheFlag.Value.Type())
}

func TestRunCmd_PipedArgsIntegration(t *testing.T) {
	// Verify that runCmd has the --args flag configured identically
	argsFlag := runCmd.Flags().Lookup("args")
	require.NotNil(t, argsFlag)
	assert.Equal(t, "stringSlice", argsFlag.Value.Type())
}
