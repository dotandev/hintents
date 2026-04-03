// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var txHashPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type ParsedDebugURI struct {
	Raw             string
	TransactionHash string
	Network         string
	Operation       *int
	Source          string
	Signature       string
}

func ParseDebugURI(raw string) (*ParsedDebugURI, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("protocol URI must not be empty")
	}
	if !strings.HasPrefix(raw, Scheme+"://") {
		return nil, fmt.Errorf("invalid protocol URI: expected %s://", Scheme)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse protocol URI: %w", err)
	}

	if parsed.Host != "debug" {
		return nil, fmt.Errorf("invalid protocol host %q: expected debug", parsed.Host)
	}

	transactionHash := strings.TrimPrefix(parsed.EscapedPath(), "/")
	transactionHash, err = url.PathUnescape(transactionHash)
	if err != nil {
		return nil, fmt.Errorf("decode transaction hash: %w", err)
	}
	if !txHashPattern.MatchString(transactionHash) {
		return nil, fmt.Errorf("invalid transaction hash format")
	}

	network := parsed.Query().Get("network")
	if network == "" {
		return nil, fmt.Errorf("missing required query parameter: network")
	}
	if network != "testnet" && network != "mainnet" && network != "futurenet" {
		return nil, fmt.Errorf("invalid network %q", network)
	}

	result := &ParsedDebugURI{
		Raw:             raw,
		TransactionHash: transactionHash,
		Network:         network,
		Source:          parsed.Query().Get("source"),
		Signature:       parsed.Query().Get("signature"),
	}

	if operation := parsed.Query().Get("operation"); operation != "" {
		parsedOperation, err := strconv.Atoi(operation)
		if err != nil || parsedOperation < 0 {
			return nil, fmt.Errorf("invalid operation index %q", operation)
		}
		result.Operation = &parsedOperation
	}

	return result, nil
}