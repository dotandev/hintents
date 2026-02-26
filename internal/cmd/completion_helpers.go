// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

var networkAliases = []string{"testnet\tStellar test network", "mainnet\tStellar public network", "futurenet\tStellar future network"}
var initNetworkAliases = []string{"public\tStellar public network", "testnet\tStellar test network", "futurenet\tStellar future network", "standalone\tLocal standalone network"}
var themeNames = []string{"default\tStandard terminal colors", "deuteranopia\tRed-green color blind friendly", "protanopia\tRed color blind friendly", "tritanopia\tBlue-yellow color blind friendly", "high-contrast\tHigh contrast for low-vision"}
var xdrFormats = []string{"json\tJSON output", "table\tTabular output"}
var xdrTypes = []string{"ledger-entry\tLedger entry XDR", "diagnostic-event\tDiagnostic event XDR"}
var reportFormats = []string{"html\tHTML report", "pdf\tPDF report", "json\tJSON report", "html,pdf\tBoth HTML and PDF"}

func completeNetworkFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return networkAliases, cobra.ShellCompDirectiveNoFileComp
}

func completeInitNetworkFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return initNetworkAliases, cobra.ShellCompDirectiveNoFileComp
}

func completeThemeFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return themeNames, cobra.ShellCompDirectiveNoFileComp
}

func completeXDRFormatFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return xdrFormats, cobra.ShellCompDirectiveNoFileComp
}

func completeXDRTypeFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return xdrTypes, cobra.ShellCompDirectiveNoFileComp
}

func completeReportFormatFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return reportFormats, cobra.ShellCompDirectiveNoFileComp
}

func completeNoOp(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
