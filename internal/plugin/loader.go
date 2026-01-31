// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"plugin"
	"sync"
)

// Loader manages plugin discovery and initialization
type Loader struct {
	mu      sync.RWMutex
	plugins map[string]DecoderPlugin
}

// NewLoader creates a new plugin loader
func NewLoader() *Loader {
	return &Loader{
		plugins: make(map[string]DecoderPlugin),
	}
}

// Load opens and initializes a plugin from a shared library file
func (l *Loader) Load(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", path, err)
	}

	sym, err := p.Lookup(FactorySymbol)
	if err != nil {
		return fmt.Errorf("plugin %s missing factory symbol: %w", path, err)
	}

	factory, ok := sym.(func() (DecoderPlugin, error))
	if !ok {
		return fmt.Errorf("plugin %s has invalid factory signature", path)
	}

	instance, err := factory()
	if err != nil {
		return fmt.Errorf("plugin %s factory failed: %w", path, err)
	}

	if err := validatePlugin(instance); err != nil {
		return fmt.Errorf("plugin %s validation failed: %w", path, err)
	}

	l.plugins[instance.Name()] = instance
	return nil
}

// Get retrieves a loaded plugin by name
func (l *Loader) Get(name string) (DecoderPlugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.plugins[name]
	return p, ok
}

// List returns all loaded plugin names
func (l *Loader) List() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.plugins))
	for name := range l.plugins {
		names = append(names, name)
	}
	return names
}

// FindForEvent returns the first plugin that can decode the event type
func (l *Loader) FindForEvent(eventType string) (DecoderPlugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, p := range l.plugins {
		if p.CanDecode(eventType) {
			return p, true
		}
	}
	return nil, false
}

func validatePlugin(p DecoderPlugin) error {
	if p.Name() == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if p.Version() == "" {
		return fmt.Errorf("plugin version cannot be empty")
	}

	meta := p.Metadata()
	if meta.APIVersion != Version {
		return fmt.Errorf("plugin API version %s does not match current %s", meta.APIVersion, Version)
	}

	return nil
}
