// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/analyzer"
)

// Profiler formats GasInefficiency data for the CLI or markdown export
type Profiler struct{}

func NewProfiler() *Profiler {
	return &Profiler{}
}

// FormatGasReport generates a formatted report of gas inefficiencies
func (p *Profiler) FormatGasReport(inefficiencies []analyzer.GasInefficiency) string {
	if len(inefficiencies) == 0 {
		return "No gas inefficiencies detected. Your footprints look optimal!\n"
	}

	var sb strings.Builder
	sb.WriteString("=== Gas Optimization Profiler Report ===\n\n")

	for i, ineff := range inefficiencies {
		sb.WriteString(fmt.Sprintf("[%d] %s (%s severity)\n", i+1, ineff.Type, ineff.Severity))
		sb.WriteString(fmt.Sprintf("    Location: %s\n", ineff.Location))
		sb.WriteString(fmt.Sprintf("    Details:  %s\n", ineff.Description))
		sb.WriteString(fmt.Sprintf("    Action:   %s\n\n", ineff.Recommendation))
	}

	return sb.String()
}
