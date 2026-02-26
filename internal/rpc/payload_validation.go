// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
)

// MaxPayloadSize is the maximum allowed size for JSON payloads (10 MB)
const MaxPayloadSize = 10 * 1024 * 1024

// WarningThreshold is the size at which a warning is issued (8 MB)
// to alert users before they hit the maximum limit.
const WarningThreshold = 8 * 1024 * 1024

// ValidatePayloadSize checks if the given payload size is within the allowed limit.
// Returns an error if the payload exceeds the maximum size, preventing
// the CLI from attempting to submit oversized payloads to the network.
// Also logs a warning when the payload size exceeds the warning threshold.
func ValidatePayloadSize(payloadSize int64) error {
	// Log a warning if payload is approaching the limit
	if payloadSize > WarningThreshold {
		logger.Logger.Warn(
			"Payload size approaching limit",
			"currentSize", formatBytes(payloadSize),
			"warningThreshold", formatBytes(WarningThreshold),
			"maxSize", formatBytes(MaxPayloadSize),
			"remaining", formatBytes(MaxPayloadSize-payloadSize),
		)
	}

	if payloadSize > MaxPayloadSize {
		return errors.WrapRPCRequestTooLarge(payloadSize, MaxPayloadSize)
	}
	return nil
}

// formatBytes converts bytes to a human-readable string (e.g., "1.5 MB")
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	n := bytes
	for n >= unit {
		n /= unit
		exp++
	}
	suffixes := []string{"B", "KB", "MB", "GB", "TB"}
	if exp >= len(suffixes) {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffixes[exp])
}
