// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

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
