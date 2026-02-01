// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package gasmodel

import (
	"encoding/json"
	"fmt"
	"os"
)

func ParseGasModel(filePath string) (*GasModel, error) {
	if filePath == "" {
		return nil, fmt.Errorf("gas model file path cannot be empty")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read gas model file: %w", err)
	}
	return ParseGasModelFromBytes(data)
}

func ParseGasModelFromBytes(data []byte) (*GasModel, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("gas model data cannot be empty")
	}
	var model GasModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("failed to parse gas model JSON: %w", err)
	}
	return &model, nil
}

func (g *GasModel) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}

func (g *GasModel) ToJSONString() (string, error) {
	data, err := g.ToJSON()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g *GasModel) GetCostByName(name string) *GasCost {
	for _, costs := range [][]GasCost{g.CPUCosts, g.HostCosts, g.LedgerCosts} {
		for i := range costs {
			if costs[i].Name == name {
				return &costs[i]
			}
		}
	}
	return nil
}

func (g *GasModel) AllCosts() []GasCost {
	var all []GasCost
	all = append(all, g.CPUCosts...)
	all = append(all, g.HostCosts...)
	all = append(all, g.LedgerCosts...)
	return all
}
