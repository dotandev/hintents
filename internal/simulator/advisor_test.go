// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func baselineForTest() Baselines {
	return Baselines{
		AvgCPUPerOp:    10_000_000,
		AvgMemoryPerOp: 5_000_000,
		AvgFeeStroops:  1_000,
		SyncedAt:       time.Now(),
		Source:         "network",
	}
}

func TestAdvisor_EfficientTransaction(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	gas := &GasEstimation{
		CPUCost:                8_000_000,
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
		CPUUsagePercent:        40,
		MemoryUsagePercent:     40,
	}

	report := advisor.Analyse(gas)
	assert.True(t, report.Efficient)
	assert.GreaterOrEqual(t, report.Score, 70.0)
}

func TestAdvisor_HighCPUTriggersWarning(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	gas := &GasEstimation{
		CPUCost:                15_000_000, // 1.5x average
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
	}

	report := advisor.Analyse(gas)
	found := false
	for _, tip := range report.Tips {
		if tip.Severity == "warning" {
			found = true
		}
	}
	assert.True(t, found, "expected a warning tip for high CPU")
}

func TestAdvisor_VeryHighCPUTriggersCritical(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	gas := &GasEstimation{
		CPUCost:                25_000_000, // 2.5x average
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
	}

	report := advisor.Analyse(gas)
	found := false
	for _, tip := range report.Tips {
		if tip.Severity == "critical" {
			found = true
		}
	}
	assert.True(t, found, "expected a critical tip for very high CPU")
	assert.LessOrEqual(t, report.Score, 70.0)
}

func TestAdvisor_DefaultBaselinesAddInfoTip(t *testing.T) {
	// No SyncBaselines call — uses hardcoded defaults
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(DefaultBaselines))

	gas := &GasEstimation{
		CPUCost:                8_000_000,
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
	}

	report := advisor.Analyse(gas)
	found := false
	for _, tip := range report.Tips {
		if tip.Severity == "info" {
			found = true
		}
	}
	assert.True(t, found, "expected info tip suggesting network sync")
}

func TestAdvisor_SyncBaselines_UsesProvider(t *testing.T) {
	custom := Baselines{
		AvgCPUPerOp:    20_000_000,
		AvgMemoryPerOp: 8_000_000,
		AvgFeeStroops:  2_000,
		SyncedAt:       time.Now(),
		Source:         "network",
	}
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(custom))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	bl := advisor.CurrentBaselines()
	assert.Equal(t, uint64(20_000_000), bl.AvgCPUPerOp)
	assert.Equal(t, "network", bl.Source)
	assert.True(t, bl.IsSynced())
}

func TestAdvisor_SyncBaselines_ProviderError(t *testing.T) {
	failing := &failingProvider{}
	advisor := NewGasOptimizationAdvisor(failing)
	err := advisor.SyncBaselines(context.Background())
	assert.Error(t, err)
}

func TestAdvisor_NetworkBaselineProvider_NilRunner(t *testing.T) {
	p := NewNetworkBaselineProvider(nil)
	_, err := p.FetchBaselines(context.Background())
	assert.Error(t, err)
}

func TestAdvisor_NetworkBaselineProvider_WithMockRunner(t *testing.T) {
	mock := NewMockRunner(func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
		return &SimulationResponse{
			Status: "success",
			BudgetUsage: &BudgetUsage{
				CPUInstructions:    12_000_000,
				MemoryBytes:        6_000_000,
				CPULimit:           100_000_000,
				MemoryLimit:        50_000_000,
				CPUUsagePercent:    12.0,
				MemoryUsagePercent: 12.0,
				OperationsCount:    3,
			},
		}, nil
	})

	p := NewNetworkBaselineProvider(mock)
	bl, err := p.FetchBaselines(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "network", bl.Source)
	assert.Equal(t, uint64(12_000_000), bl.AvgCPUPerOp)
	assert.True(t, bl.IsSynced())
}

func TestBaselines_IsSynced(t *testing.T) {
	assert.False(t, DefaultBaselines.IsSynced())
	synced := Baselines{Source: "network", SyncedAt: time.Now()}
	assert.True(t, synced.IsSynced())
}

// failingProvider always returns an error.
type failingProvider struct{}

func (f *failingProvider) FetchBaselines(_ context.Context) (*Baselines, error) {
	return nil, fmt.Errorf("network unavailable")
}

func TestCheckUnusedReadOnlyFootprint_NoUnused(t *testing.T) {
	declared := []string{"keyA", "keyB"}
	accessed := []string{"keyA", "keyB", "keyC"}

	report := CheckUnusedReadOnlyFootprint(declared, accessed)
	assert.Empty(t, report.UnusedReadOnlyKeys)
	assert.Equal(t, int64(0), report.EstimatedFeeSavings)
	assert.Nil(t, report.Tip())
}

func TestCheckUnusedReadOnlyFootprint_WithUnusedKeys(t *testing.T) {
	declared := []string{"keyA", "keyB", "keyC"}
	accessed := []string{"keyB"}

	report := CheckUnusedReadOnlyFootprint(declared, accessed)
	assert.Equal(t, []string{"keyA", "keyC"}, report.UnusedReadOnlyKeys)
	expectedSavings := int64(2) * EstimatedReadOnlyKeyFeeStroops
	assert.Equal(t, expectedSavings, report.EstimatedFeeSavings)

	tip := report.Tip()
	assert.NotNil(t, tip)
	assert.Equal(t, "warning", tip.Severity)
	assert.Contains(t, tip.Message, "Found 2 declared read-only footprint keys")
	assert.Contains(t, tip.Message, fmt.Sprintf("%d stroops", expectedSavings))
}

func TestCheckUnusedReadOnlyFootprint_SingleUnusedKey(t *testing.T) {
	declared := []string{"ContractData:Balance"}
	accessed := []string{}

	report := CheckUnusedReadOnlyFootprint(declared, accessed)
	assert.Equal(t, []string{"ContractData:Balance"}, report.UnusedReadOnlyKeys)
	assert.Equal(t, EstimatedReadOnlyKeyFeeStroops, report.EstimatedFeeSavings)

	tip := report.Tip()
	assert.NotNil(t, tip)
	assert.Equal(t, "warning", tip.Severity)
	assert.Contains(t, tip.Message, "ContractData:Balance")
	assert.Contains(t, tip.Message, fmt.Sprintf("%d stroops", EstimatedReadOnlyKeyFeeStroops))
}

func TestCheckUnusedReadOnlyFootprint_EmptyDeclared(t *testing.T) {
	report := CheckUnusedReadOnlyFootprint(nil, []string{"key1", "key2"})
	assert.Empty(t, report.UnusedReadOnlyKeys)
	assert.Equal(t, int64(0), report.EstimatedFeeSavings)
	assert.Nil(t, report.Tip())
}

func TestCheckUnusedReadOnlyFootprint_DeduplicatesDeclared(t *testing.T) {
	declared := []string{"keyA", "keyA", "keyB"}
	accessed := []string{"keyB"}

	report := CheckUnusedReadOnlyFootprint(declared, accessed)
	assert.Equal(t, []string{"keyA"}, report.UnusedReadOnlyKeys)
	assert.Equal(t, EstimatedReadOnlyKeyFeeStroops, report.EstimatedFeeSavings)
}

func TestAdvisor_SuggestUnusedFootprintKeys(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))

	declared := []string{"unusedKey", "usedKey"}
	accessed := []string{"usedKey"}

	unused, savings, tip := SuggestUnusedFootprintKeys(declared, accessed)
	assert.Equal(t, []string{"unusedKey"}, unused)
	assert.Equal(t, EstimatedReadOnlyKeyFeeStroops, savings)
	assert.NotNil(t, tip)
	assert.Equal(t, "warning", tip.Severity)

	// Also verify method on advisor
	unused2, savings2, tip2 := advisor.SuggestUnusedReadOnlyKeys(declared, accessed)
	assert.Equal(t, unused, unused2)
	assert.Equal(t, savings, savings2)
	assert.Equal(t, tip, tip2)
}

func TestAdvisor_CheckFootprint(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))

	declared := []string{"k1", "k2"}
	accessed := []string{"k1"}

	fpReport := advisor.CheckFootprint(declared, accessed)
	assert.Equal(t, []string{"k2"}, fpReport.UnusedReadOnlyKeys)
	assert.Equal(t, EstimatedReadOnlyKeyFeeStroops, fpReport.EstimatedFeeSavings)
}

func TestAdvisor_AnalyseWithFootprint_Clean(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	gas := &GasEstimation{
		CPUCost:                8_000_000,
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
		CPUUsagePercent:        40,
		MemoryUsagePercent:     40,
	}

	declared := []string{"key1", "key2"}
	accessed := []string{"key1", "key2"}

	report := advisor.AnalyseWithFootprint(gas, declared, accessed)
	assert.True(t, report.Efficient)
	assert.GreaterOrEqual(t, report.Score, 70.0)
	assert.NotNil(t, report.UnusedFootprintReport)
	assert.Empty(t, report.UnusedFootprintReport.UnusedReadOnlyKeys)

	// No footprint warning tip
	for _, tip := range report.Tips {
		assert.NotContains(t, tip.Message, "read-only footprint")
	}
}

func TestAdvisor_AnalyseWithFootprint_WithUnusedKeys(t *testing.T) {
	advisor := NewGasOptimizationAdvisor(NewStaticBaselineProvider(baselineForTest()))
	assert.NoError(t, advisor.SyncBaselines(context.Background()))

	gas := &GasEstimation{
		CPUCost:                8_000_000,
		MemoryCost:             4_000_000,
		EstimatedFeeLowerBound: 800,
		CPUUsagePercent:        40,
		MemoryUsagePercent:     40,
	}

	declared := []string{"usedKey", "unusedKey1", "unusedKey2"}
	accessed := []string{"usedKey"}

	cleanReport := advisor.Analyse(gas)
	footprintReport := advisor.AnalyseWithFootprint(gas, declared, accessed)

	// Score is penalized by 10 for having unused read-only keys
	assert.Equal(t, cleanReport.Score-10, footprintReport.Score)
	assert.NotNil(t, footprintReport.UnusedFootprintReport)
	assert.Equal(t, []string{"unusedKey1", "unusedKey2"}, footprintReport.UnusedFootprintReport.UnusedReadOnlyKeys)
	assert.Equal(t, int64(2)*EstimatedReadOnlyKeyFeeStroops, footprintReport.UnusedFootprintReport.EstimatedFeeSavings)

	// Has a warning tip for unused read-only footprint keys
	found := false
	for _, tip := range footprintReport.Tips {
		if tip.Severity == "warning" && strings.Contains(tip.Message, "read-only footprint") {
			found = true
			assert.Contains(t, tip.Message, "unusedKey1")
			assert.Contains(t, tip.Message, "unusedKey2")
		}
	}
	assert.True(t, found, "expected warning tip for unused read-only footprint keys")
}

func TestExtractReadOnlyFootprintFromEnvelope_NilAndNonSoroban(t *testing.T) {
	assert.Empty(t, ExtractReadOnlyFootprintFromEnvelope(nil))

	_, err := ExtractReadOnlyFootprintFromEnvelopeXDR("invalid base64!!!")
	assert.Error(t, err)
}
