// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import "encoding/json"

// Version is the semantic versioning for the plugin API
const Version = "1.0.0"

// DecoderPlugin defines the interface for custom decoder plugins
type DecoderPlugin interface {
	// Name returns the plugin identifier
	Name() string

	// Version returns the plugin version following semver
	Version() string

	// CanDecode returns true if this plugin can handle the given event
	CanDecode(eventType string) bool

	// Decode processes the event and returns decoded data
	Decode(data []byte) (json.RawMessage, error)

	// Metadata returns plugin capabilities and requirements
	Metadata() Metadata
}

// Metadata describes plugin capabilities
type Metadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	APIVersion  string   `json:"api_version"`
	EventTypes  []string `json:"event_types"`
	Description string   `json:"description"`
}

// Factory creates a plugin instance
type Factory interface {
	Create() (DecoderPlugin, error)
}

// FactorySymbol is the exported symbol name for dynamic loading
const FactorySymbol = "NewPluginFactory"
