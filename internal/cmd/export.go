// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/spf13/cobra"
)

var exportSnapshotFlag string
var exportIncludeMemoryFlag bool

var decodeSnapshotFlag string
var decodeOffsetFlag int
var decodeLengthFlag int

var exportCmd = &cobra.Command{
	Use:     "export",
	GroupID: "utility",
	Short:   "Export data from the current session",
	Long:    `Export debugging data, such as state snapshots, from the currently active session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if exportSnapshotFlag == "" {
			return errors.WrapCliArgumentRequired("snapshot")
		}

		// Get current session
		data := GetCurrentSession()
		if data == nil {
			return errors.WrapSimulationLogicError("no active session. Run 'erst debug <tx-hash>' first")
		}

		// Unwrap simulation request to get ledger entries
		var simReq simulator.SimulationRequest
		if err := json.Unmarshal([]byte(data.SimRequestJSON), &simReq); err != nil {
			return errors.WrapUnmarshalFailed(err, "session data")
		}

		if len(simReq.LedgerEntries) == 0 {
			fmt.Println("Warning: No ledger entries found in the current session.")
		}

		var encodedMemory string
		if exportIncludeMemoryFlag {
			memoryBase64, err := extractLinearMemoryBase64(data.SimResponseJSON)
			if err != nil {
				return errors.WrapUnmarshalFailed(err, "simulation response")
			}
			if memoryBase64 == "" {
				fmt.Println("Warning: Simulator response does not include a linear memory dump.")
			} else {
				if _, err := base64.StdEncoding.DecodeString(memoryBase64); err != nil {
					return errors.WrapValidationError(fmt.Sprintf("failed to decode simulator linear memory dump: %v", err))
				}
				encodedMemory = memoryBase64
			}
		}

		snap := snapshot.FromMap(simReq.LedgerEntries)
		if encodedMemory != "" {
			snap.LinearMemory = encodedMemory
		}

		// Save
		if err := snapshot.Save(exportSnapshotFlag, snap); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to save snapshot: %v", err))
		}

		fmt.Printf("Snapshot exported to %s (%d entries)\n", exportSnapshotFlag, len(snap.LedgerEntries))
		if snap.LinearMemory != "" {
			decodedBytes := base64.StdEncoding.DecodedLen(len(snap.LinearMemory)) - strings.Count(snap.LinearMemory, "=")
			fmt.Printf("Included linear memory dump: %d bytes\n", decodedBytes)
		}
		return nil
	},
}

var exportDecodeMemoryCmd = &cobra.Command{
	Use:   "decode-memory",
	Short: "Decode and print a linear memory dump from a snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if decodeSnapshotFlag == "" {
			return errors.WrapCliArgumentRequired("snapshot")
		}

		snap, err := snapshot.Load(decodeSnapshotFlag)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to load snapshot: %v", err))
		}

		memory, err := snap.DecodeLinearMemory()
		if err != nil {
			return errors.WrapValidationError(err.Error())
		}
		if len(memory) == 0 {
			fmt.Println("No linear memory dump found in snapshot.")
			return nil
		}

		if decodeOffsetFlag < 0 {
			return errors.WrapValidationError("offset must be >= 0")
		}
		if decodeLengthFlag <= 0 {
			return errors.WrapValidationError("length must be > 0")
		}
		if decodeOffsetFlag >= len(memory) {
			return errors.WrapValidationError(fmt.Sprintf("offset %d out of bounds for memory size %d", decodeOffsetFlag, len(memory)))
		}

		end := decodeOffsetFlag + decodeLengthFlag
		if end > len(memory) {
			end = len(memory)
		}

		segment := memory[decodeOffsetFlag:end]
		fmt.Printf("Linear memory segment [%d:%d] (%d bytes)\n", decodeOffsetFlag, end, len(segment))
		for i := 0; i < len(segment); i += 16 {
			lineEnd := i + 16
			if lineEnd > len(segment) {
				lineEnd = len(segment)
			}
			line := segment[i:lineEnd]
			fmt.Printf("0x%08x  ", decodeOffsetFlag+i)
			for _, b := range line {
				fmt.Printf("%02x ", b)
			}
			for j := len(line); j < 16; j++ {
				fmt.Print("   ")
			}
			fmt.Print(" |")
			for _, b := range line {
				if b >= 32 && b <= 126 {
					fmt.Printf("%c", b)
				} else {
					fmt.Print(".")
				}
			}
			fmt.Println("|")
		}

		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportSnapshotFlag, "snapshot", "", "Output file for JSON snapshot")
	exportCmd.Flags().BoolVar(&exportIncludeMemoryFlag, "include-memory", false, "Include Wasm linear memory dump from simulation response when available")

	exportDecodeMemoryCmd.Flags().StringVar(&decodeSnapshotFlag, "snapshot", "", "Snapshot file that contains linear memory")
	exportDecodeMemoryCmd.Flags().IntVar(&decodeOffsetFlag, "offset", 0, "Start offset in bytes")
	exportDecodeMemoryCmd.Flags().IntVar(&decodeLengthFlag, "length", 256, "Number of bytes to print")

	exportCmd.AddCommand(exportDecodeMemoryCmd)
	rootCmd.AddCommand(exportCmd)
}

func extractLinearMemoryBase64(simResponseJSON string) (string, error) {
	if simResponseJSON == "" {
		return "", nil
	}

	var payload struct {
		LinearMemoryDump   string `json:"linear_memory_dump"`
		LinearMemoryBase64 string `json:"linear_memory_base64"`
		LinearMemory       string `json:"linear_memory"`
	}

	if err := json.Unmarshal([]byte(simResponseJSON), &payload); err != nil {
		return "", err
	}

	if payload.LinearMemoryDump != "" {
		return payload.LinearMemoryDump, nil
	}

	if payload.LinearMemoryBase64 != "" {
		return payload.LinearMemoryBase64, nil
	}

	return payload.LinearMemory, nil
}
