// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/spf13/cobra"
)

var exportSnapshotFlag string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data from the current session",
	Long:  `Export debugging data, such as state snapshots, from the currently active session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if exportSnapshotFlag == "" {
			return fmt.Errorf("must specify --snapshot <file>")
		}

		// Get current session
		data := GetCurrentSession()
		if data == nil {
			return fmt.Errorf("no active session. Run 'erst debug <tx-hash>' first")
		}

		// Unwrap simulation request to get ledger entries
		var simReq simulator.SimulationRequest
		if err := json.Unmarshal([]byte(data.SimRequestJSON), &simReq); err != nil {
			return fmt.Errorf("failed to parse session data: %w", err)
		}

		if len(simReq.LedgerEntries) == 0 {
			fmt.Println("Warning: No ledger entries found in the current session.")
		}

		// Convert to snapshot
		snap := snapshot.FromMap(simReq.LedgerEntries)

		// Save
		if err := snapshot.Save(exportSnapshotFlag, snap); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}

		fmt.Printf("Snapshot exported to %s (%d entries)\n", exportSnapshotFlag, len(snap.LedgerEntries))
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportSnapshotFlag, "snapshot", "", "Output file for JSON snapshot")
	rootCmd.AddCommand(exportCmd)
}
