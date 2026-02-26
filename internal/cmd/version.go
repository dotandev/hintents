// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

var (
	// Build information populated by ldflags
	Version   = "dev"
	CommitSHA = "unknown"
	BuildDate = "unknown"
)

type VersionInfo struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:     "version",
	GroupID: "utility",
	Short:   "Show version information",
	Long:    "Display detailed build information including version, commit hash, and build date",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		info := getVersionInfo()

		if jsonOutput {
			output, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("Erst Version: %s\n", info.Version)
			fmt.Printf("Commit SHA:   %s\n", info.CommitSHA)
			fmt.Printf("Build Date:   %s\n", info.BuildDate)
			fmt.Printf("Go Version:   %s\n", info.GoVersion)
		}
		fmt.Printf("erst version %s\n", Version)
	},
}

func getVersionInfo() VersionInfo {
	info := VersionInfo{
		Version:   Version,
		CommitSHA: CommitSHA,
		BuildDate: BuildDate,
		GoVersion: "unknown",
	}

	// Use runtime/debug as fallback
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion

		// Try to get commit and build time from build info if not set by ldflags
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.CommitSHA == "unknown" {
					info.CommitSHA = setting.Value
				}
			case "vcs.time":
				if info.BuildDate == "unknown" {
					if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
						info.BuildDate = t.Format("2006-01-02 15:04:05 UTC")
					}
				}
			}
		}
	}

	return info
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().Bool("json", false, "Output version information in JSON format")
}
