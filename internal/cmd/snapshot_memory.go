// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/spf13/cobra"
)

var (
	snapshotMemoryFileFlag   string
	snapshotMemoryOffsetFlag int
	snapshotMemoryLengthFlag int
)

var snapshotMemoryCmd = &cobra.Command{
	Use:     "snapshot-memory",
	GroupID: "utility",
	Short:   "Decode and inspect linear memory from a snapshot",
	Long:    `Decode a snapshot's base64 memory dump and print human-readable segments.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if snapshotMemoryFileFlag == "" {
			return errors.WrapCliArgumentRequired("snapshot")
		}

		snap, err := snapshot.Load(snapshotMemoryFileFlag)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to load snapshot: %v", err))
		}

		mem, err := snap.DecodeLinearMemory()
		if err != nil {
			return errors.WrapValidationError(err.Error())
		}
		if len(mem) == 0 {
			return errors.WrapValidationError("snapshot does not contain linear memory dump")
		}

		if snapshotMemoryOffsetFlag < 0 || snapshotMemoryOffsetFlag > len(mem) {
			return errors.WrapValidationError("offset out of bounds")
		}

		length := snapshotMemoryLengthFlag
		if length <= 0 {
			length = 256
		}
		end := snapshotMemoryOffsetFlag + length
		if end > len(mem) {
			end = len(mem)
		}

		fmt.Printf("Linear memory bytes: %d\n", len(mem))
		fmt.Printf("Showing range [%d:%d]\n", snapshotMemoryOffsetFlag, end)
		printMemorySegment(mem[snapshotMemoryOffsetFlag:end], snapshotMemoryOffsetFlag)
		return nil
	},
}

func printMemorySegment(data []byte, baseOffset int) {
	for i := 0; i < len(data); i += 16 {
		line := data[i:]
		if len(line) > 16 {
			line = line[:16]
		}

		hexParts := make([]string, 16)
		ascii := make([]byte, len(line))
		for j := 0; j < 16; j++ {
			if j < len(line) {
				b := line[j]
				hexParts[j] = fmt.Sprintf("%02x", b)
				if b >= 32 && b <= 126 {
					ascii[j] = b
				} else {
					ascii[j] = '.'
				}
			} else {
				hexParts[j] = "  "
			}
		}

		fmt.Printf("%08x  %s  |%s|\n", baseOffset+i, strings.Join(hexParts, " "), string(ascii))
	}
}

func init() {
	snapshotMemoryCmd.Flags().StringVar(&snapshotMemoryFileFlag, "snapshot", "", "Snapshot JSON file to inspect")
	snapshotMemoryCmd.Flags().IntVar(&snapshotMemoryOffsetFlag, "offset", 0, "Byte offset to start printing")
	snapshotMemoryCmd.Flags().IntVar(&snapshotMemoryLengthFlag, "length", 256, "Number of bytes to print")
	rootCmd.AddCommand(snapshotMemoryCmd)
}
