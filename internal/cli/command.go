// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"io"

	"github.com/spf13/cobra"
)

// Command defines the interface for all CLI commands in the redesigned framework.
// It supports Protocol V2 requirements including explicit versioning,
// dependency injection, and structured error handling.
type Command interface {
	// Name returns the command name (e.g., "debug")
	Name() string
	
	// Description returns a short description
	Description() string
	
	// CreateCobraCommand creates and configures the underlying cobra.Command
	CreateCobraCommand() *cobra.Command
	
	// Validate checks if the command arguments and flags are valid
	Validate(ctx context.Context, args []string) error
	
	// Execute runs the command logic
	Execute(ctx context.Context, args []string) error
	
	// GetProtocolVersion returns the required protocol version
	GetProtocolVersion() uint32
}

// BaseCommand provides default implementations for Command interface
type BaseCommand struct {
	Use     string
	Short   string
	Long    string
	Example string
	
	// ProtocolVersion indicates the minimum protocol version required
	ProtocolVersion uint32
	
	// Dependencies
	Stdout io.Writer
	Stderr io.Writer
}

func (c *BaseCommand) GetProtocolVersion() uint32 {
	return c.ProtocolVersion
}

// Registry manages command registration and lifecycle
type Registry struct {
	commands map[string]Command
	root     *cobra.Command
}

// NewRegistry creates a new command registry
func NewRegistry(root *cobra.Command) *Registry {
	return &Registry{
		commands: make(map[string]Command),
		root:     root,
	}
}

// Register adds a command to the registry and the root command
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
	cobraCmd := cmd.CreateCobraCommand()
	r.root.AddCommand(cobraCmd)
}

// Get returns a command by name
func (r *Registry) Get(name string) Command {
	return r.commands[name]
}
