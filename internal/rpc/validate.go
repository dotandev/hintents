// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"
	"net/url"
)

func isValidURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsed.Scheme == "" {
		return fmt.Errorf("URL must include scheme (http:// or https://)")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	return nil
}

func ValidateNetworkConfig(config NetworkConfig) error {
	if config.Name == "" {
		return fmt.Errorf("network name is required")
	}

	if config.NetworkPassphrase == "" {
		return fmt.Errorf("network passphrase is required")
	}

	if config.HorizonURL == "" && config.SorobanRPCURL == "" {
		return fmt.Errorf("at least one of HorizonURL or SorobanRPCURL is required")
	}

	if config.HorizonURL != "" {
		if err := isValidURL(config.HorizonURL); err != nil {
			return fmt.Errorf("invalid HorizonURL: %w", err)
		}
	}

	if config.SorobanRPCURL != "" {
		if err := isValidURL(config.SorobanRPCURL); err != nil {
			return fmt.Errorf("invalid SorobanRPCURL: %w", err)
		}
	}

	if config.HorizonURL == "" && config.SorobanRPCURL == "" {
		return fmt.Errorf("at least one of HorizonURL or SorobanRPCURL must be provided")
	}

	if config.NetworkPassphrase == "" {
		return fmt.Errorf("network passphrase is required")
	}

	return nil
}
