// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
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

// EstimatedReadOnlyKeyFeeStroops is the estimated fee cost per declared read-only footprint key in stroops.
// Under the Soroban resource fee model, each declared read-only ledger entry incurs a base read
// entry fee plus byte read charges, regardless of whether the key is actually accessed during execution.
const EstimatedReadOnlyKeyFeeStroops int64 = 1_000

// UnusedFootprintReport details declared read-only footprint keys that were never accessed
// during simulation, along with the estimated fee savings.
type UnusedFootprintReport struct {
	DeclaredReadOnlyKeys []string `json:"declared_read_only_keys"`
	AccessedKeys         []string `json:"accessed_keys"`
	UnusedReadOnlyKeys   []string `json:"unused_read_only_keys"`
	EstimatedFeeSavings  int64    `json:"estimated_fee_savings"` // in stroops
}

// Tip returns an optimization suggestion Tip if any declared read-only keys were unused.
func (r *UnusedFootprintReport) Tip() *Tip {
	if len(r.UnusedReadOnlyKeys) == 0 {
		return nil
	}
	var msg string
	if len(r.UnusedReadOnlyKeys) == 1 {
		msg = fmt.Sprintf("Declared read-only footprint key %q was never accessed during simulation — removing it could save an estimated %d stroops in fees", r.UnusedReadOnlyKeys[0], r.EstimatedFeeSavings)
	} else {
		msg = fmt.Sprintf("Found %d declared read-only footprint keys never accessed during simulation (%s) — removing them could save an estimated %d stroops in fees", len(r.UnusedReadOnlyKeys), strings.Join(r.UnusedReadOnlyKeys, ", "), r.EstimatedFeeSavings)
	}
	return &Tip{
		Severity: "warning",
		Message:  msg,
	}
}

// Tip is a single optimization suggestion.
type Tip struct {
	Severity string // "info" | "warning" | "critical"
	Message  string
}

// AdvisorReport is the result of analysing a GasEstimation and optional footprint usage.
type AdvisorReport struct {
	Efficient             bool                   `json:"efficient"`
	Score                 float64                `json:"score"` // 0–100, higher is better
	Tips                  []Tip                  `json:"tips"`
	Baselines             Baselines              `json:"baselines"`
	UnusedFootprintReport *UnusedFootprintReport `json:"unused_footprint_report,omitempty"`
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

// CheckUnusedReadOnlyFootprint compares declared read-only footprint keys with keys
// actually accessed during simulation, returning the unused keys and estimated fee savings.
func CheckUnusedReadOnlyFootprint(declaredReadOnly []string, accessedKeys []string) UnusedFootprintReport {
	if len(declaredReadOnly) == 0 {
		return UnusedFootprintReport{
			DeclaredReadOnlyKeys: []string{},
			AccessedKeys:         accessedKeys,
			UnusedReadOnlyKeys:   []string{},
			EstimatedFeeSavings:  0,
		}
	}

	accessedSet := make(map[string]struct{}, len(accessedKeys))
	for _, k := range accessedKeys {
		accessedSet[k] = struct{}{}
	}

	seenDeclared := make(map[string]struct{}, len(declaredReadOnly))
	var unused []string

	for _, k := range declaredReadOnly {
		if _, seen := seenDeclared[k]; seen {
			continue
		}
		seenDeclared[k] = struct{}{}
		if _, accessed := accessedSet[k]; !accessed {
			unused = append(unused, k)
		}
	}

	savings := int64(len(unused)) * EstimatedReadOnlyKeyFeeStroops

	return UnusedFootprintReport{
		DeclaredReadOnlyKeys: declaredReadOnly,
		AccessedKeys:         accessedKeys,
		UnusedReadOnlyKeys:   unused,
		EstimatedFeeSavings:  savings,
	}
}

// SuggestUnusedFootprintKeys compares declared read-only footprint keys against accessed keys,
// returning the unused keys, estimated fee savings (in stroops), and an optimization Tip.
func SuggestUnusedFootprintKeys(declaredReadOnly []string, accessedKeys []string) ([]string, int64, *Tip) {
	fpReport := CheckUnusedReadOnlyFootprint(declaredReadOnly, accessedKeys)
	return fpReport.UnusedReadOnlyKeys, fpReport.EstimatedFeeSavings, fpReport.Tip()
}

// CheckFootprint compares the declared read-only footprint with keys accessed during simulation,
// returning an UnusedFootprintReport.
func (a *GasOptimizationAdvisor) CheckFootprint(declaredReadOnly []string, accessedKeys []string) UnusedFootprintReport {
	return CheckUnusedReadOnlyFootprint(declaredReadOnly, accessedKeys)
}

// SuggestUnusedReadOnlyKeys compares declared read-only footprint keys against accessed keys,
// returning the unused keys, estimated fee savings (in stroops), and an optimization Tip.
func (a *GasOptimizationAdvisor) SuggestUnusedReadOnlyKeys(declaredReadOnly []string, accessedKeys []string) ([]string, int64, *Tip) {
	return SuggestUnusedFootprintKeys(declaredReadOnly, accessedKeys)
}

// AnalyseWithFootprint scores gas against baselines and analyzes declared read-only footprint usage,
// appending optimization tips and penalizing the score if unused keys are found.
func (a *GasOptimizationAdvisor) AnalyseWithFootprint(gas *GasEstimation, declaredReadOnly []string, accessedKeys []string) AdvisorReport {
	report := a.Analyse(gas)

	fpReport := CheckUnusedReadOnlyFootprint(declaredReadOnly, accessedKeys)
	report.UnusedFootprintReport = &fpReport

	if tip := fpReport.Tip(); tip != nil {
		report.Tips = append(report.Tips, *tip)
		report.Score -= 10
		if report.Score < 0 {
			report.Score = 0
		}
		report.Efficient = report.Score >= 70
	}

	return report
}

// ExtractReadOnlyFootprintFromEnvelope extracts read-only ledger keys from a Stellar transaction envelope.
// If the envelope does not contain Soroban resources or read-only footprint keys, it returns an empty slice.
func ExtractReadOnlyFootprintFromEnvelope(env *xdr.TransactionEnvelope) []string {
	if env == nil {
		return []string{}
	}
	var tx *xdr.Transaction
	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		if env.V1 != nil {
			tx = &env.V1.Tx
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		if env.FeeBump != nil && env.FeeBump.Tx.InnerTx.V1 != nil {
			tx = &env.FeeBump.Tx.InnerTx.V1.Tx
		}
	default:
		return []string{}
	}

	if tx == nil || tx.Ext.V != 1 || tx.Ext.SorobanData == nil {
		return []string{}
	}

	var keys []string
	for _, k := range tx.Ext.SorobanData.Resources.Footprint.ReadOnly {
		b, err := k.MarshalBinary()
		if err != nil {
			continue
		}
		keys = append(keys, base64.StdEncoding.EncodeToString(b))
	}
	return keys
}

// ExtractReadOnlyFootprintFromEnvelopeXDR decodes a base64-encoded TransactionEnvelope XDR
// and extracts its declared read-only footprint keys.
func ExtractReadOnlyFootprintFromEnvelopeXDR(envXdrB64 string) ([]string, error) {
	raw, err := base64.StdEncoding.DecodeString(envXdrB64)
	if err != nil {
		return nil, fmt.Errorf("decode envelope base64: %w", err)
	}
	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return ExtractReadOnlyFootprintFromEnvelope(&env), nil
}
