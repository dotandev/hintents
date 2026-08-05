// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// UI modes
// ---------------------------------------------------------------------------

// viewMode controls which panel has keyboard focus.
type viewMode int

const (
	modeTrace  viewMode = iota // navigating the trace step list (left pane)
	modeSearch                 // typing a search query
	modeHelp                   // help overlay active
)

// ---------------------------------------------------------------------------
// lipgloss styles
// ---------------------------------------------------------------------------

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	stylePaneTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("229"))

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	styleSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))

	styleSearch = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)

	styleMatch = lipgloss.NewStyle().
			Background(lipgloss.Color("58")).
			Foreground(lipgloss.Color("229"))

	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)

	styleFilterTag = lipgloss.NewStyle().
			Foreground(lipgloss.Color("213")).
			Bold(true)
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the bubbletea Model for the interactive trace viewer.
// It holds all rendering and navigation state. The underlying ExecutionTrace
// is treated as the single source of truth for step data; this struct only
// tracks UI concerns on top of it.
type Model struct {
	// Core data
	trace *ExecutionTrace

	// Terminal dimensions
	width  int
	height int

	// Navigation
	mode        viewMode
	eventFilter string   // one of the EventType* constants, or ""
	filterCycle []string // rotation order for 'f'
	hideStdLib  bool     // hide core::* frames when true
	forked      bool
	forkStep    int

	// Search
	searchQuery   string
	searchMatches []int  // step indices that match the query
	searchIdx     int    // current position within searchMatches
	searchInput   string // characters typed so far while modeSearch

	// Async state panel
	panelState  *ExecutionState // reconstructed state for current step
	panelErr    string
	panelLoaded bool

	// Notification / status message shown in the status bar for one frame
	statusMsg string

	// navHistory for undo
	navHistory *NavigatorHistory
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewModel creates a ready-to-run bubbletea Model wrapping the given trace.
func NewModel(t *ExecutionTrace) Model {
	return Model{
		trace: t,
		mode:  modeTrace,
		filterCycle: []string{
			"",
			EventTypeTrap,
			EventTypeContractCall,
			EventTypeHostFunction,
			EventTypeAuth,
		},
		searchIdx:  -1,
		navHistory: NewNavigatorHistory(),
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

// Init satisfies tea.Model. We kick off no commands at startup.
func (m Model) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------------------
// Async messages
// ---------------------------------------------------------------------------

// panelStateMsg carries the result of an async state reconstruction.
type panelStateMsg struct {
	step  int
	state *ExecutionState
	err   error
}

// reconstructCmd launches an async ReconstructStateAt and delivers the result
// as a panelStateMsg.
func reconstructCmd(t *ExecutionTrace, step int) tea.Cmd {
	return func() tea.Msg {
		state, err := t.ReconstructStateAt(step)
		return panelStateMsg{step: step, state: state, err: err}
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update satisfies tea.Model. It routes all tea.Msg values to the appropriate
// handler and returns the updated model plus any follow-up commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case panelStateMsg:
		if msg.step == m.trace.CurrentStep {
			if msg.err != nil {
				m.panelErr = msg.err.Error()
			} else {
				m.panelState = msg.state
				m.panelLoaded = true
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey dispatches key presses to mode-specific handlers.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	default:
		return m.handleTraceKey(msg)
	}
}

// ---------------------------------------------------------------------------
// Key handler – normal trace mode
// ---------------------------------------------------------------------------

func (m Model) handleTraceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	// Quit
	case "q", "Q", "ctrl+c":
		return m, tea.Quit

	// Step forward
	case "n", "right":
		return m.stepForward()

	// Step backward
	case "p", "b", "left":
		return m.stepBackward()

	// Jump to first step
	case "0", "home":
		return m.jumpTo(0)

	// Jump to last step
	case "$", "G", "end":
		return m.jumpTo(len(m.trace.States) - 1)

	// Undo last navigation
	case "u", "ctrl+z":
		return m.undoNavigation()

	// Cycle event filter
	case "f":
		return m.cycleFilter()

	// Toggle stdlib visibility
	case "S":
		m.hideStdLib = !m.hideStdLib
		if m.hideStdLib {
			m.statusMsg = "core::* traces hidden"
		} else {
			m.statusMsg = "core::* traces shown"
		}
		return m, nil

	// Enter search mode
	case "/":
		m.mode = modeSearch
		m.searchInput = ""
		m.statusMsg = ""
		return m, nil

	// Next / previous search match
	case "N":
		return m.prevMatch()
	// lowercase n already mapped to stepForward; allow ctrl+n for next match
	case "ctrl+n":
		return m.nextMatch()

	// Help overlay
	case "?", "h":
		m.mode = modeHelp
		return m, nil
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Key handler – search mode
// ---------------------------------------------------------------------------

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Commit query and jump to first match.
		m.searchQuery = m.searchInput
		m.mode = modeTrace
		m.buildSearchMatches()
		if len(m.searchMatches) > 0 {
			m.searchIdx = 0
			return m.jumpTo(m.searchMatches[0])
		}
		m.statusMsg = fmt.Sprintf("No matches for %q", m.searchQuery)
		return m, nil

	case "esc":
		m.mode = modeTrace
		m.searchInput = ""
		m.searchQuery = ""
		m.searchMatches = nil
		m.searchIdx = -1
		return m, nil

	case "backspace", "ctrl+h":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		return m, nil

	default:
		// Append printable characters.
		if len(msg.Runes) > 0 {
			m.searchInput += string(msg.Runes)
		}
		return m, nil
	}
}

// ---------------------------------------------------------------------------
// Key handler – help overlay
// ---------------------------------------------------------------------------

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?", "h":
		m.mode = modeTrace
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------------------

func (m Model) jumpTo(step int) (Model, tea.Cmd) {
	if len(m.trace.States) == 0 {
		m.statusMsg = "no states in trace"
		return m, nil
	}
	if step < 0 {
		step = 0
	}
	if step >= len(m.trace.States) {
		step = len(m.trace.States) - 1
	}
	m.navHistory.Push(m.trace.CurrentStep)
	_, err := m.trace.JumpToStep(step)
	if err != nil {
		m.statusMsg = styleError.Render(err.Error())
		return m, nil
	}
	m.resetPanel()
	return m, reconstructCmd(m.trace, step)
}

func (m Model) stepForward() (Model, tea.Cmd) {
	next := m.trace.CurrentStep + 1
	for next < len(m.trace.States) {
		if m.eventFilter != "" && !m.trace.StepMatchesFilter(next, m.eventFilter) {
			next++
			continue
		}
		if m.hideStdLib && strings.HasPrefix(m.trace.States[next].Function, "core::") {
			next++
			continue
		}
		break
	}
	return m.jumpTo(next)
}

func (m Model) stepBackward() (Model, tea.Cmd) {
	prev := m.trace.CurrentStep - 1
	for prev >= 0 {
		if m.eventFilter != "" && !m.trace.StepMatchesFilter(prev, m.eventFilter) {
			prev--
			continue
		}
		if m.hideStdLib && strings.HasPrefix(m.trace.States[prev].Function, "core::") {
			prev--
			continue
		}
		break
	}
	return m.jumpTo(prev)
}

func (m Model) undoNavigation() (Model, tea.Cmd) {
	idx, ok := m.navHistory.Pop()
	if !ok {
		m.statusMsg = "nothing to undo"
		return m, nil
	}
	_, err := m.trace.JumpToStep(idx)
	if err != nil {
		m.statusMsg = styleError.Render(err.Error())
		return m, nil
	}
	m.resetPanel()
	return m, reconstructCmd(m.trace, idx)
}

func (m Model) cycleFilter() (Model, tea.Cmd) {
	for i, f := range m.filterCycle {
		if f == m.eventFilter {
			m.eventFilter = m.filterCycle[(i+1)%len(m.filterCycle)]
			break
		}
	}
	if m.eventFilter == "" {
		m.statusMsg = "filter: off"
	} else {
		count := m.trace.FilteredStepCount(m.eventFilter)
		m.statusMsg = styleFilterTag.Render(fmt.Sprintf("filter: %s (%d steps)", m.eventFilter, count))
	}
	return m, nil
}

func (m *Model) resetPanel() {
	m.panelState = nil
	m.panelErr = ""
	m.panelLoaded = false
}

// ---------------------------------------------------------------------------
// Search helpers
// ---------------------------------------------------------------------------

func (m *Model) buildSearchMatches() {
	m.searchMatches = nil
	q := strings.ToLower(m.searchQuery)
	if q == "" {
		return
	}
	for i := range m.trace.States {
		s := &m.trace.States[i]
		if strings.Contains(strings.ToLower(s.Operation), q) ||
			strings.Contains(strings.ToLower(s.ContractID), q) ||
			strings.Contains(strings.ToLower(s.Function), q) ||
			strings.Contains(strings.ToLower(s.Error), q) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
}

func (m Model) nextMatch() (Model, tea.Cmd) {
	if len(m.searchMatches) == 0 {
		m.statusMsg = "no search active"
		return m, nil
	}
	m.searchIdx = (m.searchIdx + 1) % len(m.searchMatches)
	return m.jumpTo(m.searchMatches[m.searchIdx])
}

func (m Model) prevMatch() (Model, tea.Cmd) {
	if len(m.searchMatches) == 0 {
		m.statusMsg = "no search active"
		return m, nil
	}
	m.searchIdx--
	if m.searchIdx < 0 {
		m.searchIdx = len(m.searchMatches) - 1
	}
	return m.jumpTo(m.searchMatches[m.searchIdx])
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View satisfies tea.Model and returns the full terminal frame as a string.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading...\n"
	}

	if m.mode == modeHelp {
		return m.viewHelp()
	}

	// Layout: header (1 line) + body (height-3 lines) + status bar (1 line)
	// + search bar (1 line, only in modeSearch).
	bodyHeight := m.height - 2
	if m.mode == modeSearch {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	header := m.viewHeader()
	body := m.viewBody(bodyHeight)
	status := m.viewStatusBar()

	parts := []string{header, body, status}
	if m.mode == modeSearch {
		parts = append(parts, m.viewSearchBar())
	}

	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

func (m Model) viewHeader() string {
	tx := m.trace.TransactionHash
	if len(tx) > 20 {
		tx = tx[:8] + "…" + tx[len(tx)-8:]
	}
	total := len(m.trace.States)
	cur := m.trace.CurrentStep

	title := styleHeader.Render("ERST Trace Viewer")
	info := styleDim.Render(fmt.Sprintf("tx:%s  step:%d/%d", tx, cur, max(total-1, 0)))
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(info) - 2
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + info
}

// ---------------------------------------------------------------------------
// Body (two-pane split)
// ---------------------------------------------------------------------------

func (m Model) viewBody(height int) string {
	leftW := m.width / 2
	rightW := m.width - leftW - 1 // -1 for divider

	left := m.viewTracePane(leftW, height)
	right := m.viewDetailPane(rightW, height)

	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	var sb strings.Builder
	for i := 0; i < height; i++ {
		lCell := lineAt(leftLines, i, leftW)
		rCell := lineAt(rightLines, i, rightW)
		sb.WriteString(lCell)
		sb.WriteString(styleDim.Render("│"))
		sb.WriteString(rCell)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ---------------------------------------------------------------------------
// Left pane – step list
// ---------------------------------------------------------------------------

func (m Model) viewTracePane(width, height int) string {
	title := stylePaneTitle.Render("Trace Steps")
	lines := []string{title}

	cur := m.trace.CurrentStep
	total := len(m.trace.States)

	// Compute window of visible steps centred on the current one.
	rowsAvail := height - 1 // reserve one for title
	if rowsAvail < 1 {
		rowsAvail = 1
	}
	start := cur - rowsAvail/2
	if start < 0 {
		start = 0
	}
	end := start + rowsAvail
	if end > total {
		end = total
		start = end - rowsAvail
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		state := &m.trace.States[i]

		if m.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			lines = append(lines, strings.Repeat(" ", width))
			continue
		}

		marker := "  "
		if i == cur {
			marker = "▶ "
		}

		op := state.Operation
		fn := state.Function
		errSuffix := ""
		if state.Error != "" {
			errSuffix = " " + styleError.Render("[FAIL]")
		}

		matchSuffix := ""
		if m.isSearchMatch(i) {
			matchSuffix = " " + styleMatch.Render("●")
		}

		label := fmt.Sprintf("%s%3d: %s", marker, i, op)
		if fn != "" {
			label += fmt.Sprintf(" (%s)", fn)
		}
		label += errSuffix + matchSuffix

		var line string
		if i == cur {
			line = styleSelected.Render(padRight(label, width))
		} else {
			line = padRight(label, width)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) isSearchMatch(step int) bool {
	for _, idx := range m.searchMatches {
		if idx == step {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Right pane – detail panel
// ---------------------------------------------------------------------------

func (m Model) viewDetailPane(width, height int) string {
	title := stylePaneTitle.Render("Step Detail")
	lines := []string{title}

	if len(m.trace.States) == 0 {
		lines = append(lines, styleDim.Render("(no trace data)"))
		return padPane(lines, width, height)
	}

	state, err := m.trace.GetCurrentState()
	if err != nil {
		lines = append(lines, styleError.Render(err.Error()))
		return padPane(lines, width, height)
	}

	add := func(key, val string) {
		row := styleDim.Render(padRight(key+":", 14)) + " " + val
		lines = append(lines, wrapLineAt(row, width))
	}

	add("Step", strconv.Itoa(state.Step)+"/"+strconv.Itoa(max(len(m.trace.States)-1, 0)))
	add("Time", state.Timestamp.Format("15:04:05.000"))
	add("Operation", state.Operation)
	if state.ContractID != "" {
		add("Contract", state.ContractID)
	}
	if state.Function != "" {
		add("Function", state.Function)
	}
	if len(state.Arguments) > 0 {
		add("Args", fmt.Sprintf("%v", state.Arguments))
	}
	if state.ReturnValue != nil {
		add("Return", fmt.Sprintf("%v", state.ReturnValue))
	}
	if state.WasmInstruction != "" {
		add("WASM", state.WasmInstruction)
	}
	if state.Error != "" {
		lines = append(lines, styleError.Render(wrapLineAt("Error: "+state.Error, width)))
	}

	// Async reconstructed state panel.
	lines = append(lines, styleDim.Render(strings.Repeat("─", width)))
	switch {
	case m.panelLoaded && m.panelState != nil:
		if len(m.panelState.HostState) > 0 {
			add("HostState", fmt.Sprintf("%d entries", len(m.panelState.HostState)))
		}
		if len(m.panelState.Memory) > 0 {
			add("Memory", fmt.Sprintf("%d entries", len(m.panelState.Memory)))
		}
	case m.panelErr != "":
		lines = append(lines, styleError.Render("Fetch err: "+m.panelErr))
	default:
		lines = append(lines, styleDim.Render("[ reconstructing state… ]"))
	}

	return padPane(lines, width, height)
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

func (m Model) viewStatusBar() string {
	left := "[n]ext [p]rev [f]ilter [/]search [?]help [q]quit"
	if m.eventFilter != "" {
		left = styleFilterTag.Render("filter:"+m.eventFilter) + "  " + left
	}
	if m.statusMsg != "" {
		left = m.statusMsg + "  " + left
		m.statusMsg = "" //nolint:staticcheck // field cleared next frame
	}

	matchInfo := ""
	if len(m.searchMatches) > 0 {
		matchInfo = styleSearch.Render(
			fmt.Sprintf("  match %d/%d", m.searchIdx+1, len(m.searchMatches)),
		)
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(matchInfo)
	if gap < 0 {
		gap = 0
	}
	return styleStatusBar.Render(left + strings.Repeat(" ", gap) + matchInfo)
}

// ---------------------------------------------------------------------------
// Search bar
// ---------------------------------------------------------------------------

func (m Model) viewSearchBar() string {
	prompt := styleSearch.Render("/") + m.searchInput + "█"
	return styleBorder.Render(prompt)
}

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func (m Model) viewHelp() string {
	lines := []string{
		styleHeader.Render("ERST Trace Viewer – Keyboard Shortcuts"),
		"",
		stylePaneTitle.Render("Navigation"),
		"  n / →        Step forward",
		"  p / b / ←    Step backward",
		"  0 / Home     Jump to first step",
		"  $ / G / End  Jump to last step",
		"  u / Ctrl+Z   Undo last jump",
		"",
		stylePaneTitle.Render("Filters & Display"),
		"  f             Cycle event filter (trap / contract_call / host_function / auth)",
		"  S             Toggle core::* stdlib visibility",
		"",
		stylePaneTitle.Render("Search"),
		"  /             Enter search mode",
		"  Ctrl+N        Next match",
		"  N             Previous match",
		"  Esc           Clear search / exit search mode",
		"",
		stylePaneTitle.Render("Other"),
		"  ? / h         Toggle this help overlay",
		"  q / Ctrl+C    Quit",
		"",
		styleDim.Render("Press Esc, q, or ? to close"),
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Rendering utilities
// ---------------------------------------------------------------------------

// lineAt returns line i from a slice, padded or clipped to exactly width runes.
func lineAt(lines []string, i, width int) string {
	s := ""
	if i < len(lines) {
		s = lines[i]
	}
	return padRight(s, width)
}

// padRight pads s with spaces to at least width, then clips to width.
func padRight(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis < width {
		return s + strings.Repeat(" ", width-vis)
	}
	// Clip: walk runes until we hit the budget.
	runes := []rune(s)
	total := 0
	for i, r := range runes {
		rw := lipgloss.Width(string(r))
		if total+rw > width {
			return string(runes[:i])
		}
		total += rw
	}
	return s
}

// wrapLineAt clips a line to width, appending "…" if truncated.
func wrapLineAt(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	total := 0
	for i, r := range runes {
		total += lipgloss.Width(string(r))
		if total >= width-1 {
			return string(runes[:i]) + "…"
		}
	}
	return s
}

// padPane pads lines to height, each to width, and joins them with newlines.
func padPane(lines []string, width, height int) string {
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	padded := make([]string, len(lines))
	for i, l := range lines {
		padded[i] = padRight(l, width)
	}
	return strings.Join(padded, "\n")
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
