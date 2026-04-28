// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/trace"
	"github.com/dotandev/hintents/internal/visualizer/style"
)

// DiffType represents the nature of a difference between two nodes
type DiffType string

const (
	DiffAdded     DiffType = "added"
	DiffRemoved   DiffType = "removed"
	DiffModified  DiffType = "modified"
	DiffUnchanged DiffType = "unchanged"
)

// StateDiff represents a difference in a single state modification
type StateDiff struct {
	Type      DiffType
	Operation string // "put", "del"
	Key       string
	BaseValue string
	HeadValue string
}

// DiffNode represents a node in the differential call tree
type DiffNode struct {
	Type        DiffType
	NodeType    string // "contract_call", "host_fn", etc.
	ContractID  string
	Function    string
	Base        *trace.TraceNode
	Head        *trace.TraceNode
	Children    []*DiffNode
	CPUDiff     int64
	MemoryDiff  int64
	StateDiffs  []StateDiff
	Divergent   bool
}

// DiffEngine implements the differential trace analysis engine
type DiffEngine struct{}

// NewDiffEngine creates a new DiffEngine
func NewDiffEngine() *DiffEngine {
	return &DiffEngine{}
}

// Compare computes the difference between two trace trees
func (e *DiffEngine) Compare(base, head *trace.TraceNode) *DiffNode {
	if base == nil && head == nil {
		return nil
	}

	if base == nil {
		return e.nodeAdded(head)
	}

	if head == nil {
		return e.nodeRemoved(base)
	}

	diff := &DiffNode{
		NodeType:   head.Type,
		ContractID: head.ContractID,
		Function:   head.Function,
		Base:       base,
		Head:       head,
		Children:   make([]*DiffNode, 0),
	}

	// Compare budget
	baseCPU := uint64(0)
	if base.CPUDelta != nil {
		baseCPU = *base.CPUDelta
	}
	headCPU := uint64(0)
	if head.CPUDelta != nil {
		headCPU = *head.CPUDelta
	}
	diff.CPUDiff = int64(headCPU) - int64(baseCPU)

	baseMem := uint64(0)
	if base.MemoryDelta != nil {
		baseMem = *base.MemoryDelta
	}
	headMem := uint64(0)
	if head.MemoryDelta != nil {
		headMem = *head.MemoryDelta
	}
	diff.MemoryDiff = int64(headMem) - int64(baseMem)

	// Compare state changes
	diff.StateDiffs = e.compareStateChanges(base.StateChanges, head.StateChanges)

	// Compare basic properties
	modified := base.Type != head.Type ||
		base.ContractID != head.ContractID ||
		base.Function != head.Function ||
		base.Error != head.Error ||
		diff.CPUDiff != 0 ||
		diff.MemoryDiff != 0 ||
		len(diff.StateDiffs) > 0

	// Compare children
	e.compareChildren(diff, base.Children, head.Children)

	// Check if any children are divergent
	anyChildDivergent := false
	for _, child := range diff.Children {
		if child.Divergent {
			anyChildDivergent = true
			break
		}
	}

	diff.Divergent = modified || anyChildDivergent
	if modified {
		diff.Type = DiffModified
	} else if anyChildDivergent {
		diff.Type = DiffUnchanged // Node itself matches, but children differ
	} else {
		diff.Type = DiffUnchanged
	}

	return diff
}

func (e *DiffEngine) nodeAdded(n *trace.TraceNode) *DiffNode {
	diff := &DiffNode{
		Type:       DiffAdded,
		NodeType:   n.Type,
		ContractID: n.ContractID,
		Function:   n.Function,
		Head:       n,
		Divergent:  true,
		Children:   make([]*DiffNode, 0),
	}

	if n.CPUDelta != nil {
		diff.CPUDiff = int64(*n.CPUDelta)
	}
	if n.MemoryDelta != nil {
		diff.MemoryDiff = int64(*n.MemoryDelta)
	}

	for _, child := range n.Children {
		diff.Children = append(diff.Children, e.nodeAdded(child))
	}

	return diff
}

func (e *DiffEngine) nodeRemoved(n *trace.TraceNode) *DiffNode {
	diff := &DiffNode{
		Type:       DiffRemoved,
		NodeType:   n.Type,
		ContractID: n.ContractID,
		Function:   n.Function,
		Base:       n,
		Divergent:  true,
		Children:   make([]*DiffNode, 0),
	}

	if n.CPUDelta != nil {
		diff.CPUDiff = -int64(*n.CPUDelta)
	}
	if n.MemoryDelta != nil {
		diff.MemoryDiff = -int64(*n.MemoryDelta)
	}

	for _, child := range n.Children {
		diff.Children = append(diff.Children, e.nodeRemoved(child))
	}

	return diff
}

func (e *DiffEngine) compareChildren(parent *DiffNode, baseChildren, headChildren []*trace.TraceNode) {
	// Simple positional matching for now.
	// In a more advanced implementation, we would use a tree matching algorithm (e.g. Zhang-Shasha).
	maxLen := len(baseChildren)
	if len(headChildren) > maxLen {
		maxLen = len(headChildren)
	}

	for i := 0; i < maxLen; i++ {
		var bc, hc *trace.TraceNode
		if i < len(baseChildren) {
			bc = baseChildren[i]
		}
		if i < len(headChildren) {
			hc = headChildren[i]
		}

		childDiff := e.Compare(bc, hc)
		if childDiff != nil {
			parent.Children = append(parent.Children, childDiff)
		}
	}
}

func (e *DiffEngine) compareStateChanges(base, head []trace.StateChange) []StateDiff {
	diffs := make([]StateDiff, 0)
	
	// Create maps for easier matching by key
	baseMap := make(map[string]trace.StateChange)
	for _, sc := range base {
		baseMap[sc.Key] = sc
	}
	headMap := make(map[string]trace.StateChange)
	for _, sc := range head {
		headMap[sc.Key] = sc
	}

	// Check for removals and modifications
	for key, bsc := range baseMap {
		if hsc, exists := headMap[key]; exists {
			if bsc.Type != hsc.Type || bsc.Value != hsc.Value {
				diffs = append(diffs, StateDiff{
					Type:      DiffModified,
					Operation: hsc.Type,
					Key:       key,
					BaseValue: bsc.Value,
					HeadValue: hsc.Value,
				})
			}
		} else {
			diffs = append(diffs, StateDiff{
				Type:      DiffRemoved,
				Operation: bsc.Type,
				Key:       key,
				BaseValue: bsc.Value,
			})
		}
	}

	// Check for additions
	for key, hsc := range headMap {
		if _, exists := baseMap[key]; !exists {
			diffs = append(diffs, StateDiff{
				Type:      DiffAdded,
				Operation: hsc.Type,
				Key:       key,
				HeadValue: hsc.Value,
			})
		}
	}

	return diffs
}

// RenderDiff produces a colorized string representation of the diff
func RenderDiff(node *DiffNode, indent int) string {
	if node == nil {
		return ""
	}

	sb := strings.Builder{}
	padding := strings.Repeat("  ", indent)

	prefix := "  "
	color := ""
	switch node.Type {
	case DiffAdded:
		prefix = "+ "
		color = "green"
	case DiffRemoved:
		prefix = "- "
		color = "red"
	case DiffModified:
		prefix = "M "
		color = "yellow"
	}

	line := fmt.Sprintf("%s%s[%s]", padding, prefix, node.NodeType)
	if node.ContractID != "" {
		cid := node.ContractID
		if len(cid) > 8 {
			cid = cid[:8]
		}
		line += fmt.Sprintf(" %s", cid)
	}
	if node.Function != "" {
		line += fmt.Sprintf("::%s", node.Function)
	}

	// Add budget diffs if significant
	budgetInfo := ""
	if node.CPUDiff != 0 {
		budgetInfo += fmt.Sprintf(" CPU:%+d", node.CPUDiff)
	}
	if node.MemoryDiff != 0 {
		budgetInfo += fmt.Sprintf(" MEM:%+d", node.MemoryDiff)
	}
	if budgetInfo != "" {
		line += " " + style.Colorize(budgetInfo, "dim")
	}

	sb.WriteString(style.Colorize(line, color) + "\n")

	// Add state diffs
	for _, sd := range node.StateDiffs {
		statePrefix := "    "
		stateColor := "dim"
		switch sd.Type {
		case DiffAdded:
			statePrefix = "  + "
			stateColor = "green"
		case DiffRemoved:
			statePrefix = "  - "
			stateColor = "red"
		case DiffModified:
			statePrefix = "  M "
			stateColor = "yellow"
		}
		
		keyDisplay := sd.Key
		if len(keyDisplay) > 8 {
			keyDisplay = keyDisplay[:8]
		}
		
		stateLine := fmt.Sprintf("%s%sSTATE: %s %s", padding, statePrefix, sd.Operation, keyDisplay)
		if sd.Type == DiffModified {
			baseVal := sd.BaseValue
			if len(baseVal) > 8 { baseVal = baseVal[:8] }
			headVal := sd.HeadValue
			if len(headVal) > 8 { headVal = headVal[:8] }
			stateLine += fmt.Sprintf(" (%s -> %s)", baseVal, headVal)
		} else if sd.Type == DiffAdded {
			headVal := sd.HeadValue
			if len(headVal) > 8 { headVal = headVal[:8] }
			stateLine += fmt.Sprintf(" (-> %s)", headVal)
		} else if sd.Type == DiffRemoved {
			baseVal := sd.BaseValue
			if len(baseVal) > 8 { baseVal = baseVal[:8] }
			stateLine += fmt.Sprintf(" (%s ->)", baseVal)
		}
		
		sb.WriteString(style.Colorize(stateLine, stateColor) + "\n")
	}

	for _, child := range node.Children {
		sb.WriteString(RenderDiff(child, indent+1))
	}

	return sb.String()
}
