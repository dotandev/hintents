// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"fmt"
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
