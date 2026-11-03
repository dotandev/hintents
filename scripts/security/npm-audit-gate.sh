#!/usr/bin/env bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Fails on high or critical npm advisories in packages that ship, excluding a
# reviewed list of accepted exceptions.
#
# Development dependencies are omitted: a vulnerable test runner is not reachable
# by anyone running this project, and including them turns the gate into noise
# that gets switched off.
#
# Usage:
#   scripts/security/npm-audit-gate.sh [dir...]

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
EXCEPTIONS="${REPO_ROOT}/.github/npm-audit-exceptions.txt"

if [ "$#" -gt 0 ]; then
    TARGETS=("$@")
else
    TARGETS=()
    while IFS= read -r lock; do
        case "$lock" in
            */node_modules/*) continue ;;
        esac
        TARGETS+=("$(dirname "$lock")")
    done < <(cd "$REPO_ROOT" && git ls-files '*package-lock.json')
fi

failed=0

for target in "${TARGETS[@]}"; do
    dir="${REPO_ROOT}/${target}"
    [ "$target" = "." ] && dir="$REPO_ROOT"

    report="$(cd "$dir" && npm audit --omit=dev --json 2>/dev/null)"
    if [ -z "$report" ]; then
        echo "  ${target}: no audit output"
        continue
    fi

    result="$(EXCEPTIONS_FILE="$EXCEPTIONS" TARGET="$target" python3 -c '
import json, os, sys

report = json.load(sys.stdin)
target = os.environ["TARGET"]

accepted = set()
path = os.environ["EXCEPTIONS_FILE"]
if os.path.exists(path):
    for line in open(path):
        line = line.split("#", 1)[0].strip()
        if line:
            accepted.add(line)

blocking, waived = [], []
for name, vuln in report.get("vulnerabilities", {}).items():
    if vuln.get("severity") not in ("high", "critical"):
        continue
    (waived if name in accepted else blocking).append((vuln["severity"], name))

for severity, name in sorted(waived):
    print(f"NOTE\t{target}\t{severity}\t{name}")
for severity, name in sorted(blocking):
    print(f"FAIL\t{target}\t{severity}\t{name}")
if not blocking:
    print(f"PASS\t{target}\t\t")
' <<< "$report")"

    while IFS=$'\t' read -r verdict tgt severity name; do
        case "$verdict" in
            PASS) echo "  ${tgt}: no unaccepted high or critical advisories" ;;
            NOTE) echo "  ${tgt}: ${severity} in ${name} — accepted in .github/npm-audit-exceptions.txt" ;;
            FAIL)
                echo "::error ::${tgt}: ${severity} advisory in ${name}"
                failed=1
                ;;
        esac
    done <<< "$result"
done

if [ "$failed" -ne 0 ]; then
    echo
    echo "FAILED: unaccepted high or critical advisories in shipped dependencies"
    exit 1
fi

echo
echo "PASSED: no unaccepted high or critical advisories"
