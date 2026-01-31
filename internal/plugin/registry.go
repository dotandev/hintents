// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
)

// Registry manages the plugin ecosystem with isolation and versioning
type Registry struct {
	mu     sync.RWMutex
	loader *Loader
	cache  map[string]json.RawMessage
}

// NewRegistry initializes a fresh registry
func NewRegistry() *Registry {
	return &Registry{
		loader: NewLoader(),
		cache:  make(map[string]json.RawMessage),
	}
}

// LoadFromDirectory scans and loads all plugins from a directory
func (r *Registry) LoadFromDirectory(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pattern := filepath.Join(dir, "*.so")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to scan plugin directory: %w", err)
	}

	var loadErrors []error
	for _, path := range matches {
		if err := r.loader.Load(path); err != nil {
			loadErrors = append(loadErrors, err)
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("encountered %d plugin loading errors", len(loadErrors))
	}

	return nil
}

// Decode uses a plugin to decode an event
func (r *Registry) Decode(pluginName string, eventType string, data []byte) (json.RawMessage, error) {
	r.mu.RLock()

	p, ok := r.loader.Get(pluginName)
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	if !p.CanDecode(eventType) {
		r.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s cannot decode event type %s", pluginName, eventType)
	}

	r.mu.RUnlock()

	result, err := p.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("plugin %s decode failed: %w", pluginName, err)
	}

	return result, nil
}

// FindAndDecode searches for a capable plugin and decodes the event
func (r *Registry) FindAndDecode(eventType string, data []byte) (json.RawMessage, string, error) {
	r.mu.RLock()
	p, ok := r.loader.FindForEvent(eventType)
	r.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("no plugin available for event type %s", eventType)
	}

	result, err := p.Decode(data)
	if err != nil {
		return nil, "", err
	}

	return result, p.Name(), nil
}

// ListPlugins returns information about all loaded plugins
func (r *Registry) ListPlugins() []PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.loader.List()
	metadata := make([]PluginMetadata, 0, len(names))

	for _, name := range names {
		if p, ok := r.loader.Get(name); ok {
			metadata = append(metadata, p.Metadata())
		}
	}

	return metadata
}

// Clear removes all loaded plugins
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loader = NewLoader()
	r.cache = make(map[string]json.RawMessage)
}
