// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package deeplink

import (
	"os/exec"
	"strings"
)

// checkRegistration queries the Windows registry for the erst:// URL handler.
func checkRegistration(selfPath string) Result {
	res := Result{
		FixSteps: genericFixSteps(),
	}

	// Query HKEY_CLASSES_ROOT\erst to see if the key exists.
	out, err := exec.Command(
		"reg", "query", `HKEY_CLASSES_ROOT\erst`, "/ve",
	).Output()

	if err != nil {
		return res
	}

	value := strings.ToLower(string(out))
	if strings.Contains(value, "url:erst") || strings.Contains(value, "url protocol") {
		res.Registered = true
		res.Handler = strings.TrimSpace(string(out))
		res.FixSteps = nil
	}

	return res
}
