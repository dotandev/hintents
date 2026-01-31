package main

import (
	"encoding/json"
	"fmt"

	"github.com/dotandev/hintents/internal/plugin"
)

// CustomDecoder is an example implementation of a DecoderPlugin
type CustomDecoder struct{}

func (d *CustomDecoder) Name() string {
	return "custom-decoder"
}

func (d *CustomDecoder) Version() string {
	return "1.0.0"
}

func (d *CustomDecoder) CanDecode(eventType string) bool {
	return eventType == "custom.event" || eventType == "proprietary.format"
}

func (d *CustomDecoder) Decode(data []byte) (json.RawMessage, error) {
	var payload struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	decoded := struct {
		Decoded bool   `json:"decoded"`
		Type    string `json:"type"`
		Value   string `json:"value"`
		Plugin  string `json:"plugin"`
	}{
		Decoded: true,
		Type:    payload.Type,
		Value:   payload.Value,
		Plugin:  d.Name(),
	}

	return json.Marshal(decoded)
}

func (d *CustomDecoder) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:        d.Name(),
		Version:     d.Version(),
		APIVersion:  plugin.Version,
		EventTypes:  []string{"custom.event", "proprietary.format"},
		Description: "Example custom event decoder for proprietary formats",
	}
}

// NewPluginFactory exports the factory function for dynamic loading
func NewPluginFactory() (plugin.DecoderPlugin, error) {
	return &CustomDecoder{}, nil
}

func main() {
	// This exists to satisfy Go's requirement for a main package to have a main function
	// but this plugin is intended to be built as a .so file.
}
