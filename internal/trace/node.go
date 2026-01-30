package trace

// TraceNode represents a single node in the execution trace tree
type TraceNode struct {
	ID         string       // Unique identifier for this node
	Type       string       // Type of event: "contract_call", "host_fn", "error", "event"
	ContractID string       // Contract ID if applicable
	Function   string       // Function name being called
	Error      string       // Error message if this is an error node
	EventData  string       // Event data/payload
	Depth      int          // Depth in the call tree (0 = root)
	Children   []*TraceNode // Child nodes in the execution tree
	Parent     *TraceNode   // Parent node (nil for root)
	Expanded   bool         // Whether this node is expanded in the UI
}

// NewTraceNode creates a new trace node
func NewTraceNode(id, nodeType string) *TraceNode {
	return &TraceNode{
		ID:       id,
		Type:     nodeType,
		Children: make([]*TraceNode, 0),
		Expanded: true, // Expanded by default
	}
}

// AddChild adds a child node to this node
func (n *TraceNode) AddChild(child *TraceNode) {
	child.Parent = n
	child.Depth = n.Depth + 1
	n.Children = append(n.Children, child)
}

// IsLeaf returns true if this node has no children
func (n *TraceNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// Flatten returns a flat list of all nodes in depth-first order
// Only includes visible nodes (respects Expanded state)
func (n *TraceNode) Flatten() []*TraceNode {
	result := []*TraceNode{n}

	if n.Expanded {
		for _, child := range n.Children {
			result = append(result, child.Flatten()...)
		}
	}

	return result
}

// FlattenAll returns a flat list of all nodes regardless of expansion state
func (n *TraceNode) FlattenAll() []*TraceNode {
	result := []*TraceNode{n}

	for _, child := range n.Children {
		result = append(result, child.FlattenAll()...)
	}

	return result
}

// ToggleExpanded toggles the expanded state of this node
func (n *TraceNode) ToggleExpanded() {
	n.Expanded = !n.Expanded
}

// ExpandAll expands this node and all descendants
func (n *TraceNode) ExpandAll() {
	n.Expanded = true
	for _, child := range n.Children {
		child.ExpandAll()
	}
}

// CollapseAll collapses this node and all descendants
func (n *TraceNode) CollapseAll() {
	n.Expanded = false
	for _, child := range n.Children {
		child.CollapseAll()
	}
}
