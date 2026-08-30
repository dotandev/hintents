// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package version

var (
	// Version is the SDK version, populated by ldflags during build
	Version = "0.0.0-dev"
	// CommitSHA is the git commit SHA, populated by ldflags during build
	CommitSHA = "unknown"
	// BuildDate is the build date, populated by ldflags during build
	BuildDate = "unknown"
)
