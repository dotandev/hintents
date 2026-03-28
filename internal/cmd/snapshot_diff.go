// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/spf13/cobra"
)

var (
	snapshotDiffAFlag       string
	snapshotDiffBFlag       string
	snapshotDiffOffsetFlag  int
	snapshotDiffLengthFlag  int
	snapshotDiffContextFlag int
)

var snapshotDiffCmd = &cobra.Command{
	Use:     "snapshot-diff",
	GroupID: "utility",
	Short:   "Compare linear memory between two snapshots",
	Long: `Compare linear memory dumps from two snapshot files (Snapshot A and Snapshot B)
and display the differences in a side-by-side HEX/ASCII format.

This command is useful for debugging state-related bugs by seeing how memory
changes between two points in contract execution.

Examples:
  # Compare two snapshots
  erst snapshot-diff --snapshot-a before.json --snapshot-b after.json

  # Compare a specific memory region
  erst snapshot-diff --snapshot-a before.json --snapshot-b after.json --offset 0x100 --length 256

  # Show more context around changes
  erst snapshot-diff --snapshot-a before.json --snapshot-b after.json --context 32`,
	RunE: runSnapshotDiff,
}

func init() {
	snapshotDiffCmd.Flags().StringVar(&snapshotDiffAFlag, "snapshot-a", "", "First snapshot file (Snapshot A)")
	snapshotDiffCmd.Flags().StringVar(&snapshotDiffBFlag, "snapshot-b", "", "Second snapshot file (Snapshot B)")
	snapshotDiffCmd.Flags().IntVar(&snapshotDiffOffsetFlag, "offset", -1, "Start offset to compare (default: compare all)")
	snapshotDiffCmd.Flags().IntVar(&snapshotDiffLengthFlag, "length", 0, "Number of bytes to compare (default: all from offset)")
	snapshotDiffCmd.Flags().IntVar(&snapshotDiffContextFlag, "context", 16, "Bytes of context to show around changes")
	rootCmd.AddCommand(snapshotDiffCmd)
}

func runSnapshotDiff(cmd *cobra.Command, args []string) error {
	if snapshotDiffAFlag == "" {
		return errors.WrapCliArgumentRequired("snapshot-a")
	}
	if snapshotDiffBFlag == "" {
		return errors.WrapCliArgumentRequired("snapshot-b")
	}

	snapA, err := snapshot.Load(snapshotDiffAFlag)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to load snapshot A: %v", err))
	}

	snapB, err := snapshot.Load(snapshotDiffBFlag)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to load snapshot B: %v", err))
	}

	memA, err := snapA.DecodeLinearMemory()
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode memory from snapshot A: %v", err))
	}

	memB, err := snapB.DecodeLinearMemory()
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode memory from snapshot B: %v", err))
	}

	if len(memA) == 0 && len(memB) == 0 {
		fmt.Println("Neither snapshot contains linear memory data.")
		return nil
	}

	printDiffHeader(snapshotDiffAFlag, snapshotDiffBFlag, len(memA), len(memB))

	var diffResult *snapshot.MemoryDiffResult
	if snapshotDiffOffsetFlag >= 0 {
		length := snapshotDiffLengthFlag
		if length <= 0 {
			maxLen := len(memA)
			if len(memB) > maxLen {
				maxLen = len(memB)
			}
			length = maxLen - snapshotDiffOffsetFlag
		}
		diffResult = snapshot.DiffMemoryFull(memA, memB, snapshotDiffOffsetFlag, length)
	} else {
		diffResult = snapshot.DiffMemory(memA, memB, snapshotDiffContextFlag)
	}

	if diffResult.TotalChanged == 0 {
		fmt.Printf("\n%s Memory contents are identical\n", visualizer.Success())
		return nil
	}

	fmt.Printf("\n%s Found %d changed bytes in %d region(s)\n\n",
		visualizer.Warning(), diffResult.TotalChanged, len(diffResult.ChangedRegions))

	for i, region := range diffResult.ChangedRegions {
		printRegionDiff(i+1, &region)
	}

	printDiffSummary(diffResult)
	return nil
}

func printDiffHeader(pathA, pathB string, sizeA, sizeB int) {
	sep := strings.Repeat("─", 78)
	fmt.Println()
	fmt.Println(visualizer.Colorize("╔"+strings.Repeat("═", 78)+"╗", "cyan"))
	fmt.Printf(visualizer.Colorize("║", "cyan")+"  MEMORY SNAPSHOT COMPARISON%s"+visualizer.Colorize("║", "cyan")+"\n", strings.Repeat(" ", 50))
	fmt.Println(visualizer.Colorize("╚"+strings.Repeat("═", 78)+"╝", "cyan"))
	fmt.Println()

	fmt.Printf("  Snapshot A: %s (%d bytes)\n", pathA, sizeA)
	fmt.Printf("  Snapshot B: %s (%d bytes)\n", pathB, sizeB)
	fmt.Println(visualizer.Colorize("  "+sep, "dim"))
}

func printRegionDiff(regionNum int, region *snapshot.MemoryRegion) {
	fmt.Printf("%s Region #%d: offset 0x%08x, length %d bytes\n",
		visualizer.Colorize("──", "cyan"), regionNum, region.Offset, region.Length)
	fmt.Println()

	printDiffTable(region)
	fmt.Println()
}

func printDiffTable(region *snapshot.MemoryRegion) {
	fmt.Printf("  %-10s  %-48s  %-48s\n", "OFFSET", "SNAPSHOT A (HEX + ASCII)", "SNAPSHOT B (HEX + ASCII)")
	fmt.Printf("  %s\n", strings.Repeat("-", 110))

	for i := 0; i < region.Length; i += 16 {
		offset := region.Offset + i

		endA := i + 16
		if endA > len(region.BytesA) {
			endA = len(region.BytesA)
		}
		endB := i + 16
		if endB > len(region.BytesB) {
			endB = len(region.BytesB)
		}

		lineA := region.BytesA[i:endA]
		lineB := region.BytesB[i:endB]

		hexA, asciiA := formatHexAscii(lineA)
		hexB, asciiB := formatHexAscii(lineB)

		// Highlight differences
		hexAColored, hexBColored := colorizeHexDiff(lineA, lineB, hexA, hexB)
		asciiAColored, asciiBColored := colorizeAsciiDiff(lineA, lineB, asciiA, asciiB)

		fmt.Printf("  0x%08x  %s |%s|  %s |%s|\n",
			offset, hexAColored, asciiAColored, hexBColored, asciiBColored)
	}
}

func formatHexAscii(data []byte) (string, string) {
	hexParts := make([]string, 16)
	ascii := make([]byte, len(data))

	for i := 0; i < 16; i++ {
		if i < len(data) {
			b := data[i]
			hexParts[i] = fmt.Sprintf("%02x", b)
			if b >= 32 && b <= 126 {
				ascii[i] = b
			} else {
				ascii[i] = '.'
			}
		} else {
			hexParts[i] = "  "
		}
	}

	return strings.Join(hexParts, " "), string(ascii)
}

func colorizeHexDiff(a, b []byte, hexA, hexB string) (string, string) {
	partsA := strings.Split(hexA, " ")
	partsB := strings.Split(hexB, " ")

	for i := 0; i < 16; i++ {
		var byteA, byteB byte
		if i < len(a) {
			byteA = a[i]
		}
		if i < len(b) {
			byteB = b[i]
		}

		if i < len(a) && i < len(b) && byteA != byteB {
			partsA[i] = visualizer.Colorize(partsA[i], "red")
			partsB[i] = visualizer.Colorize(partsB[i], "green")
		} else if i >= len(a) && i < len(b) {
			partsB[i] = visualizer.Colorize(partsB[i], "green")
		} else if i < len(a) && i >= len(b) {
			partsA[i] = visualizer.Colorize(partsA[i], "red")
		}
	}

	return strings.Join(partsA, " "), strings.Join(partsB, " ")
}

func colorizeAsciiDiff(a, b []byte, asciiA, asciiB string) (string, string) {
	resultA := make([]byte, 0, len(asciiA)*10)
	resultB := make([]byte, 0, len(asciiB)*10)

	for i := 0; i < len(asciiA) || i < len(asciiB); i++ {
		var charA, charB byte = ' ', ' '
		if i < len(asciiA) {
			charA = asciiA[i]
		}
		if i < len(asciiB) {
			charB = asciiB[i]
		}

		var byteA, byteB byte
		if i < len(a) {
			byteA = a[i]
		}
		if i < len(b) {
			byteB = b[i]
		}

		if i < len(a) && i < len(b) && byteA != byteB {
			resultA = append(resultA, []byte(visualizer.Colorize(string(charA), "red"))...)
			resultB = append(resultB, []byte(visualizer.Colorize(string(charB), "green"))...)
		} else if i >= len(a) && i < len(b) {
			resultA = append(resultA, ' ')
			resultB = append(resultB, []byte(visualizer.Colorize(string(charB), "green"))...)
		} else if i < len(a) && i >= len(b) {
			resultA = append(resultA, []byte(visualizer.Colorize(string(charA), "red"))...)
			resultB = append(resultB, ' ')
		} else {
			resultA = append(resultA, charA)
			resultB = append(resultB, charB)
		}
	}

	return string(resultA), string(resultB)
}

func printDiffSummary(result *snapshot.MemoryDiffResult) {
	sep := strings.Repeat("─", 78)
	fmt.Println(visualizer.Colorize(sep, "dim"))
	fmt.Println()
	fmt.Printf("  %s Summary\n", visualizer.Colorize("──", "bold"))
	fmt.Printf("  Snapshot A size: %d bytes\n", result.SizeA)
	fmt.Printf("  Snapshot B size: %d bytes\n", result.SizeB)
	fmt.Printf("  Total changed:   %s bytes\n", visualizer.Colorize(fmt.Sprintf("%d", result.TotalChanged), "yellow"))
	fmt.Printf("  Changed regions: %d\n", len(result.ChangedRegions))
	fmt.Println()
}
