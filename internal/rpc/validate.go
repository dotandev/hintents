// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"
	"net/url"

	"github.com/dotandev/hintents/internal/errors"
)

func isValidURL(urlStr string) error {
	if urlStr == "" {
		return errors.WrapValidationError("URL cannot be empty")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid URL format: %v", err))
	}

	if parsed.Scheme == "" {
		return errors.WrapValidationError("URL must include scheme (http:// or https://)")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.WrapValidationError(fmt.Sprintf("URL scheme must be http or https, got %q", parsed.Scheme))
	}

	if parsed.Host == "" {
		return errors.WrapValidationError("URL must include a host")
	}

	return nil
}

func ValidateNetworkConfig(config NetworkConfig) error {
	if config.Name == "" {
		return errors.WrapValidationError("network name is required")
	}

	if config.NetworkPassphrase == "" {
		return errors.WrapValidationError("network passphrase is required")
	}

	if config.HorizonURL == "" && config.SorobanRPCURL == "" {
		return errors.WrapValidationError("at least one of HorizonURL or SorobanRPCURL is required")
	}

	if config.HorizonURL != "" {
		if err := isValidURL(config.HorizonURL); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid HorizonURL: %v", err))
		}
	}

	if config.SorobanRPCURL != "" {
		if err := isValidURL(config.SorobanRPCURL); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid SorobanRPCURL: %v", err))
		}
	}

	if config.HorizonURL == "" && config.SorobanRPCURL == "" {
		return errors.WrapValidationError("at least one of HorizonURL or SorobanRPCURL must be provided")
	}

	if config.NetworkPassphrase == "" {
		return errors.WrapValidationError("network passphrase is required")
	}

	return nil
}
