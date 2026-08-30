#!/bin/bash
# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0


create_issue() {
    title="$1"
    body="$2"
    echo "Creating issue: $title"
    # Removed the failing 'scf-ready' label to ensure creation succeeds
    gh issue create --title "$title" --body "$body" --label "good first issue"
}

# 2. Gosec
create_issue "[Security] Add gosec to CI" "Integrate securego/gosec into the GitHub Actions workflow. Set it to fail on High and Critical issues only for initial rollout.
Files: .github/workflows/ci.yml
Max Files To Be Changed.: 1"

# 3. Build Timeout
create_issue "[Stability] Add Build Timeout to Doctor" "Wrap the cargo build execution in FixSimulatorBinary with context.WithTimeout. If build takes >5 mins, kill it and return a Build timed out error.
Files: internal/cmd/doctor_fixers.go
Max Files To Be Changed.: 1"

# 4. Path Normalization
create_issue "[Fix] Windows Path Normalization Audit" "Systematically replace filepath.Join with logic that ensures forward slashes in config files and registry entries to avoid mixed slash errors on Windows.
Files: internal/config/config.go, internal/cmd/doctor.go
Max Files To Be Changed.: 2"

# 5. Gitignore
create_issue "[Git] Windows Build Artifacts in .gitignore" "Prevent local Windows build artifacts and debug symbols from polluting the repo.
Files: .gitignore
Max Files To Be Changed.: 1"

# 6. Functional Version
create_issue "[Doctor] Functional Version Check" "Instead of os.Stat, execute cmd := exec.Command(path, '--version'). Fail if the exit code is non-zero or if the output doesn't contain 'erst-sim'.
Files: internal/cmd/doctor.go
Max Files To Be Changed.: 1"

# 7. Color Status
create_issue "[Doctor] Colorized Fixable Status" "Modify the runDoctor loop to check dep.Fixable. If !dep.Installed && dep.Fixable, print in Yellow (ANSI 33).
Files: internal/cmd/doctor.go
Max Files To Be Changed.: 1"

# 8. All Version
create_issue "[CLI] Add --version to all Subcommands" "Use Cobra's TraverseChildren or manually add a PersistentFlag to ensure 'erst debug --version' doesn't return an 'unknown flag' error.
Files: internal/cmd/root.go
Max Files To Be Changed.: 1"

# 9. Centralize Version
create_issue "[Version] Centralize SDK Version" "Create a new package internal/version. Reference version.Version in root.go and updater.go.
Files: internal/version/version.go [NEW], internal/cmd/root.go, internal/updater/updater.go
Max Files To Be Changed.: 3"

# 10. Shorten Paths
create_issue "[UX] Shorten Registry Paths" "Create a helper FriendlyPath(path string) string that replaces the user's home directory string with ~.
Files: internal/cmd/doctor.go
Max Files To Be Changed.: 1"

# 11. Test Hint
create_issue "[Test] Unit Test for buildDeepLinkFixHint" "Add tests cases: nil steps, 1 step, 5 steps. Verify it always returns a meaningful string.
Files: internal/cmd/doctor_test.go
Max Files To Be Changed.: 1"

# 12. Perf Mean
create_issue "[CI] Jitter-Aware Performance Mean" "Modify TestPerfRegression to loop 3 times. Store ns/op. Discard highest/lowest or just take the mean. Compare mean against baseline.
Files: internal/simulator/perf_regression_test.go
Max Files To Be Changed.: 1"

# 13. Mock HSM
create_issue "[Test] Mock HSM for Integration Tests" "Provide a minimal shared library (.so/.dll) that implements the C_Initialize and C_GetSlotList PKCS#11 stubs for CI testing.
Files: simulator/src/hsm/mock.rs [NEW], internal/protocolreg/hsm_test.go
Max Files To Be Changed.: 2"

# 14. Nolint reasons
create_issue "[Lint] Standardize nolint Reasons" "Audit all 50+ //nolint comments. Append //nolint:xxx // rationale to explain why the linter is bypassed.
Files: Multiple files (Global Audit)
Max Files To Be Changed.: 10+"

# 15. Const subdirs
create_issue "[Style] Constantize Cache Subdirs" "Define const ( DirTransactions = 'transactions' ... ). Update checkCacheDir and FixMissingCacheDir to use these.
Files: internal/cmd/doctor.go, internal/cmd/doctor_fixers.go
Max Files To Be Changed.: 2"

# 16. Cleanup TODOs
create_issue "[Cleanup] Remove Legacy TODOs" "The // TODO: Use d.Runner in debug.go is obsolete. Clean up the variable shadowing and use the established runner pattern.
Files: internal/cmd/debug.go
Max Files To Be Changed.: 1"

# 17. User-Agent
create_issue "[RPC] Add User-Agent Header" "In NewClient, add a default header req.Header.Set('User-Agent', 'ERST-SDK/'+Version).
Files: internal/rpc/client.go
Max Files To Be Changed.: 1"

# 18. Validate Trace Depth
create_issue "[Config] Validate MaxTraceDepth" "Add a Validate() check to error out if the user sets max_trace_depth to 0 or >1000 in .erst.toml.
Files: internal/config/config.go
Max Files To Be Changed.: 1"

# 19. Registry Conflict
create_issue "[Logic] Detect Protocol Registry Conflicts" "In registerWindows(), run reg query first. If the (Default) value exists and doesn't contain 'erst', return a specific ErrRegistryConflict.
Files: internal/protocolreg/registration.go
Max Files To Be Changed.: 1"

# 20. Success Banner
create_issue "[UX] Success Banner for init" "After successful init, print a box with: 1. Setup RPC, 2. Build Simulator, 3. Run Doctor.
Files: internal/cmd/init.go
Max Files To Be Changed.: 1"
