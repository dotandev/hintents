// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dotandev/hintents/internal/visualizer"
)

const (
	defaultTermWidth = 80
	defaultTraceRows = 12
	defaultSrcRows   = 12
	defaultRadius    = 5
)

// SourceRef links a trace node to a position in Rust source code.
type SourceRef struct {
	File     string
	Line     int // 1-based; 0 = unknown
	Column   int // 1-based; 0 = unknown
	Function string
}

// SourceContext holds a windowed slice of source lines for the lower pane.
type SourceContext struct {
	Ref        SourceRef
	Lines      []string
	FocusIndex int // 0-based index into Lines for the focal line
}

// LoadSourceContext opens the file named by ref.File and returns a window of
// ±radius lines centred on ref.Line. Returns an error when the file is
// unreadable.
func LoadSourceContext(ref SourceRef, radius int) (*SourceContext, error) {
	if radius <= 0 {
		radius = defaultRadius
	}
	f, err := os.Open(ref.File)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", ref.File, err)
	}
	defer f.Close()

	var all []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		all = append(all, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", ref.File, err)
	}

	total := len(all)
	if total == 0 {
		return &SourceContext{Ref: ref}, nil
	}

	focus := ref.Line - 1 // convert to 0-based
	if focus < 0 {
		focus = 0
	}
	if focus >= total {
		focus = total - 1
	}

	start := max(0, focus-radius)
	end := min(total-1, focus+radius)

	return &SourceContext{
		Ref:        ref,
		Lines:      all[start : end+1],
		FocusIndex: focus - start,
	}, nil
}

// SplitPane renders a horizontal two-pane split: execution trace node above,
// mapped Rust source code below.
type SplitPane struct {
	// Width is the terminal column count. 0 = auto-detect via $COLUMNS then 80.
	Width int
	// TraceRows is the number of content rows in the trace pane.
	TraceRows int
	// SrcRows is the number of content rows in the source pane.
	SrcRows int
}

// DefaultSplitPane returns a SplitPane with sensible defaults.
func DefaultSplitPane() *SplitPane {
	return &SplitPane{
		TraceRows: defaultTraceRows,
		SrcRows:   defaultSrcRows,
	}
}

// Render writes the split view to w. node must not be nil.
// src may be nil when no source mapping is available for the node.
func (p *SplitPane) Render(w io.Writer, node *TraceNode, src *SourceContext) {
	width := p.resolveWidth()
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	p.renderTracePane(bw, node, width)
	p.renderDivider(bw, width, src)
	p.renderSourcePane(bw, src, width)
}

func (p *SplitPane) resolveWidth() int {
	if p.Width > 0 {
		return p.Width
	}
	if w := envColumns(); w > 0 {
		return w
	}
	return defaultTermWidth
}

func envColumns() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		var n int
		if _, err := fmt.Sscanf(c, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func paneRows(configured, dflt int) int {
	if configured > 0 {
		return configured
	}
	return dflt
}

func (p *SplitPane) renderTracePane(w io.Writer, node *TraceNode, width int) {
	fmt.Fprintln(w, hBorder(" Trace Node ", width))
	limit := paneRows(p.TraceRows, defaultTraceRows)
	lines := nodeDisplayLines(node)
	for i, line := range lines {
		if i >= limit {
			fmt.Fprintf(w, "  %s\n", visualizer.Colorize(fmt.Sprintf("... (%d more)", len(lines)-limit), "dim"))
			break
		}
		fmt.Fprintf(w, "  %s\n", line)
	}
	for i := len(lines); i < limit; i++ {
		fmt.Fprintln(w)
	}
}

func (p *SplitPane) renderDivider(w io.Writer, width int, src *SourceContext) {
	label := " Source "
	if src != nil && src.Ref.File != "" {
		raw := fmt.Sprintf(" %s:%d ", src.Ref.File, src.Ref.Line)
		if len(raw) > width-4 {
			keep := width - 7
			if keep < 1 {
				keep = 1
			}
			raw = " ..." + raw[len(raw)-keep:]
		}
		label = raw
	}
	fmt.Fprintln(w, hBorder(label, width))
}

func (p *SplitPane) renderSourcePane(w io.Writer, src *SourceContext, width int) {
	limit := paneRows(p.SrcRows, defaultSrcRows)
	if src == nil || len(src.Lines) == 0 {
		fmt.Fprintf(w, "  %s\n", visualizer.Colorize("No source mapping available for this node.", "dim"))
		for i := 1; i < limit; i++ {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, hBorder("", width))
		return
	}
	startLine := src.Ref.Line - src.FocusIndex
	if startLine < 1 {
		startLine = 1
	}
	for i, line := range src.Lines {
		if i >= limit {
			fmt.Fprintf(w, "  %s\n", visualizer.Colorize(fmt.Sprintf("... (%d more)", len(src.Lines)-limit), "dim"))
			break
		}
		lineNum := startLine + i
		numStr := fmt.Sprintf("%4d", lineNum)
		if i == src.FocusIndex {
			fmt.Fprintf(w, "%s | %s\n", visualizer.Colorize(numStr, "yellow"), visualizer.Colorize(line, "bold"))
		} else {
			fmt.Fprintf(w, "%s | %s\n", visualizer.Colorize(numStr, "dim"), line)
		}
	}
	for i := len(src.Lines); i < limit; i++ {
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, hBorder("", width))
}

// nodeDisplayLines converts a TraceNode into display strings for the trace pane.
func nodeDisplayLines(node *TraceNode) []string {
	var out []string
	out = append(out, fmt.Sprintf("Type:     %s", visualizer.Colorize(strings.ToUpper(node.Type), "bold")))
	if node.ContractID != "" {
		out = append(out, fmt.Sprintf("Contract: %s", visualizer.Colorize(node.ContractID, "cyan")))
	}
	if node.Function != "" {
		out = append(out, fmt.Sprintf("Function: %s", visualizer.Colorize(node.Function, "yellow")))
	}
	if node.EventData != "" {
		out = append(out, fmt.Sprintf("Data:     %s", node.EventData))
	}
	if node.Error != "" {
		out = append(out, fmt.Sprintf("Error:    %s", visualizer.Colorize(node.Error, "red")))
	}
	indent := strings.Repeat("  ", node.Depth)
	out = append(out, fmt.Sprintf("Depth:    %s%d", indent, node.Depth))
	if node.SourceRef != nil {
		loc := fmt.Sprintf("%s:%d", node.SourceRef.File, node.SourceRef.Line)
		if node.SourceRef.Column > 0 {
			loc = fmt.Sprintf("%s:%d:%d", node.SourceRef.File, node.SourceRef.Line, node.SourceRef.Column)
		}
		out = append(out, fmt.Sprintf("Source:   %s", visualizer.Colorize(loc, "dim")))
	}
	return out
}

// hBorder draws a full-width horizontal border with the label centred.
func hBorder(label string, width int) string {
	const (
		corner = "+"
		fill   = "-"
	)
	inner := width - 2
	if inner <= 0 {
		return corner + corner
	}
	if label == "" {
		return corner + strings.Repeat(fill, inner) + corner
	}
	pad := inner - len(label)
	if pad < 0 {
		return corner + label[:inner] + corner
	}
	left := pad / 2
	right := pad - left
	return corner + strings.Repeat(fill, left) + label + strings.Repeat(fill, right) + corner
}
