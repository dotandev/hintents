// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTraceTree_UsesCrossContractCallsAsNestedNodes(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	trace.AddState(ExecutionState{Operation: "call", ContractID: "A", Function: "deposit"})
	trace.AddState(ExecutionState{Operation: "call", ContractID: "B", Function: "swap"})
	trace.AddState(ExecutionState{Operation: "call", ContractID: "C", Function: "approve"})

	root := BuildTraceTree(trace)
	require.NotNil(t, root)
	require.Len(t, root.Children, 1)
	require.Equal(t, "A", root.Children[0].ContractID)
	require.Len(t, root.Children[0].Children, 1)
	require.Equal(t, "B", root.Children[0].Children[0].ContractID)
	require.Len(t, root.Children[0].Children[0].Children, 1)
	require.Equal(t, "C", root.Children[0].Children[0].Children[0].ContractID)
}
