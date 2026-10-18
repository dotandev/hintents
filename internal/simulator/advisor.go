// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Baselines holds average cost-per-operation values used to score efficiency.
type Baselines struct {
	AvgCPUPerOp    uint64    `json:"avg_cpu_per_op"`
	AvgMemoryPerOp uint64    `json:"avg_memory_per_op"`
	AvgFeeStroops  int64     `json:"avg_fee_stroops"`
	SyncedAt       time.Time `json:"synced_at"`
	Source         string    `json:"source"` // "hardcoded" | "network" | "file"
}

// DefaultBaselines are conservative fallback values used before any network sync.
var DefaultBaselines = Baselines{
	AvgCPUPerOp:    10_000_000,
	AvgMemoryPerOp: 5_000_000,
	AvgFeeStroops:  1_000,
	Source:         "hardcoded",
}

// IsSynced returns true when baselines came from the network, not defaults.
func (b *Baselines) IsSynced() bool {
	return b.Source != "hardcoded" && !b.SyncedAt.IsZero()
}

// BaselineProvider is the interface for fetching dynamic baselines.
type BaselineProvider interface {
	FetchBaselines(ctx context.Context) (*Baselines, error)
}

// StaticBaselineProvider always returns a fixed Baselines value.
// Useful for testing and offline mode.
type StaticBaselineProvider struct {
	baselines Baselines
}

func NewStaticBaselineProvider(b Baselines) *StaticBaselineProvider {
	return &StaticBaselineProvider{baselines: b}
}

func (p *StaticBaselineProvider) FetchBaselines(_ context.Context) (*Baselines, error) {
	return &p.baselines, nil
}

// NetworkBaselineProvider derives baselines from a live simulation run.
type NetworkBaselineProvider struct {
	runner RunnerInterface
}

func NewNetworkBaselineProvider(runner RunnerInterface) *NetworkBaselineProvider {
	return &NetworkBaselineProvider{runner: runner}
}

func (p *NetworkBaselineProvider) FetchBaselines(ctx context.Context) (*Baselines, error) {
	if p.runner == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	// Use the runner's last known gas estimate as a network sample.
	// In production this would aggregate recent ledger fee stats.
	req := &SimulationRequest{
		EnvelopeXdr:   "AAAA",
		ResultMetaXdr: "AAAA",
	}
	resp, err := p.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("baseline fetch failed: %w", err)
	}
	if resp.BudgetUsage == nil {
		return &DefaultBaselines, nil
	}
	b := resp.BudgetUsage
	avgFee := BaseFeeStroops + int64(b.CPUInstructions/CPUStroopsPerUnit) + int64(b.MemoryBytes/MemStroopsPerUnit)
	return &Baselines{
		AvgCPUPerOp:    b.CPUInstructions,
		AvgMemoryPerOp: b.MemoryBytes,
		AvgFeeStroops:  avgFee,
		SyncedAt:       time.Now(),
		Source:         "network",
	}, nil
}

// ─── Advisor ─────────────────────────────────────────────────────────────────

// Tip is a single optimization suggestion.
type Tip struct {
	Severity string // "info" | "warning" | "critical"
	Message  string
}

// AdvisorReport is the result of analysing a GasEstimation.
type AdvisorReport struct {
	Efficient bool
	Score     float64 // 0–100, higher is better
	Tips      []Tip
	Baselines Baselines
}

// GasOptimizationAdvisor scores a GasEstimation against dynamic baselines
// and produces human-readable optimisation tips.
type GasOptimizationAdvisor struct {
	mu       sync.RWMutex
	provider BaselineProvider
	current  Baselines
}

// NewGasOptimizationAdvisor creates an advisor using the given provider.
// Call SyncBaselines to load live data before analysing transactions.
func NewGasOptimizationAdvisor(provider BaselineProvider) *GasOptimizationAdvisor {
	return &GasOptimizationAdvisor{
		provider: provider,
		current:  DefaultBaselines,
	}
}

// SyncBaselines fetches fresh baselines from the provider and stores them.
func (a *GasOptimizationAdvisor) SyncBaselines(ctx context.Context) error {
	b, err := a.provider.FetchBaselines(ctx)
	if err != nil {
		return fmt.Errorf("sync baselines: %w", err)
	}
	a.mu.Lock()
	a.current = *b
	a.mu.Unlock()
	return nil
}

// CurrentBaselines returns a snapshot of the currently loaded baselines.
func (a *GasOptimizationAdvisor) CurrentBaselines() Baselines {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

// Analyse scores gas against the current baselines and returns tips.
func (a *GasOptimizationAdvisor) Analyse(gas *GasEstimation) AdvisorReport {
	a.mu.RLock()
	bl := a.current
	a.mu.RUnlock()

	var tips []Tip
	score := 100.0

	// CPU efficiency
	if bl.AvgCPUPerOp > 0 {
		ratio := float64(gas.CPUCost) / float64(bl.AvgCPUPerOp)
		if ratio > 2.0 {
			score -= 30
			tips = append(tips, Tip{"critical", fmt.Sprintf("CPU cost is %.1fx the network average — consider reducing host function calls", ratio)})
		} else if ratio > 1.2 {
			score -= 15
			tips = append(tips, Tip{"warning", fmt.Sprintf("CPU cost is %.1fx the network average", ratio)})
		}
	}

	// Memory efficiency
	if bl.AvgMemoryPerOp > 0 {
		ratio := float64(gas.MemoryCost) / float64(bl.AvgMemoryPerOp)
		if ratio > 2.0 {
			score -= 30
			tips = append(tips, Tip{"critical", fmt.Sprintf("Memory cost is %.1fx the network average — reduce large data reads", ratio)})
		} else if ratio > 1.2 {
			score -= 15
			tips = append(tips, Tip{"warning", fmt.Sprintf("Memory cost is %.1fx the network average", ratio)})
		}
	}

	// Fee efficiency
	if bl.AvgFeeStroops > 0 {
		ratio := float64(gas.EstimatedFeeLowerBound) / float64(bl.AvgFeeStroops)
		if ratio > 2.0 {
			score -= 20
			tips = append(tips, Tip{"critical", fmt.Sprintf("Estimated fee is %.1fx the network average", ratio)})
		} else if ratio > 1.2 {
			score -= 10
			tips = append(tips, Tip{"warning", fmt.Sprintf("Estimated fee is %.1fx the network average", ratio)})
		}
	}

	// Budget pressure tips
	if gas.IsCPUCritical() {
		tips = append(tips, Tip{"critical", "CPU budget critically high — transaction may fail under load"})
	}
	if gas.IsMemoryCritical() {
		tips = append(tips, Tip{"critical", "Memory budget critically high"})
	}

	if score < 0 {
		score = 0
	}

	if !bl.IsSynced() {
		tips = append(tips, Tip{"info", "Baselines are hardcoded defaults — run 'erst gas-advisor sync' for network-accurate scoring"})
	}

	return AdvisorReport{
		Efficient: score >= 70,
		Score:     score,
		Tips:      tips,
		Baselines: bl,
	}
}
