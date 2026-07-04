// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dotandev/hintents/internal/logger"
)

// MultiProtocolResult represents the simulation result for a single protocol version
type MultiProtocolResult struct {
	ProtocolVersion uint32
	Response        *SimulationResponse
	Error           error
}

// MultiProtocolComparison contains comparative analysis across protocol versions
type MultiProtocolComparison struct {
	Results        []*MultiProtocolResult
	GasCosts       map[uint32]uint64
	SuccessByProto map[uint32]bool
	ErrorByProto   map[uint32]string
	FeatureChanges []FeatureChange
	GasImpact      GasImpact
}

// FeatureChange describes a behavioral difference between protocol versions
type FeatureChange struct {
	FeatureName       string
	OldValue          interface{}
	NewValue          interface{}
	AffectedProtocols []uint32
}

// GasImpact summarizes gas cost differences across protocol versions
type GasImpact struct {
	MinCost     uint64
	MaxCost     uint64
	MinProtocol uint32
	MaxProtocol uint32
	Variance    float64
}

// RunMultiProtocol simulates the same transaction against multiple protocol versions
// simultaneously, allowing developers to predict how protocol upgrades will impact
// contract behavior and gas costs.
func (r *Runner) RunMultiProtocol(ctx context.Context, req *SimulationRequest, protocolVersions []uint32) (*MultiProtocolComparison, error) {
	if len(protocolVersions) == 0 {
		// Default to all supported protocols if none specified
		protocolVersions = Supported()
	}

	// Validate all requested protocol versions
	for _, v := range protocolVersions {
		if err := Validate(v); err != nil {
			return nil, fmt.Errorf("invalid protocol version %d: %w", v, err)
		}
	}

	logger.Logger.Info("Starting multi-protocol simulation",
		"transaction", req.EnvelopeXdr[:min(50, len(req.EnvelopeXdr))],
		"protocol_versions", protocolVersions,
		"count", len(protocolVersions),
	)

	// Run simulations in parallel
	results := make([]*MultiProtocolResult, len(protocolVersions))
	var wg sync.WaitGroup
	errCh := make(chan error, len(protocolVersions))

	for i, protoVersion := range protocolVersions {
		wg.Add(1)
		go func(idx int, version uint32) {
			defer wg.Done()

			// Clone the request to avoid race conditions
			reqCopy := cloneRequest(req)
			reqCopy.ProtocolVersion = &version

			resp, err := r.Run(ctx, reqCopy)
			results[idx] = &MultiProtocolResult{
				ProtocolVersion: version,
				Response:        resp,
				Error:           err,
			}

			if err != nil {
				logger.Logger.Warn("Multi-protocol simulation failed",
					"protocol", version,
					"error", err,
				)
			}
		}(i, protoVersion)
	}

	wg.Wait()
	close(errCh)

	// Check for critical errors (context cancellation, etc.)
	select {
	case err := <-errCh:
		if err == ctx.Err() {
			return nil, ctx.Err()
		}
	default:
	}

	// Build comparison
	comparison := &MultiProtocolComparison{
		Results:        results,
		GasCosts:       make(map[uint32]uint64),
		SuccessByProto: make(map[uint32]bool),
		ErrorByProto:   make(map[uint32]string),
	}

	for _, result := range results {
		if result.Error != nil {
			comparison.ErrorByProto[result.ProtocolVersion] = result.Error.Error()
			comparison.SuccessByProto[result.ProtocolVersion] = false
			continue
		}

		if result.Response != nil {
			comparison.SuccessByProto[result.ProtocolVersion] = true
			if result.Response.BudgetUsage != nil {
				// Use CPU instructions as a proxy for gas cost
				comparison.GasCosts[result.ProtocolVersion] = result.Response.BudgetUsage.CPUInstructions
			}
		}
	}

	// Analyze gas impact
	comparison.GasImpact = analyzeGasImpact(comparison.GasCosts)

	// Detect feature changes
	comparison.FeatureChanges = detectFeatureChanges(protocolVersions)

	logger.Logger.Info("Multi-protocol simulation complete",
		"successful", len(comparison.SuccessByProto),
		"failed", len(comparison.ErrorByProto),
		"gas_variance", comparison.GasImpact.Variance,
	)

	return comparison, nil
}

// cloneRequest creates a deep copy of a SimulationRequest
func cloneRequest(req *SimulationRequest) *SimulationRequest {
	reqCopy := *req

	// Deep copy maps
	if req.LedgerEntries != nil {
		reqCopy.LedgerEntries = make(map[string]string, len(req.LedgerEntries))
		for k, v := range req.LedgerEntries {
			reqCopy.LedgerEntries[k] = v
		}
	}

	if req.ForkParams != nil {
		reqCopy.ForkParams = make(map[string]string, len(req.ForkParams))
		for k, v := range req.ForkParams {
			reqCopy.ForkParams[k] = v
		}
	}

	if req.CustomAuthCfg != nil {
		reqCopy.CustomAuthCfg = make(map[string]interface{}, len(req.CustomAuthCfg))
		for k, v := range req.CustomAuthCfg {
			reqCopy.CustomAuthCfg[k] = v
		}
	}

	if req.RestorePreamble != nil {
		reqCopy.RestorePreamble = make(map[string]interface{}, len(req.RestorePreamble))
		for k, v := range req.RestorePreamble {
			reqCopy.RestorePreamble[k] = v
		}
	}

	// Copy pointers
	if req.WasmPath != nil {
		wasmPath := *req.WasmPath
		reqCopy.WasmPath = &wasmPath
	}

	if req.MockArgs != nil {
		mockArgs := make([]string, len(*req.MockArgs))
		copy(mockArgs, *req.MockArgs)
		reqCopy.MockArgs = &mockArgs
	}

	if req.MockBaseFee != nil {
		baseFee := *req.MockBaseFee
		reqCopy.MockBaseFee = &baseFee
	}

	if req.MockGasPrice != nil {
		gasPrice := *req.MockGasPrice
		reqCopy.MockGasPrice = &gasPrice
	}

	if req.MemoryLimit != nil {
		memLimit := *req.MemoryLimit
		reqCopy.MemoryLimit = &memLimit
	}

	if req.CoverageLCOVPath != nil {
		covPath := *req.CoverageLCOVPath
		reqCopy.CoverageLCOVPath = &covPath
	}

	if req.AuthTraceOpts != nil {
		authOpts := *req.AuthTraceOpts
		reqCopy.AuthTraceOpts = &authOpts
	}

	if req.ResourceCalibration != nil {
		calib := *req.ResourceCalibration
		reqCopy.ResourceCalibration = &calib
	}

	if req.SandboxNativeTokenCapStroops != nil {
		cap := *req.SandboxNativeTokenCapStroops
		reqCopy.SandboxNativeTokenCapStroops = &cap
	}

	if req.ContractWasm != nil {
		wasm := *req.ContractWasm
		reqCopy.ContractWasm = &wasm
	}

	return &reqCopy
}

// estimateGasCost extracts gas cost from simulation response
// This is a placeholder; actual implementation should parse XDR
func estimateGasCost(resp *SimulationResponse) uint64 {
	// Use CPU instructions as a proxy for gas cost
	if resp.BudgetUsage != nil {
		return resp.BudgetUsage.CPUInstructions
	}
	return 0
}

// analyzeGasImpact calculates gas cost variance across protocol versions
func analyzeGasImpact(gasCosts map[uint32]uint64) GasImpact {
	if len(gasCosts) == 0 {
		return GasImpact{}
	}

	var minCost, maxCost uint64
	var minProto, maxProto uint32
	first := true

	for proto, cost := range gasCosts {
		if first || cost < minCost {
			minCost = cost
			minProto = proto
		}
		if first || cost > maxCost {
			maxCost = cost
			maxProto = proto
		}
		first = false
	}

	variance := 0.0
	if minCost > 0 {
		variance = float64(maxCost-minCost) / float64(minCost) * 100
	}

	return GasImpact{
		MinCost:     minCost,
		MaxCost:     maxCost,
		MinProtocol: minProto,
		MaxProtocol: maxProto,
		Variance:    variance,
	}
}

// detectFeatureChanges identifies feature differences between protocol versions
func detectFeatureChanges(versions []uint32) []FeatureChange {
	if len(versions) < 2 {
		return nil
	}

	// Sort versions to ensure consistent comparison
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	var changes []FeatureChange

	// Compare consecutive protocol versions
	for i := 0; i < len(versions)-1; i++ {
		current, _ := Get(versions[i])
		next, _ := Get(versions[i+1])

		if current == nil || next == nil {
			continue
		}

		// Check for feature additions/removals
		for key, val := range next.Features {
			if _, exists := current.Features[key]; !exists {
				changes = append(changes, FeatureChange{
					FeatureName:       key,
					OldValue:          nil,
					NewValue:          val,
					AffectedProtocols: []uint32{versions[i], versions[i+1]},
				})
			}
		}

		// Check for value changes in existing features
		for key, currentVal := range current.Features {
			if nextVal, exists := next.Features[key]; exists {
				if !valuesEqual(currentVal, nextVal) {
					changes = append(changes, FeatureChange{
						FeatureName:       key,
						OldValue:          currentVal,
						NewValue:          nextVal,
						AffectedProtocols: []uint32{versions[i], versions[i+1]},
					})
				}
			}
		}
	}

	return changes
}

// valuesEqual compares two interface{} values for equality
func valuesEqual(a, b interface{}) bool {
	// Simple equality check; could be enhanced for complex types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
