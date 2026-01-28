package analytics

import (
	"fmt"
)

func PrintStorageReport(report *StorageGrowthReport, fee int64) {
	fmt.Println("📦 Contract Storage Growth Report")
	fmt.Println("--------------------------------")
	fmt.Printf("Before: %d bytes\n", report.BeforeBytes)
	fmt.Printf("After:  %d bytes\n", report.AfterBytes)
	fmt.Printf("Delta:  %+d bytes\n", report.DeltaBytes)
	fmt.Printf("Fee Impact: %d stroops\n\n", fee)

	fmt.Println("Per-Key Changes:")
	for key, delta := range report.PerKeyDelta {
		if delta != 0 {
			fmt.Printf("  %s: %+d bytes\n", key, delta)
		}
	}
}
