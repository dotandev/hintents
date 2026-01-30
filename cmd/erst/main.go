// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/updater"
)

var Version = "dev"

func main() {
	// Set version in cmd package
	cmd.Version = Version

	// Start update checker in background (non-blocking)
	checker := updater.NewChecker(Version)
	go checker.CheckForUpdates()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
