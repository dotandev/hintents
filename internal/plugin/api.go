// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package plugin

type WasmAnalysisPass struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WasmVisualizer struct {
	Name string `json:"name"`
}

type WasmProtocolHandler struct {
	Protocol string `json:"protocol"`
}

type WasmRegistry struct {
	AnalysisPasses   map[string]WasmAnalysisPass
	Visualizers      map[string]WasmVisualizer
	ProtocolHandlers map[string]WasmProtocolHandler
}

func NewWasmRegistry() *WasmRegistry {
	return &WasmRegistry{
		AnalysisPasses:   make(map[string]WasmAnalysisPass),
		Visualizers:      make(map[string]WasmVisualizer),
		ProtocolHandlers: make(map[string]WasmProtocolHandler),
	}
}
