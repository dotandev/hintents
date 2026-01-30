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

type GasCost struct {
	Name        string `json:"name"`
	Linear      uint64 `json:"linear"`
	Const       uint64 `json:"const"`
	Description string `json:"description,omitempty"`
}

type GasModel struct {
	Version        string         `json:"version"`
	NetworkID      string         `json:"network_id"`
	Metadata       ModelMetadata  `json:"metadata,omitempty"`
	CPUCosts       []GasCost      `json:"cpu_costs,omitempty"`
	HostCosts      []GasCost      `json:"host_costs,omitempty"`
	LedgerCosts    []GasCost      `json:"ledger_costs,omitempty"`
	ResourceLimits ResourceLimits `json:"resource_limits,omitempty"`
}

type ModelMetadata struct {
	NetworkName string `json:"network_name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Author      string `json:"author,omitempty"`
}

type ResourceLimits struct {
	MaxTxnSize       uint64 `json:"max_txn_size,omitempty"`
	MaxCPUInsns      uint64 `json:"max_cpu_insns,omitempty"`
	MaxMemory        uint64 `json:"max_memory,omitempty"`
	MaxLedgerEntries uint64 `json:"max_ledger_entries,omitempty"`
}
