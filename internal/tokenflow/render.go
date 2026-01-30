// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tokenflow

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// SummaryLines produces human-readable summaries like:
//
//	AccountA -> 50 XLM -> AccountB
func (r *Report) SummaryLines() []string {
	var lines []string
	for _, t := range r.Agg {
		lines = append(lines, fmt.Sprintf("%s -> %s %s -> %s", t.From, formatAmount(t), t.Token.Display(), t.To))
	}
	return lines
}

// MermaidFlowchart renders a Mermaid flowchart (text) that can be pasted into Markdown.
func (r *Report) MermaidFlowchart() string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	nodeID := map[string]string{}
	next := 0
	getNode := func(label string) string {
		if id, ok := nodeID[label]; ok {
			return id
		}
		next++
		id := fmt.Sprintf("n%d", next)
		nodeID[label] = id
		b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", id, escapeMermaidLabel(label)))
		return id
	}

	for _, t := range r.Agg {
		from := getNode(t.From)
		to := getNode(t.To)
		label := fmt.Sprintf("%s %s", formatAmount(t), t.Token.Display())
		b.WriteString(fmt.Sprintf("  %s -->|\"%s\"| %s\n", from, escapeMermaidLabel(label), to))
	}

	return b.String()
}

func formatAmount(t Transfer) string {
	if t.Amount == nil {
		return "0"
	}
	if t.Token.Symbol == "XLM" && t.Token.ID == "" {
		return formatStroopsAsXLM(t.Amount)
	}
	// For SAC tokens we don't know decimals here; show raw integer.
	return t.Amount.String()
}

func formatStroopsAsXLM(stroops *big.Int) string {
	if stroops == nil {
		return "0"
	}
	neg := stroops.Sign() < 0
	n := new(big.Int).Abs(stroops)

	scale := big.NewInt(10_000_000) // 7 decimals
	intPart, frac := new(big.Int), new(big.Int)
	intPart.DivMod(n, scale, frac)

	fracStr := fmt.Sprintf("%07s", frac.String())
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		if neg {
			return "-" + intPart.String()
		}
		return intPart.String()
	}

	if neg {
		return fmt.Sprintf("-%s.%s", intPart.String(), fracStr)
	}
	return fmt.Sprintf("%s.%s", intPart.String(), fracStr)
}

var mermaidUnsafe = regexp.MustCompile(`[]"]`)

func escapeMermaidLabel(s string) string {
	return mermaidUnsafe.ReplaceAllStringFunc(s, func(m string) string {
		return "\\" + m
	})
}
