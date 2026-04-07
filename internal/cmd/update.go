// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dotandev/hintents/internal/updater"
	"github.com/spf13/cobra"
)

var (
	updateVersionFlag  string
	updateDetailedFlag bool
	updateYesFlag      bool
)

var updateCmd = &cobra.Command{
	Use:     "update",
	GroupID: "utility",
	Short:   "Update erst to the latest version or a specific version",
	Long: `Check for the latest version of erst and upgrade to it.
You can also specify a target version using the --version flag.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		checker := updater.NewChecker(Version)

		targetVersion := updateVersionFlag
		if targetVersion == "" {
			targetVersion = "latest"
		}

		fmt.Printf("Checking for version %s...\n", targetVersion)
		release, err := checker.FetchReleaseInfo(cmd.Context(), targetVersion)
		if err != nil {
			return fmt.Errorf("failed to fetch release information: %w", err)
		}

		fmt.Printf("Found version: %s\n\n", release.TagName)

		if release.Body != "" {
			fmt.Println("Changelog:")
			fmt.Println("----------")
			body := release.Body
			if !updateDetailedFlag && len(body) > 500 {
				body = body[:500] + "\n... (use --detailed to see full changelog)"
			}
			fmt.Println(body)
			fmt.Println("----------")
			fmt.Println()
		}

		if !updateYesFlag {
			fmt.Printf("Do you want to proceed with the update to %s? [y/N]: ", release.TagName)
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Update cancelled.")
				return nil
			}
		}

		return checker.PerformUpdate(cmd.Context(), release.TagName)
	},
}

func init() {
	updateCmd.Flags().StringVar(&updateVersionFlag, "version", "", "Target version to update to")
	updateCmd.Flags().BoolVar(&updateDetailedFlag, "detailed", false, "Show full changelog details")
	updateCmd.Flags().BoolVarP(&updateYesFlag, "yes", "y", false, "Skip confirmation prompt")

	rootCmd.AddCommand(updateCmd)
}
