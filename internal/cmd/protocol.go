// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dotandev/hintents/internal/protocolreg"
	"github.com/spf13/cobra"
)

var protocolRegisterCmd = &cobra.Command{
	Use:     "protocol:register",
	Short:   "Register the erst:// protocol handler in the operating system",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}
		if err := registrar.Register(); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Registered ERST protocol handler for %s://\n", protocolreg.Scheme)
		return nil
	},
}

var protocolUnregisterCmd = &cobra.Command{
	Use:     "protocol:unregister",
	Short:   "Unregister the erst:// protocol handler from the operating system",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}
		if err := registrar.Unregister(); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Unregistered ERST protocol handler")
		return nil
	},
}

var protocolStatusCmd = &cobra.Command{
	Use:     "protocol:status",
	Short:   "Check current registration status of the erst:// protocol handler",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		if registrar.IsRegistered() {
			fmt.Fprintln(cmd.OutOrStdout(), "ERST protocol handler is currently REGISTERED")
			return nil
		}

		return fmt.Errorf("ERST protocol handler is NOT REGISTERED")
	},
}

var protocolVerifyCmd = &cobra.Command{
	Use:     "protocol:verify",
	Short:   "Verify the native OS registration for the erst:// protocol handler",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		report, err := registrar.Verify()
		for _, check := range report.Checks {
			fmt.Fprintf(cmd.OutOrStdout(), "[OK] %s\n", check)
		}
		for _, issue := range report.Issues {
			fmt.Fprintf(cmd.ErrOrStderr(), "[FAIL] %s\n", issue)
		}
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Verified ERST protocol registration on %s\n", report.Platform)
		return nil
	},
}

var protocolHandlerCmd = &cobra.Command{
	Use:    "protocol-handler <uri>",
	Short:  "Internal protocol entrypoint used by the OS",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsed, err := protocolreg.ParseDebugURI(args[0])
		if err != nil {
			return err
		}

		executablePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}

		debugArgs := []string{"debug", parsed.TransactionHash, "--network", parsed.Network}
		child := exec.CommandContext(cmd.Context(), executablePath, debugArgs...)
		child.Stdout = cmd.OutOrStdout()
		child.Stderr = cmd.ErrOrStderr()
		return child.Run()
	},
}

func init() {
	rootCmd.AddCommand(protocolRegisterCmd)
	rootCmd.AddCommand(protocolUnregisterCmd)
	rootCmd.AddCommand(protocolStatusCmd)
	rootCmd.AddCommand(protocolVerifyCmd)
	rootCmd.AddCommand(protocolHandlerCmd)
}