// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/decoder"
)

// GenerateCallGraphSVG generates a premium SVG call graph from a decoder.CallNode tree
func GenerateCallGraphSVG(root *decoder.CallNode) string {
	if root == nil {
		return ""
	}

	// Layout and dimensions
	nodeWidth := 200
	nodeHeight := 80
	horizontalGap := 40
	verticalGap := 60

	// Track total dimensions and compute positions
	positions := make(map[*decoder.CallNode][2]int) // node -> [x, y]
	
	// First pass: calculate tree width and positions
	var calculatePositions func(node *decoder.CallNode, x, y int) int
	calculatePositions = func(node *decoder.CallNode, x, y int) int {
		positions[node] = [2]int{x, y}
		
		if len(node.SubCalls) == 0 {
			return nodeWidth
		}

		totalChildWidth := 0
		currentX := x
		for i, child := range node.SubCalls {
			childWidth := calculatePositions(child, currentX, y+nodeHeight+verticalGap)
			totalChildWidth += childWidth
			if i < len(node.SubCalls)-1 {
				totalChildWidth += horizontalGap
				currentX += childWidth + horizontalGap
			}
		}

		// Center parent over children
		positions[node] = [2]int{x + (totalChildWidth-nodeWidth)/2, y}
		return totalChildWidth
	}

	totalWidth := calculatePositions(root, 0, 0)
	if totalWidth < nodeWidth {
		totalWidth = nodeWidth
	}
	
	// Find max depth for height
	maxDepth := 0
	var findDepth func(node *decoder.CallNode, depth int)
	findDepth = func(node *decoder.CallNode, depth int) {
		if depth > maxDepth {
			maxDepth = depth
		}
		for _, child := range node.SubCalls {
			findDepth(child, depth+1)
		}
	}
	findDepth(root, 1)
	totalHeight := maxDepth*(nodeHeight+verticalGap) - verticalGap + 40 // + padding

	// Build SVG
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg viewBox="-20 -20 %d %d" xmlns="http://www.w3.org/2000/svg" font-family="Inter, system-ui, sans-serif">`, totalWidth+40, totalHeight+40))
	
	// CSS for styling and dark mode
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
		}
	}
	rect { transition: fill 0.2s; }
	rect:hover { fill: var(--bg); stroke-width: 2px; }
	.node-title { font-weight: 600; font-size: 14px; fill: var(--text-main); }
	.node-sub { font-size: 11px; fill: var(--text-mute); }
	.node-metric { font-size: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
</style>
<rect width="100%" height="100%" fill="var(--bg)" />`)

	// Second pass: Draw links
	for node, pos := range positions {
		for _, child := range node.SubCalls {
			childPos := positions[child]
			x1 := pos[0] + nodeWidth/2
			y1 := pos[1] + nodeHeight
			x2 := childPos[0] + nodeWidth/2
			y2 := childPos[1]
			
			// Cubic bezier for smooth curves
			midY := y1 + (y2-y1)/2
			sb.WriteString(fmt.Sprintf(`<path d="M %d %d C %d %d, %d %d, %d %d" stroke="var(--link)" fill="none" stroke-width="1.5" />`,
				x1, y1, x1, midY, x2, midY, x2, y2))
		}
	}

	// Third pass: Draw nodes
	for node, pos := range positions {
		x, y := pos[0], pos[1]
		
		contractShort := node.ContractID
		if len(contractShort) > 12 {
			contractShort = contractShort[:6] + "..." + contractShort[len(contractShort)-4:]
		}

		sb.WriteString(fmt.Sprintf(`
	<g transform="translate(%d, %d)">
		<rect width="%d" height="%d" rx="8" fill="var(--node-bg)" stroke="var(--node-border)" />
		<text x="12" y="24" class="node-title">%s</text>
		<text x="12" y="40" class="node-sub">%s</text>
		<text x="12" y="60" class="node-metric" fill="var(--cpu)">CPU: %d</text>
		<text x="100" y="60" class="node-metric" fill="var(--mem)">Mem: %s</text>
	</g>`, x, y, nodeWidth, nodeHeight, node.Function, contractShort, node.CPUInstructions, formatBytes(node.MemoryBytes)))
	}

	sb.WriteString("</svg>")
	return sb.String()
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
