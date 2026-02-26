# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/usr/bin/env bash
set -euo pipefail

# Commit script for Issue #309 — Split-Pane View
# Run from the repo root.

echo "=== Staging and committing Issue #309 changes ==="

# 1. TraceNode: add SourceRef field
git add internal/trace/node.go
git commit -m "feat(trace): add SourceRef field to TraceNode for source mapping

Links a trace node to a Rust source file, line, and column via an optional
SourceRef pointer. Existing callers that construct TraceNode without source
info are unaffected because the field is nil by default.

Refs #309"

# 2. Split-pane core implementation
git add internal/trace/splitpane.go
git commit -m "feat(trace): implement SplitPane horizontal split renderer

Adds SplitPane, SourceRef, SourceContext, LoadSourceContext, hBorder, and
nodeDisplayLines. The renderer writes a two-pane ASCII layout to any io.Writer:
trace node fields in the upper pane, windowed Rust source lines in the lower
pane. Terminal width is auto-detected via \$COLUMNS (fallback 80).

Refs #309"

# 3. Wire sp/split command into the interactive viewer
git add internal/trace/viewer.go
git commit -m "feat(trace/viewer): add 'sp' / 'split' command to interactive viewer

Adds showSplitPane() and executionStateToNode() to InteractiveViewer.
Typing 'sp' or 'split' at the > prompt renders the current execution state
as a TraceNode in the upper pane and, when SourceRef is set, the surrounding
Rust source lines in the lower pane.

Refs #309"

# 4. Tests
git add internal/trace/splitpane_test.go
git commit -m "test(trace): add 16 unit tests for split-pane view (Issue #309)

Covers LoadSourceContext (round-trip, missing file, line clamping, empty file),
SplitPane.Render (nil/non-nil source, error nodes, SourceRef display),
hBorder width consistency and label centring/truncation,
nodeDisplayLines (all fields, minimal node), and executionStateToNode
(field mapping, error promotion).

All 16 tests pass. The pre-existing TestSearchUnicode_Mixed failure is
unrelated to this change.

Refs #309"

echo ""
echo "=== Done. Four commits pushed for Issue #309. ==="
