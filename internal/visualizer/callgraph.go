// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

)

var elapsedPattern = regexp.MustCompile(`(?i)elapsed(?:[_\s-]*time)?(?:[_\s-]*(ms|us|ns|s))?[\s:=]+([0-9]+(?:\.[0-9]+)?)`)

// GenerateCallGraphSVG generates a premium SVG call graph from a decoder.CallNode tree
func GenerateCallGraphSVG(root *decoder.CallNode, maxDepth int) string {
	if root == nil {
		return ""
	}

	nodeWidth := 200
	nodeHeight := 96
	horizontalGap := 40
	verticalGap := 60
	legendHeight := 56

	positions := make(map[*decoder.CallNode][2]int)

	actualMaxDepth := 0
	var calculatePositions func(node *decoder.CallNode, x, y, depth int) int
	calculatePositions = func(node *decoder.CallNode, x, y, depth int) int {
		positions[node] = [2]int{x, y}
		if depth > actualMaxDepth {
			actualMaxDepth = depth
		}

		if len(node.SubCalls) == 0 || (maxDepth > 0 && depth >= maxDepth) {
			return nodeWidth
		}

		totalChildWidth := 0
		currentX := x
		for i, child := range node.SubCalls {
			childWidth := calculatePositions(child, currentX, y+nodeHeight+verticalGap, depth+1)
			totalChildWidth += childWidth
			if i < len(node.SubCalls)-1 {
				totalChildWidth += horizontalGap
				currentX += childWidth + horizontalGap
			}
		}

		positions[node] = [2]int{x + (totalChildWidth-nodeWidth)/2, y}
		return totalChildWidth
	}

	totalWidth := calculatePositions(root, 0, 0, 1)
	if totalWidth < nodeWidth {
		totalWidth = nodeWidth
	}

	totalHeight := actualMaxDepth*(nodeHeight+verticalGap) - verticalGap + 40

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg viewBox="-20 -20 %d %d" xmlns="http://www.w3.org/2000/svg" font-family="Inter, system-ui, sans-serif">`, totalWidth+40, totalHeight+40+legendHeight+16)

	sb.WriteString(`
<style>
	:root {
		--bg: #ffffff;
		--node-bg: #f6f8fa;
		--node-border: #d0d7de;
		--text-main: #24292f;
		--text-mute: #57606a;
		--link: #8c959f;
		--cpu: #0969da;
		--mem: #1a7f37;
		--gas-low-bg: #e6f4ea;
		--gas-mid-bg: #fff3cd;
		--gas-high-bg: #ffeef0;
		--gas-low-swatch: #1a7f37;
		--gas-mid-swatch: #d4a017;
		--gas-high-swatch: #da3633;
	}
	@media (prefers-color-scheme: dark) {
		:root {
			--bg: #0d1117;
			--node-bg: #161b22;
			--node-border: #30363d;
			--text-main: #c9d1d9;
			--text-mute: #8b949e;
			--link: #484f58;
			--cpu: #58a6ff;
			--mem: #3fb950;
			--gas-low-bg: #0d2818;
			--gas-mid-bg: #2d1f00;
			--gas-high-bg: #3d0c0c;
			--gas-low-swatch: #3fb950;
			--gas-mid-swatch: #e3b341;
			--gas-high-swatch: #f85149;
		}
	}
</style>
<rect width="100%" height="100%" fill="var(--bg)" />`)

	// Draw links
	for node, pos := range positions {
		for _, child := range node.SubCalls {
			childPos, ok := positions[child]
			if !ok {
				continue
			}
			x1 := pos[0] + nodeWidth/2
			y1 := pos[1] + nodeHeight
			x2 := childPos[0] + nodeWidth/2
			y2 := childPos[1]

			midY := y1 + (y2-y1)/2
			fmt.Fprintf(&sb, `<path d="M %d %d C %d %d, %d %d, %d %d" stroke="var(--link)" fill="none" stroke-width="1.5" />`,
				x1, y1, x1, midY, x2, midY, x2, y2)
		}
	}

	// Draw nodes
	for node, pos := range positions {
		x, y := pos[0], pos[1]

		contractShort := node.ContractID
		if len(contractShort) > 12 {
			contractShort = contractShort[:6] + "..." + contractShort[len(contractShort)-4:]
		}

		fmt.Fprintf(&sb, `
	<g transform="translate(%d, %d)">
		<rect width="%d" height="%d" rx="8" class="gas-%s" stroke="var(--node-border)" />
		<text x="12" y="24" class="node-title">%s</text>
		<text x="12" y="40" class="node-sub">%s</text>
		<text x="12" y="60" class="node-metric" fill="var(--cpu)">CPU: %d</text>
		<text x="100" y="60" class="node-metric" fill="var(--mem)">Mem: %s</text>
		<text x="12" y="78" class="node-metric" fill="var(--text-mute)">Elapsed: %s</text>
	</g>`,
			x,
			y,
			nodeWidth,
			nodeHeight,
			gasLevel(node.CPUInstructions),
			node.Function,
			contractShort,
			node.CPUInstructions,
			formatBytes(node.MemoryBytes),
			formatElapsedPerCall(node),
		)
	}

	legendY := totalHeight + 8
	fmt.Fprintf(&sb, `
	<g transform="translate(0, %d)">
		<rect width="320" height="%d" rx="6" fill="var(--node-bg)" stroke="var(--node-border)" />
	</g>`, legendY, legendHeight)

	sb.WriteString("</svg>")
	return sb.String()
}

func formatElapsedPerCall(node *decoder.CallNode) string {
	for _, event := range node.Events {
		if event.Data == "" {
			continue
		}
		m := elapsedPattern.FindStringSubmatch(event.Data)
		if len(m) < 3 {
			continue
		}
		unit := strings.ToLower(strings.TrimSpace(m[1]))
		valueRaw := strings.TrimSpace(m[2])
		if valueRaw == "" {
			continue
		}
		if unit == "" {
			unit = "ms"
		}
		if _, err := strconv.ParseFloat(valueRaw, 64); err != nil {
			continue
		}
		return valueRaw + unit
	}
	return "n/a"
}

func gasLevel(cpu uint64) string {
	switch {
	case cpu > 1_000_000:
		return "gas-high"
	case cpu > 100_000:
		return "gas-mid"
	default:
		return "gas-low"
	}
}

func formatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}

func formatGas(value int64, human bool) string {
	if !human || value == 0 {
		return fmt.Sprintf("%d", value)
	}

	if value >= 1_000_000 {
		v := float64(value) / 1_000_000
		return strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0") + " MGas"
	}
	if value >= 1_000 {
		v := float64(value) / 1_000
		return strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0") + " kGas"
	}
	return fmt.Sprintf("%d", value)
}

