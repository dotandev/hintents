// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package deeplink

import (
	"os/exec"
	"strings"
)

// checkRegistration queries Launch Services to find the handler for erst://.
func checkRegistration(selfPath string) Result {
	res := Result{
		FixSteps: genericFixSteps(),
	}

	// `lsregister -dump` is heavy; use the lighter `open -Ra` probe instead.
	// We ask the system which app handles the scheme by querying with a dry-run.
	out, err := exec.Command("bash", "-c",
		`/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -dump 2>/dev/null | grep -i "erst://" | head -5`,
	).Output()

	if err == nil && strings.Contains(strings.ToLower(string(out)), "erst") {
		res.Registered = true
		res.Handler = strings.TrimSpace(string(out))
		res.FixSteps = nil
		return res
	}

	// Fallback: try `open -Ra erst://` which exits 0 when a handler exists.
	if err2 := exec.Command("open", "-Ra", "erst://").Run(); err2 == nil {
		res.Registered = true
		res.Handler = "registered (via open -Ra)"
		res.FixSteps = nil
		return res
	}

	return res
}
