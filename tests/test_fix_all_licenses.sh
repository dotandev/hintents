# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/usr/bin/env bash
# tests/test_fix_all_licenses.sh
# Unit tests for fix_all_licenses.sh
#
# What this file does:
#   Creates small temporary git repos, puts files with or without license
#   headers inside them, runs fix_all_licenses.sh against them, and checks
#   the result is what we expect.
#
# Run locally with:
#   bash tests/test_fix_all_licenses.sh
#
# This file is also called automatically by the CI workflow
# .github/workflows/license-audit.yml on every push and pull request.

set -euo pipefail

# ---------------------------------------------------------------------------
# SETUP
# Find fix_all_licenses.sh relative to this test file, no matter where
# the test is run from.
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${SCRIPT_DIR}/fix_all_licenses.sh"

# Counters for final summary
PASS=0
FAIL=0

# ---------------------------------------------------------------------------
# HELPER FUNCTIONS
# These are small utilities used by every test below.
# ---------------------------------------------------------------------------

# Create a fresh throwaway directory for one test
tmp_dir() { mktemp -d; }

# Run fix_all_licenses.sh from inside a given directory, passing any extra args
run_audit() {
  local dir="$1"; shift
  (cd "$dir" && bash "$SCRIPT" "$@")
}

# Set up a minimal git repo so git ls-files works inside the test directory.
# fix_all_licenses.sh uses git ls-files to find files, so every test needs this.
make_git_repo() {
  local dir="$1"
  git -C "$dir" init -q
  git -C "$dir" config user.email "test@example.com"
  git -C "$dir" config user.name "Test"
}

# Stage a file so git ls-files can see it
add_tracked() {
  local dir="$1" file="$2"
  git -C "$dir" add "$file"
}

# Assert: the command exits 0 (success). Prints PASS or FAIL.
assert_exit_zero() {
  local desc="$1"; shift
  if "$@" > /dev/null 2>&1; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected exit 0, got non-zero)"
    FAIL=$((FAIL + 1))
  fi
}

# Assert: the command exits non-zero (failure). Prints PASS or FAIL.
assert_exit_nonzero() {
  local desc="$1"; shift
  if ! "$@" > /dev/null 2>&1; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected non-zero exit, got 0)"
    FAIL=$((FAIL + 1))
  fi
}

# Assert: a file contains a specific string. Prints PASS or FAIL.
assert_file_contains() {
  local desc="$1" file="$2" pattern="$3"
  if grep -qF "$pattern" "$file"; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected to find '$pattern' in $file)"
    FAIL=$((FAIL + 1))
  fi
}

# ---------------------------------------------------------------------------
# TEST 1
# A Go file that already has the correct license header should pass --check.
# --check is what CI runs. If this fails, the CI would be broken for clean files.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf '// Copyright (c) Hintents Authors.\n// SPDX-License-Identifier: Apache-2.0\n\npackage main\n' > "$T/main.go"
add_tracked "$T" main.go
assert_exit_zero "Go file with correct header passes --check" run_audit "$T" --check
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 2
# A Go file with NO license header should cause --check to exit non-zero.
# This is the core CI gate: new files without headers must be caught.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'package main\n' > "$T/nohdr.go"
add_tracked "$T" nohdr.go
assert_exit_nonzero "Go file missing header fails --check" run_audit "$T" --check
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 3
# Running without --check (fix mode) should prepend the header to a Go file.
# Developers run this locally to fix missing headers before pushing.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'package main\n' > "$T/fixme.go"
add_tracked "$T" fixme.go
run_audit "$T" > /dev/null 2>&1 || true
assert_file_contains "Fix mode prepends header to Go file" "$T/fixme.go" "// Copyright (c) Hintents Authors."
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 4
# After fix mode has run, --check should now pass on the same file.
# This confirms the fix and the check are consistent with each other.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'package main\n' > "$T/fixme.go"
add_tracked "$T" fixme.go
run_audit "$T" > /dev/null 2>&1 || true
assert_exit_zero "After fix mode, --check passes on same file" run_audit "$T" --check
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 5
# A Rust file with NO license header should cause --check to fail.
# The project has Rust code in simulator/ so Rust files must be covered.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'fn main() {}\n' > "$T/lib.rs"
add_tracked "$T" lib.rs
assert_exit_nonzero "Rust file missing header fails --check" run_audit "$T" --check
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 6
# Fix mode should prepend the header to a Rust file.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'fn main() {}\n' > "$T/lib.rs"
add_tracked "$T" lib.rs
run_audit "$T" > /dev/null 2>&1 || true
assert_file_contains "Fix mode prepends header to Rust file" "$T/lib.rs" "// Copyright (c) Hintents Authors."
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 7
# A TypeScript file with NO license header should cause --check to fail.
# The project has TypeScript in the audit extension (audit.test.ts etc.).
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf 'export const x = 1;\n' > "$T/index.ts"
add_tracked "$T" index.ts
assert_exit_nonzero "TypeScript file missing header fails --check" run_audit "$T" --check
rm -rf "$T"

# ---------------------------------------------------------------------------
# TEST 8
# Fix mode must NOT modify a file that already has the correct header.
# Without this guarantee, fix mode would keep prepending on every run.
# We check this by comparing the md5 checksum before and after.
# ---------------------------------------------------------------------------
T=$(tmp_dir); make_git_repo "$T"
printf '// Copyright (c) Hintents Authors.\n// SPDX-License-Identifier: Apache-2.0\n\npackage main\n' > "$T/clean.go"
add_tracked "$T" clean.go
BEFORE=$(md5sum "$T/clean.go")
run_audit "$T" > /dev/null 2>&1 || true
AFTER=$(md5sum "$T/clean.go")
if [[ "$BEFORE" == "$AFTER" ]]; then
  echo "PASS: Fix mode does not modify already-correct files"
  PASS=$((PASS + 1))
else
  echo "FAIL: Fix mode modified a file that already had the correct header"
  FAIL=$((FAIL + 1))
fi
rm -rf "$T"

# ---------------------------------------------------------------------------
# SUMMARY
# Print final results. Exit 1 if any test failed so CI catches it.
# ---------------------------------------------------------------------------
echo ""
echo "Results: $PASS passed, $FAIL failed."
[[ $FAIL -eq 0 ]]