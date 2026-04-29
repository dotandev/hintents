// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"
	"sync"
)

type WasmHost struct {
	registry *WasmRegistry
	modules  map[string]WasmModule
	mu       sync.Mutex
}

type WasmModule interface {
	Name() string
	Call(funcName string, args ...interface{}) (interface{}, error)
}

func NewWasmHost(ctx context.Context) (*WasmHost, error) {
	return &WasmHost{
		registry: NewWasmRegistry(),
		modules:  make(map[string]WasmModule),
	}, nil
}

func (h *WasmHost) LoadPlugin(ctx context.Context, name string, wasmBytes []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.modules[name] = &stubWasmModule{name: name}
	return nil
}

func (h *WasmHost) RegisterAnalysisPass(pass WasmAnalysisPass) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registry.AnalysisPasses[pass.Name] = pass
}

func (h *WasmHost) RegisterVisualizer(viz WasmVisualizer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registry.Visualizers[viz.Name] = viz
}

func (h *WasmHost) RegisterProtocolHandler(handler WasmProtocolHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registry.ProtocolHandlers[handler.Protocol] = handler
}

type stubWasmModule struct {
	name string
}

func (s *stubWasmModule) Name() string { return s.name }
func (s *stubWasmModule) Call(funcName string, args ...interface{}) (interface{}, error) {
	return nil, nil
}
