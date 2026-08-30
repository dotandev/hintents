// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"fmt"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/visualizer"
)

// PrintAssetAnomalies formats and prints Move-Level asset safety anomalies.
func PrintAssetAnomalies(anomalies []simulator.AssetAnomaly) {
	if len(anomalies) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(visualizer.Colorize("=== Move-Level Asset Safety Anomalies Detected ===", "red"))
	fmt.Println(visualizer.Colorize("These mathematical violations were detected during simulation:", "yellow"))

	for i, anomaly := range anomalies {
		fmt.Printf("\n%d. [%s] in Contract %s\n", i+1, visualizer.Colorize(anomaly.AnomalyType, "red"), anomaly.ContractID)
		fmt.Printf("   Details: %s\n", anomaly.Message)
		fmt.Printf("   Amount involved: %d\n", anomaly.Amount)
	}

	fmt.Println(visualizer.Colorize("==================================================", "red"))
	fmt.Println()
}
