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

package simulator

// RunnerInterface defines the contract for simulator execution
type RunnerInterface interface {
	Run(req *SimulationRequest) (*SimulationResponse, error)
}

// NewRunnerInterface creates a RunnerInterface implementation
// This allows for easy swapping between real and mock implementations
func NewRunnerInterface() (RunnerInterface, error) {
	return NewRunner("", false)
}

// ExampleUsage of how commands can accept the interface
func ExampleUsage(runner RunnerInterface, req *SimulationRequest) (*SimulationResponse, error) {
	// Commands can now work with any implementation of RunnerInterface
	// This enables easy testing with mocks and flexible production usage
	return runner.Run(req)
}
