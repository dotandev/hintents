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
