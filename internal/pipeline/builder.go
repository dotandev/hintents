// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"encoding/json"
	"fmt"
)

// Pipeline simulates a Programmable Transaction Block (PTB).
type Pipeline struct {
	Commands []Command `json:"commands"`
}

// Command represents a single operation in a pipeline.
type Command struct {
	Type     string            `json:"type"`   // "MoveCall", "TransferObjects", etc.
	Target   string            `json:"target"` // e.g. "0x2::coin::mint"
	Args     []string          `json:"args"`   // arguments or references to previous results
	Metadata map[string]string `json:"metadata"`
}

// NewBuilder creates a new pipeline builder.
func NewBuilder() *Pipeline {
	return &Pipeline{
		Commands: make([]Command, 0),
	}
}

// AddCommand appends a command to the pipeline.
func (p *Pipeline) AddCommand(cmdType, target string, args []string) {
	p.Commands = append(p.Commands, Command{
		Type:   cmdType,
		Target: target,
		Args:   args,
	})
}

// ToJSON serializes the pipeline to JSON.
func (p *Pipeline) ToJSON() (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromJSON parses a pipeline from JSON.
func FromJSON(data []byte) (*Pipeline, error) {
	var p Pipeline
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline JSON: %w", err)
	}
	return &p, nil
}
