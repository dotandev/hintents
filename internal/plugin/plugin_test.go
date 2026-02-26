// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"testing"
)

var _ DecoderPlugin = (*mockDecoder)(nil)

type mockDecoder struct {
	name       string
	version    string
	canDecodes map[string]bool
	decodeErr  error
}

func (m *mockDecoder) Name() string {
	return m.name
}

func (m *mockDecoder) Version() string {
	return m.version
}

func (m *mockDecoder) CanDecode(eventType string) bool {
	return m.canDecodes[eventType]
}

func (m *mockDecoder) Decode(data []byte) (json.RawMessage, error) {
	if m.decodeErr != nil {
		return nil, m.decodeErr
	}
	return json.RawMessage(`{"decoded": true}`), nil
}

func (m *mockDecoder) Metadata() PluginMetadata {
	return PluginMetadata{
		Name:       m.name,
		Version:    m.version,
		APIVersion: Version,
		EventTypes: []string{"test.event"},
	}
}

func TestLoaderBasic(t *testing.T) {
	l := NewLoader()

	p, ok := l.Get("nonexistent")
	if ok {
		t.Errorf("expected plugin not found")
	}
	if p != nil {
		t.Errorf("expected nil plugin")
	}
}

func TestRegistryDecoding(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	mock := &mockDecoder{
		name:       "test-decoder",
		version:    "1.0.0",
		canDecodes: map[string]bool{"test.event": true},
	}
	r.loader.plugins["test-decoder"] = mock
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]string{"key": "value"})
	result, err := r.Decode("test-decoder", "test.event", data)

	if err != nil {
		t.Errorf("unexpected decode error: %v", err)
	}
	if !json.Valid(result) {
		t.Errorf("decode result is not valid JSON")
	}
}

func TestFindAndDecode(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	mock := &mockDecoder{
		name:       "finder-plugin",
		version:    "1.0.0",
		canDecodes: map[string]bool{"custom.event": true},
	}
	r.loader.plugins["finder-plugin"] = mock
	r.mu.Unlock()

	data, _ := json.Marshal(map[string]string{"data": "test"})
	result, pluginName, err := r.FindAndDecode("custom.event", data)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pluginName != "finder-plugin" {
		t.Errorf("expected plugin finder-plugin, got %s", pluginName)
	}
	if !json.Valid(result) {
		t.Errorf("result is not valid JSON")
	}
}

func TestListPlugins(t *testing.T) {
	r := NewRegistry()

	r.mu.Lock()
	r.loader.plugins["plugin1"] = &mockDecoder{
		name:    "plugin1",
		version: "1.0.0",
	}
	r.loader.plugins["plugin2"] = &mockDecoder{
		name:    "plugin2",
		version: "2.0.0",
	}
	r.mu.Unlock()

	plugins := r.ListPlugins()

	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestManagerInitialization(t *testing.T) {
	m, err := NewManager("")
	if err != nil {
		t.Errorf("failed to create manager: %v", err)
	}
	if m == nil {
		t.Errorf("manager is nil")
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		plugin  DecoderPlugin
		wantErr bool
	}{
		{
			name: "valid plugin",
			plugin: &mockDecoder{
				name:    "valid",
				version: "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			plugin: &mockDecoder{
				name:    "",
				version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "empty version",
			plugin: &mockDecoder{
				name:    "test",
				version: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlugin(tt.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePlugin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
