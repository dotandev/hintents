// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"fmt"
	"io"

	"github.com/dotandev/hintents/internal/trace"
	"github.com/google/pprof/profile"
)

const (
	// SampleTypeGas is the pprof sample type for gas consumption.
	SampleTypeGas = "gas"
	// SampleUnitCount is the unit for gas samples.
	SampleUnitCount = "count"
)

// TraceToPprof synthesizes an execution trace into a pprof-compliant profile
// that maps gas consumption to functions. The result can be viewed with
// go tool pprof.
func TraceToPprof(execTrace *trace.ExecutionTrace) (*profile.Profile, error) {
	if execTrace == nil {
		return nil, fmt.Errorf("execution trace is nil")
	}

	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: SampleTypeGas, Unit: SampleUnitCount},
		},
		DefaultSampleType: SampleTypeGas,
		Mapping: []*profile.Mapping{
			{ID: 1, Start: 0, Limit: 0, File: "soroban", HasFunctions: true},
		},
		Function: make([]*profile.Function, 0),
		Location: make([]*profile.Location, 0),
		Sample:   make([]*profile.Sample, 0),
	}

	funcByKey := make(map[string]*profile.Function)
	locByKey := make(map[string]*profile.Location)
	mapping := p.Mapping[0]
	var funcID, locID uint64

	nextFuncID := func() uint64 {
		funcID++
		return funcID
	}
	nextLocID := func() uint64 {
		locID++
		return locID
	}

	for i := range execTrace.States {
		state := &execTrace.States[i]
		gas := extractGasFromState(state)
		if gas < 0 {
			gas = 0
		}

		name := functionName(state)
		if name == "" {
			name = state.Operation
		}
		if name == "" {
			name = fmt.Sprintf("step_%d", state.Step)
		}

		key := name
		loc, ok := locByKey[key]
		if !ok {
			fn, ok := funcByKey[key]
			if !ok {
				fn = &profile.Function{
					ID:   nextFuncID(),
					Name: name,
				}
				p.Function = append(p.Function, fn)
				funcByKey[key] = fn
			}
			loc = &profile.Location{
				ID:       nextLocID(),
				Mapping:  mapping,
				Address:  uint64(state.Step),
				Line:     []profile.Line{{Function: fn, Line: int64(state.Step)}},
			}
			p.Location = append(p.Location, loc)
			locByKey[key] = loc
		}

		if gas > 0 {
			p.Sample = append(p.Sample, &profile.Sample{
				Location: []*profile.Location{loc},
				Value:    []int64{gas},
			})
		}
	}

	if err := p.CheckValid(); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}
	return p, nil
}

func functionName(state *trace.ExecutionState) string {
	if state.ContractID != "" && state.Function != "" {
		return state.ContractID + "::" + state.Function
	}
	if state.Function != "" {
		return state.Function
	}
	return ""
}

func extractGasFromState(state *trace.ExecutionState) int64 {
	if state.HostState == nil {
		return 0
	}
	v, ok := state.HostState["gas_used"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case uint64:
		return int64(n)
	default:
		return 0
	}
}

// WritePprof writes the trace as a pprof profile to w (gzip-compressed protobuf).
func WritePprof(execTrace *trace.ExecutionTrace, w io.Writer) error {
	p, err := TraceToPprof(execTrace)
	if err != nil {
		return err
	}
	return p.Write(w)
}
