// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dotandev/hintents/internal/simulator"
)

// ProtocolManager handles protocol version negotiation and validation
type ProtocolManager struct {
	CurrentVersion uint32
	MinSupported   uint32
	MaxSupported   uint32
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager() *ProtocolManager {
	return &ProtocolManager{
		CurrentVersion: simulator.LatestVersion(), // Default to latest (22)
		MinSupported:   20,
		MaxSupported:   22,
	}
}

// Validate checks if the requested version is supported
func (pm *ProtocolManager) Validate(version uint32) error {
	if version < pm.MinSupported || version > pm.MaxSupported {
		return fmt.Errorf("protocol version %d is not supported (supported: %d-%d)", version, pm.MinSupported, pm.MaxSupported)
	}
	return nil
}

// CheckCompatibility verifies if a command is compatible with the current protocol
func (pm *ProtocolManager) CheckCompatibility(cmd Command) error {
	// If the command doesn't specify a version, assume it's compatible with all
	// If it does, check against current version
	
	// Check version
	reqVersion := cmd.GetProtocolVersion()
	if reqVersion > pm.CurrentVersion {
		return fmt.Errorf("command requires protocol version %d, but current environment is %d", reqVersion, pm.CurrentVersion)
	}
	return nil
}

// Validator defines the interface for argument validators
type Validator interface {
	Validate(args []string) error
}

// ProtocolValidator ensures arguments are valid for the active protocol
type ProtocolValidator struct {
	ProtocolVersion uint32
}

func (v *ProtocolValidator) Validate(args []string) error {
	// Implement protocol-specific validation logic here
	// e.g., check if contract addresses match the protocol format
	return nil
}
