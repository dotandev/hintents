// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manager coordinates plugin operations with the main decoder system
type Manager struct {
	registry *Registry
	baseDir  string
}

// NewManager creates a plugin manager with an optional base directory
func NewManager(baseDir string) (*Manager, error) {
	if baseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to determine working directory: %w", err)
		}
		baseDir = cwd
	}

	return &Manager{
		registry: NewRegistry(),
		baseDir:  baseDir,
	}, nil
}

// Initialize loads plugins from the plugins directory
func (m *Manager) Initialize() error {
	pluginDir := filepath.Join(m.baseDir, "plugins")

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugins directory not found at %s", pluginDir)
	}

	return m.registry.LoadFromDirectory(pluginDir)
}

// DecodeEvent decodes using the most appropriate plugin
func (m *Manager) DecodeEvent(eventType string, data []byte) (json.RawMessage, error) {
	result, _, err := m.registry.FindAndDecode(eventType, data)
	return result, err
}

// DecodeEventWithPlugin uses a specific plugin
func (m *Manager) DecodeEventWithPlugin(pluginName string, eventType string, data []byte) (json.RawMessage, error) {
	return m.registry.Decode(pluginName, eventType, data)
}

// GetPlugins returns available plugins
func (m *Manager) GetPlugins() []PluginMetadata {
	return m.registry.ListPlugins()
}

// GetPlugin retrieves a specific plugin
func (m *Manager) GetPlugin(name string) (DecoderPlugin, bool) {
	m.registry.mu.RLock()
	defer m.registry.mu.RUnlock()
	return m.registry.loader.Get(name)
}
