// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dotandev/hintents/internal/decoder"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABB2BF")).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EE6FF8")).
			Background(lipgloss.Color("#3E4452"))

	metricStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Bold(true).
			Underline(true)

	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

type item struct {
	node     *decoder.CallNode
	depth    int
	expanded bool
	visible  bool
	filtered bool
}

type model struct {
	root           *decoder.CallNode
	items          []*item
	cursor         int
	traceViewport  viewport.Model
	detailViewport viewport.Model
	filterInput    textinput.Model
	filtering      bool
	ready          bool
	width          int
	height         int
}

func NewTUI(root *decoder.CallNode) *model {
	ti := textinput.New()
	ti.Placeholder = "Filter events..."
	ti.CharLimit = 156
	ti.Width = 20

	m := &model{
		root:        root,
		filterInput: ti,
	}
	m.flatten(root, 0)
	if len(m.items) > 0 {
		m.items[0].expanded = true
		m.updateVisibility()
	}
	return m
}

func (m *model) flatten(n *decoder.CallNode, depth int) {
	it := &item{
		node:    n,
		depth:   depth,
		visible: true,
	}
	m.items = append(m.items, it)
	for _, child := range n.SubCalls {
		m.flatten(child, depth+1)
	}
}

func (m *model) updateVisibility() {
	if len(m.items) == 0 {
		return
	}

	query := strings.ToLower(m.filterInput.Value())

	for _, it := range m.items {
		it.filtered = query != "" && !strings.Contains(strings.ToLower(it.node.Function), query) && !strings.Contains(strings.ToLower(it.node.ContractID), query)
	}

	m.items[0].visible = true
	for i := 0; i < len(m.items); i++ {
		if !m.items[i].visible {
			continue
		}
		expanded := m.items[i].expanded
		depth := m.items[i].depth
		for j := i + 1; j < len(m.items); j++ {
			if m.items[j].depth <= depth {
				break
			}
			if !expanded {
				m.items[j].visible = false
			} else {
				if m.items[j].depth == depth+1 {
					m.items[j].visible = true
				}
			}
		}
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.filtering {
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
		m.updateVisibility()

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" || keyMsg.String() == "esc" {
				m.filtering = false
				m.filterInput.Blur()
			}
		}
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter", " ":
			if m.cursor < len(m.items) {
				m.items[m.cursor].expanded = !m.items[m.cursor].expanded
				m.updateVisibility()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.traceViewport = viewport.New(msg.Width/2-2, msg.Height-8)
			m.detailViewport = viewport.New(msg.Width/2-2, msg.Height-8)
			m.ready = true
		} else {
			m.traceViewport.Width = msg.Width/2 - 2
			m.traceViewport.Height = msg.Height - 8
			m.detailViewport.Width = msg.Width/2 - 2
			m.detailViewport.Height = msg.Height - 8
		}
	}

	m.traceViewport.SetContent(m.renderTrace())
	m.detailViewport.SetContent(m.renderDetails())

	return m, tea.Batch(cmds...)
}

func (m *model) moveCursor(dir int) {
	newCursor := m.cursor
	for {
		newCursor += dir
		if newCursor < 0 || newCursor >= len(m.items) {
			return
		}
		if m.items[newCursor].visible && !m.items[newCursor].filtered {
			m.cursor = newCursor
			return
		}
	}
}

func (m model) renderTrace() string {
	var s strings.Builder
	for i, it := range m.items {
		if !it.visible || it.filtered {
			continue
		}
		prefix := strings.Repeat("  ", it.depth)
		if len(it.node.SubCalls) > 0 {
			if it.expanded {
				prefix += "▼ "
			} else {
				prefix += "▶ "
			}
		} else {
			prefix += "• "
		}
		line := fmt.Sprintf("%s%s (%s)", prefix, it.node.Function, ShortenContractID(it.node.ContractID))
		if i == m.cursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")
	}
	return s.String()
}

func (m model) renderDetails() string {
	if m.cursor >= len(m.items) {
		return ""
	}
	n := m.items[m.cursor].node
	var s strings.Builder
	s.WriteString(headerStyle.Render("Node Details"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("Function: %s\n", n.Function))
	s.WriteString(fmt.Sprintf("Contract: %s\n", n.ContractID))
	s.WriteString(fmt.Sprintf("CPU:      %d\n", n.CPUInstructions))
	s.WriteString(fmt.Sprintf("Memory:   %s\n", FormatBytes(n.MemoryBytes)))
	s.WriteString("\n")
	s.WriteString(headerStyle.Render("Events"))
	s.WriteString("\n")
	for _, e := range n.Events {
		s.WriteString(fmt.Sprintf("- %v: %s\n", e.Topics, e.Data))
	}
	return s.String()
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	help := "↑/↓: navigate • Enter: expand/collapse • /: filter • q: quit"
	if m.filtering {
		help = "type to filter • Enter: confirm • Esc: cancel"
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Execution Trace Explorer"),
		infoStyle.Render(help),
		"",
		m.filterInput.View(),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.traceViewport.View(),
			lipgloss.NewStyle().Padding(0, 1).Render("│"),
			m.detailViewport.View(),
		),
	)

	return docStyle.Render(view)
}

func StartTUI(root *decoder.CallNode) error {
	p := tea.NewProgram(NewTUI(root), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
