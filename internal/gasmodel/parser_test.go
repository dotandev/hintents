// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gasmodel

import (
	"testing"
)

func TestParseGasModelFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *GasModel
		wantErr bool
	}{
		{
			name: "valid basic model",
			data: []byte(`{
				"version": "1.0",
				"network_id": "test-private-1",
				"cpu_costs": [
					{
						"name": "wasm_inst",
						"linear": 100,
						"const": 10
					}
				],
				"host_costs": [
					{
						"name": "host_invoke",
						"linear": 500,
						"const": 50
					}
				]
			}`),
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid json}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGasModelFromBytes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGasModelFromBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("ParseGasModelFromBytes() got nil, want model")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		model     *GasModel
		wantValid bool
	}{
		{
			name: "valid model",
			model: &GasModel{
				Version:   "1.0",
				NetworkID: "test-network",
				CPUCosts: []GasCost{
					{Name: "wasm_inst", Linear: 100, Const: 10},
				},
			},
			wantValid: true,
		},
		{
			name: "missing version",
			model: &GasModel{
				NetworkID: "test-network",
			},
			wantValid: false,
		},
		{
			name: "missing network_id",
			model: &GasModel{
				Version: "1.0",
			},
			wantValid: false,
		},
		{
			name: "cost with zero linear and const",
			model: &GasModel{
				Version:   "1.0",
				NetworkID: "test-network",
				CPUCosts: []GasCost{
					{Name: "wasm_inst", Linear: 0, Const: 0},
				},
			},
			wantValid: false,
		},
		{
			name: "duplicate cost names",
			model: &GasModel{
				Version:   "1.0",
				NetworkID: "test-network",
				CPUCosts: []GasCost{
					{Name: "wasm_inst", Linear: 100, Const: 10},
					{Name: "wasm_inst", Linear: 200, Const: 20},
				},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.Validate()
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() Valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestGetCostByName(t *testing.T) {
	model := &GasModel{
		CPUCosts: []GasCost{
			{Name: "wasm_inst", Linear: 100, Const: 10},
		},
		HostCosts: []GasCost{
			{Name: "host_invoke", Linear: 500, Const: 50},
		},
	}

	cost := model.GetCostByName("wasm_inst")
	if cost == nil || cost.Name != "wasm_inst" {
		t.Errorf("GetCostByName() failed to find cost")
	}

	cost = model.GetCostByName("nonexistent")
	if cost != nil {
		t.Errorf("GetCostByName() should return nil for nonexistent cost")
	}
}

func TestAllCosts(t *testing.T) {
	model := &GasModel{
		CPUCosts: []GasCost{
			{Name: "wasm_inst", Linear: 100, Const: 10},
		},
		HostCosts: []GasCost{
			{Name: "host_invoke", Linear: 500, Const: 50},
		},
		LedgerCosts: []GasCost{
			{Name: "ledger_read", Linear: 1000, Const: 100},
		},
	}

	costs := model.AllCosts()
	if len(costs) != 3 {
		t.Errorf("AllCosts() got %d costs, want 3", len(costs))
	}
}

func BenchmarkValidate(b *testing.B) {
	model := &GasModel{
		Version:   "1.0",
		NetworkID: "test-network",
		CPUCosts: []GasCost{
			{Name: "wasm_inst", Linear: 100, Const: 10},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.Validate()
	}
}
