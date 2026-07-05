// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

func TestDryRunCmd_NetworkValidation_ValidNetworks(t *testing.T) {
	validNetworks := []string{"testnet", "mainnet", "futurenet"}
	for _, network := range validNetworks {
		t.Run(network, func(t *testing.T) {
			prev := dryRunNetworkFlag
			t.Cleanup(func() { dryRunNetworkFlag = prev })
			dryRunNetworkFlag = network

			err := dryRunCmd.PreRunE(dryRunCmd, []string{})
			if err != nil {
				t.Errorf("expected network %q to pass validation, got error: %v", network, err)
			}
		})
	}
}

func TestDryRunCmd_NetworkValidation_InvalidNetwork(t *testing.T) {
	prev := dryRunNetworkFlag
	t.Cleanup(func() { dryRunNetworkFlag = prev })
	dryRunNetworkFlag = "invalidnet"

	err := dryRunCmd.PreRunE(dryRunCmd, []string{})
	if err == nil {
		t.Fatal("expected validation to fail for invalid network, got nil error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalidnet") {
		t.Errorf("expected error to contain the invalid value %q, got: %s", "invalidnet", errMsg)
	}
	for _, valid := range []string{"testnet", "mainnet", "futurenet"} {
		if !strings.Contains(errMsg, valid) {
			t.Errorf("expected error to list supported value %q, got: %s", valid, errMsg)
		}
	}
}

func TestDryRunCmd_NetworkValidation_EmptyNetwork(t *testing.T) {
	prev := dryRunNetworkFlag
	t.Cleanup(func() { dryRunNetworkFlag = prev })
	dryRunNetworkFlag = ""

	err := dryRunCmd.PreRunE(dryRunCmd, []string{})
	if err == nil {
		t.Fatal("expected validation to fail for empty network, got nil error")
	}
}
