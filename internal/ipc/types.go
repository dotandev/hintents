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

package ipc

import "encoding/json"

func UnmarshalSimulationRequestSchema(data []byte) (SimulationRequestSchema, error) {
	var r SimulationRequestSchema
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SimulationRequestSchema) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSimulationResponseSchema(data []byte) (SimulationResponseSchema, error) {
	var r SimulationResponseSchema
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SimulationResponseSchema) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type SimulationRequestSchema struct {
	Network Network `json:"network"`
	// Client-generated unique request identifier
	RequestID string `json:"request_id"`
	Version   string `json:"version"`
	Xdr       string `json:"xdr"`
}

type SimulationResponseSchema struct {
	Error     *Error  `json:"error,omitempty"`
	RequestID string  `json:"request_id"`
	Result    *Result `json:"result,omitempty"`
	Success   bool    `json:"success"`
	Version   string  `json:"version"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	// Fee charged in stroops
	FeeCharged string `json:"fee_charged"`
}

type Network string

const (
	Futurenet Network = "futurenet"
	Public    Network = "public"
	Testnet   Network = "testnet"
)
