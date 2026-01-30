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
	"fmt"
)

type ValidationError struct {
	Field   string
	Message string
}

type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

func (g *GasModel) Validate() *ValidationResult {
	result := &ValidationResult{Valid: true}

	if g.Version == "" {
		result.addError("version", "version is required")
	}
	if g.NetworkID == "" {
		result.addError("network_id", "network_id is required")
	}

	result.validateCosts(g.CPUCosts, "cpu_costs")
	result.validateCosts(g.HostCosts, "host_costs")
	result.validateCosts(g.LedgerCosts, "ledger_costs")
	result.validateResourceLimits(g.ResourceLimits)
	result.validateNoDuplicates(g.AllCosts())

	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

func (g *GasModel) ValidateStrict() *ValidationResult {
	result := g.Validate()
	if !result.Valid {
		return result
	}
	result.validateMathematicalSoundness(g)
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

func (vr *ValidationResult) validateCosts(costs []GasCost, categoryName string) {
	const maxCost = 1000000000
	for i, cost := range costs {
		prefix := fmt.Sprintf("%s[%d]", categoryName, i)
		if cost.Name == "" {
			vr.addError(prefix, "name is required")
		}
		if cost.Linear > maxCost {
			vr.addError(fmt.Sprintf("%s.linear", prefix), "linear cost > 1B")
		}
		if cost.Const > maxCost {
			vr.addError(fmt.Sprintf("%s.const", prefix), "const cost > 1B")
		}
		if cost.Linear == 0 && cost.Const == 0 {
			vr.addError(prefix, "linear or const must be non-zero")
		}
	}
}

func (vr *ValidationResult) validateResourceLimits(limits ResourceLimits) {
	checks := []struct {
		value uint64
		min   uint64
		field string
		msg   string
	}{
		{limits.MaxTxnSize, 256, "max_txn_size", "too small (< 256)"},
		{limits.MaxCPUInsns, 1000, "max_cpu_insns", "too small (< 1000)"},
		{limits.MaxMemory, 1024, "max_memory", "too small (< 1024)"},
		{limits.MaxLedgerEntries, 1, "max_ledger_entries", "< 1"},
	}
	for _, c := range checks {
		if c.value > 0 && c.value < c.min {
			vr.addError("resource_limits."+c.field, c.msg)
		}
	}
	if limits.MaxTxnSize > 0 && limits.MaxMemory > 0 && limits.MaxTxnSize > limits.MaxMemory {
		vr.addWarning("resource_limits", "max_txn_size > max_memory")
	}
}

func (vr *ValidationResult) validateNoDuplicates(costs []GasCost) {
	seen := make(map[string]bool)
	for _, cost := range costs {
		if seen[cost.Name] {
			vr.addError("costs", fmt.Sprintf("duplicate: %s", cost.Name))
		}
		seen[cost.Name] = true
	}
}

func (vr *ValidationResult) validateMathematicalSoundness(g *GasModel) {
	allCosts := g.AllCosts()
	if len(allCosts) == 0 {
		vr.addWarning("costs", "no costs defined")
	}

	const maxUint64 = uint64(18446744073709551615)
	for _, cost := range allCosts {
		if cost.Linear > 0 && cost.Const > 0 && cost.Linear > maxUint64-cost.Const {
			vr.addError("costs", fmt.Sprintf("%s overflow risk", cost.Name))
		}
	}

	if g.ResourceLimits.MaxCPUInsns > 0 && len(g.CPUCosts) == 0 {
		vr.addWarning("cpu_costs", "limit set but no costs")
	}
	if g.ResourceLimits.MaxLedgerEntries > 0 && len(g.LedgerCosts) == 0 {
		vr.addWarning("ledger_costs", "limit set but no costs")
	}
}

func (vr *ValidationResult) addError(field, message string) {
	vr.Errors = append(vr.Errors, ValidationError{Field: field, Message: message})
}

func (vr *ValidationResult) addWarning(field, message string) {
	vr.Errors = append(vr.Errors, ValidationError{Field: field, Message: "[WARN] " + message})
}

func (vr *ValidationResult) ErrorsAsString() string {
	if vr.Valid {
		return ""
	}
	result := fmt.Sprintf("Validation failed (%d errors):\n", len(vr.Errors))
	for _, err := range vr.Errors {
		result += fmt.Sprintf("  [%s] %s\n", err.Field, err.Message)
	}
	return result
}
