// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"html"
	"regexp"
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
	if m.searchQuery == "" {
		return
	}
	for i := range m.trace.States {
		s := &m.trace.States[i]
		if fuzzyMatchAny(m.searchQuery, s.Operation, s.ContractID, s.Function, s.Error) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
}

// fuzzyMatchAny returns true if query fuzzy-matches any of the given fields.
func fuzzyMatchAny(query string, fields ...string) bool {
	for _, f := range fields {
		if f == "" {
			continue
		}
		score, _ := FuzzyMatch(query, f, false)
		if score >= 0 {
			return true
		}
	}
	return false
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

// ---------------------------------------------------------------------------
// ANSI trace → interactive call-graph
// ---------------------------------------------------------------------------

// ansiEscapePattern matches ANSI/VT escape sequences (SGR colour codes, cursor
// movement and private modes) so they can be stripped from trace logs.
var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// stepPattern matches the "[N]" step marker emitted for every state in a
// printed execution trace.
var stepPattern = regexp.MustCompile(`\[(\d+)\]`)

// cpuPattern and memPattern match the "CPU: 1,234" / "MEM: 1.5 KB" budget
// sub-lines emitted for nodes that carry resource deltas.
var (
	cpuPattern = regexp.MustCompile(`(?i)cpu:\s*([0-9,]+)`)
	memPattern = regexp.MustCompile(`(?i)mem:\s*([0-9.,]+\s*(?:b|kb|mb|gb)?)`)
)

// contractIDPattern matches Stellar contract/account identifiers, which are
// 56-character base32 strings starting with C or G.
var contractIDPattern = regexp.MustCompile(`\b[CG][A-Z2-7]{55}\b`)

// knownIcons lists the glyphs used to tag operation types in printed traces.
var knownIcons = []string{"◆", "⚙", "🔐", "◉", "▪", "▸", "■", "⚠", "…", "·", "▾", "[FAIL]", "[READY]"}

// decorationPrefixes are header/footer lines that carry no trace steps.
var decorationPrefixes = []string{
	"transaction execution trace",
	"hash  :",
	"start :",
	"steps :",
	"steps:",
}

// CallGraphNode is a single function-call node in an interactive call graph
// parsed from text-based trace logs.
type CallGraphNode struct {
	ID          string
	Operation   string
	EventType   string
	ContractID  string
	Function    string
	Error       string
	CPUDelta    uint64
	MemoryDelta uint64
	Depth       int
	Calls       []*CallGraphNode
	parent      *CallGraphNode
}

// AddCall appends a callee node to this node, wiring its Depth and parent.
func (n *CallGraphNode) AddCall(child *CallGraphNode) {
	child.parent = n
	child.Depth = n.Depth + 1
	n.Calls = append(n.Calls, child)
}

// IsLeaf reports whether the node has no callees.
func (n *CallGraphNode) IsLeaf() bool {
	return len(n.Calls) == 0
}

// Label returns a human-readable label for the node: Function (Contract) when
// both are known, otherwise the operation name.
func (n *CallGraphNode) Label() string {
	if n.Function != "" {
		if n.ContractID != "" {
			return n.Function + " (" + shortID(n.ContractID, 66) + ")"
		}
		return n.Function
	}
	if n.Operation != "" {
		return n.Operation
	}
	return n.ID
}

// CallGraphEdge is a directed call edge (caller → callee) in a CallGraph.
type CallGraphEdge struct {
	Caller string
	Callee string
}

// CallGraph is an interactive call-graph structure parsed from ANSI trace
// output. Root is the transaction entry point; Edges lists every caller/callee
// relationship so the graph can be rendered by tools or in a browser.
type CallGraph struct {
	Root  *CallGraphNode
	Edges []CallGraphEdge
}

// NewCallGraph creates an empty call graph rooted at a transaction node.
func NewCallGraph() *CallGraph {
	return &CallGraph{
		Root:  &CallGraphNode{ID: "root", Operation: "transaction"},
		Edges: make([]CallGraphEdge, 0),
	}
}

// Nodes returns every node in the graph in depth-first order.
func (g *CallGraph) Nodes() []*CallGraphNode {
	if g == nil || g.Root == nil {
		return nil
	}
	nodes := make([]*CallGraphNode, 0)
	g.Root.walk(&nodes)
	return nodes
}

func (n *CallGraphNode) walk(acc *[]*CallGraphNode) {
	*acc = append(*acc, n)
	for _, call := range n.Calls {
		call.walk(acc)
	}
}

// Find returns all nodes whose label contains the given query substring,
// case-insensitively.
func (g *CallGraph) Find(query string) []*CallGraphNode {
	if g == nil || query == "" {
		return nil
	}
	q := strings.ToLower(query)
	matches := make([]*CallGraphNode, 0)
	for _, node := range g.Nodes() {
		if strings.Contains(strings.ToLower(node.Label()), q) {
			matches = append(matches, node)
		}
	}
	return matches
}

// ParseANSITrace converts ANSI-styled trace output into an interactive call
// graph. The input may be a plain-text trace (as produced by
// PrintExecutionTrace with --no-color) or the same trace including ANSI colour
// codes, which are stripped before parsing. Each "[N] ..." step line becomes a
// call-graph node; cross-contract calls reuse the same nesting heuristic as
// BuildTraceTree.
func ParseANSITrace(input string) (*CallGraph, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("trace input is empty")
	}

	graph := NewCallGraph()
	stack := []*CallGraphNode{graph.Root}
	var lastNode *CallGraphNode

	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(stripANSIEscapes(rawLine))
		if line == "" || isTraceDecoration(line) {
			continue
		}

		switch {
		case strings.Contains(line, "[FAIL]"):
			if lastNode != nil && lastNode != graph.Root {
				lastNode.Error = extractErrorMessage(line)
			}
		case strings.Contains(strings.ToLower(line), "cpu:") || strings.Contains(strings.ToLower(line), "mem:"):
			if lastNode != nil {
				attachBudget(lastNode, line)
			}
		case stepPattern.MatchString(line):
			node := parseTraceStepLine(line)
			if node == nil {
				continue
			}
			lastNode = graph.insertNode(&stack, node)
		default:
			// Free-form log/event text attaches to the current frame so no
			// trace content is lost, but never overwrites the operation.
			if lastNode != nil && lastNode != graph.Root && lastNode.Operation == "" {
				lastNode.Operation = line
			}
		}
	}

	graph.buildEdges()
	return graph, nil
}

// insertNode places a parsed step node into the call graph using the same
// cross-contract nesting heuristic as BuildTraceTree: a call into a different
// contract nests under the previous contract call, while a call back into the
// same contract is treated as a sibling.
func (g *CallGraph) insertNode(stack *[]*CallGraphNode, node *CallGraphNode) *CallGraphNode {
	top := (*stack)[len(*stack)-1]

	if node.EventType == EventTypeContractCall {
		switch {
		case top != g.Root && top.ContractID != "" && node.ContractID != "" && node.ContractID != top.ContractID:
			top.AddCall(node)
		case top != g.Root && top.parent != nil:
			top.parent.AddCall(node)
		default:
			g.Root.AddCall(node)
		}
		*stack = append(*stack, node)
	} else {
		top.AddCall(node)
	}
	return node
}

// buildEdges derives the directed caller/callee edge list from the tree.
func (g *CallGraph) buildEdges() {
	g.Edges = make([]CallGraphEdge, 0)
	var walk func(n *CallGraphNode)
	walk = func(n *CallGraphNode) {
		for _, call := range n.Calls {
			g.Edges = append(g.Edges, CallGraphEdge{Caller: n.Label(), Callee: call.Label()})
			walk(call)
		}
	}
	walk(g.Root)
}

// parseTraceStepLine extracts a call-graph node from a single "[N] ..." trace
// line, returning nil when the line carries no recognisable trace data.
func parseTraceStepLine(line string) *CallGraphNode {
	marker := stepPattern.FindStringSubmatch(line)
	if marker == nil {
		return nil
	}
	step, err := strconv.Atoi(marker[1])
	if err != nil {
		step = -1
	}

	node := &CallGraphNode{ID: fmt.Sprintf("step-%d", step)}
	rest := strings.TrimSpace(stepPattern.ReplaceAllString(line, " "))
	rest = strings.TrimSpace(stripLeadingIcon(rest))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return node
	}

	node.Operation = strings.ToUpper(fields[0])
	node.EventType = eventTypeForOperation(node.Operation)

	if id := contractIDPattern.FindString(rest); id != "" {
		node.ContractID = id
	}

	switch node.EventType {
	case EventTypeContractCall, EventTypeHostFunction, EventTypeAuth:
		for _, token := range fields[1:] {
			if strings.HasPrefix(token, "→") || contractIDPattern.MatchString(token) {
				continue
			}
			node.Function = token
			break
		}
	}
	return node
}

// eventTypeForOperation maps an uppercased operation label to its event type.
func eventTypeForOperation(op string) string {
	switch op {
	case "CONTRACT_CALL", "CONTRACT_INIT", "BALANCE_CHECK":
		return EventTypeContractCall
	case "HOST_FUNCTION", "HOST_FN":
		return EventTypeHostFunction
	case "AUTH":
		return EventTypeAuth
	case "EVENT":
		return "event"
	case "LOG":
		return "log"
	case "TRAP":
		return EventTypeTrap
	case "TRANSACTION":
		return "transaction"
	default:
		return ""
	}
}

// stripLeadingIcon removes the operation-type glyph and tree connectors at the
// start of a trace label, returning the remaining text.
func stripLeadingIcon(s string) string {
	fields := strings.Fields(s)
	for i, token := range fields {
		if isIcon(token) || !containsAlphaNumeric(token) {
			continue
		}
		return strings.Join(fields[i:], " ")
	}
	return ""
}

// isIcon reports whether token is a known operation-type glyph.
func isIcon(token string) bool {
	for _, icon := range knownIcons {
		if token == icon {
			return true
		}
	}
	return false
}

// containsAlphaNumeric reports whether s contains at least one letter or digit.
func containsAlphaNumeric(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// attachBudget parses "CPU:" / "MEM:" budget metrics into the node.
func attachBudget(node *CallGraphNode, line string) {
	if m := cpuPattern.FindStringSubmatch(line); len(m) == 2 {
		node.CPUDelta = parseBudgetNumber(m[1])
	}
	if m := memPattern.FindStringSubmatch(line); len(m) == 2 {
		node.MemoryDelta = parseMemoryBytes(m[1])
	}
}

// parseBudgetNumber parses a comma-separated integer like "1,234,567".
func parseBudgetNumber(s string) uint64 {
	v, err := strconv.ParseUint(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseMemoryBytes parses human-readable byte strings like "1.5 KB", "42 B".
func parseMemoryBytes(s string) uint64 {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(s)))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	unit := "B"
	if len(fields) > 1 {
		unit = fields[1]
	}
	switch unit {
	case "KB":
		return uint64(value * 1024)
	case "MB":
		return uint64(value * 1024 * 1024)
	case "GB":
		return uint64(value * 1024 * 1024 * 1024)
	default:
		return uint64(value)
	}
}

// extractErrorMessage returns the text after the "[FAIL]" marker on a line.
func extractErrorMessage(line string) string {
	if idx := strings.Index(line, "[FAIL]"); idx >= 0 {
		return strings.TrimSpace(line[idx+len("[FAIL]"):])
	}
	return strings.TrimSpace(line)
}

// stripANSIEscapes removes ANSI/VT escape sequences from a line.
func stripANSIEscapes(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

// isTraceDecoration reports whether a line is a trace header, footer or
// separator that carries no step data.
func isTraceDecoration(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range decorationPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if strings.Trim(line, "─-═") == "" {
		return true
	}
	return false
}

// RenderInteractiveHTML renders the call graph as a self-contained, interactive
// HTML document. Nodes are grouped in collapsible <details> elements,
// colour-coded by event type, with CPU/memory deltas and errors shown inline.
func (g *CallGraph) RenderInteractiveHTML() string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	sb.WriteString("<title>Erst Interactive Call Graph</title>\n")
	sb.WriteString("<style>")
	sb.WriteString("body{font-family:ui-monospace,Menlo,Consolas,monospace;margin:24px;color:#222}")
	sb.WriteString("details{margin-left:18px}.op{color:#333;font-weight:600}.err{color:#d33}.delta{color:#2a9d4f}")
	sb.WriteString("</style>\n</head>\n<body>\n")
	sb.WriteString("<h1>Erst Interactive Call Graph</h1>\n")
	if g == nil || g.Root == nil {
		sb.WriteString("<p>No trace data.</p>\n")
	} else {
		sb.WriteString(renderCallGraphNode(g.Root))
		sb.WriteString("\n<h2>Call edges</h2>\n<ul>\n")
		for _, edge := range g.Edges {
			fmt.Fprintf(&sb, "<li>%s &rarr; %s</li>\n", html.EscapeString(edge.Caller), html.EscapeString(edge.Callee))
		}
		sb.WriteString("</ul>\n")
	}
	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

// renderCallGraphNode renders a single node and its descendants as nested,
// collapsible HTML.
func renderCallGraphNode(n *CallGraphNode) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	if len(n.Calls) > 0 {
		sb.WriteString("<details open>\n<summary>")
	} else {
		sb.WriteString("<div>")
	}
	fmt.Fprintf(&sb, "<span class=\"op\">%s</span>", html.EscapeString(n.Label()))
	if n.EventType != "" {
		fmt.Fprintf(&sb, " <span>[%s]</span>", html.EscapeString(n.EventType))
	}
	if n.CPUDelta > 0 {
		fmt.Fprintf(&sb, " <span class=\"delta\">CPU %s</span>", formatNum(n.CPUDelta))
	}
	if n.MemoryDelta > 0 {
		fmt.Fprintf(&sb, " <span class=\"delta\">MEM %s</span>", formatBytes(n.MemoryDelta))
	}
	if n.Error != "" {
		fmt.Fprintf(&sb, " <span class=\"err\">%s</span>", html.EscapeString(n.Error))
	}
	if len(n.Calls) > 0 {
		sb.WriteString("</summary>\n")
	} else {
		sb.WriteString("</div>\n")
	}
	for _, call := range n.Calls {
		sb.WriteString(renderCallGraphNode(call))
	}
	if len(n.Calls) > 0 {
		sb.WriteString("</details>\n")
	}
	return sb.String()
}
