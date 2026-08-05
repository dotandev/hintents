// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
)

// GenerateFlamegraph dynamically calculates canvas height based on max stack depth.
func GenerateFlamegraph(maxStackDepth int) string {
	// 20 pixels per frame plus 50 for labels/padding
	canvasHeight := maxStackDepth*20 + 50
	return fmt.Sprintf("<svg height=\"%d\"></svg>", canvasHeight)
}
