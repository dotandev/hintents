#!/usr/bin/env bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Inspects dependency manifest changes in a pull request.
#
# A new dependency is the cheapest way to get code into a project: nobody reads
# a transitive package, and `go mod tidy` makes the diff look routine. This
# check makes every manifest change say out loud what it pulled in, and blocks
# the handful of constructs that let a dependency run code or come from
# somewhere other than the official registry.
#
# Usage:
#   scripts/security/check-dependency-changes.sh [base-ref] [head-ref]

set -euo pipefail

BASE_REF="${1:-${BASE_REF:-origin/main}}"
HEAD_REF="${2:-${HEAD_REF:-HEAD}}"

block_count=0
review_count=0

annotate() {
    local level="$1" file="$2" line="$3" message="$4"
    if [ -n "${GITHUB_ACTIONS:-}" ]; then
        echo "::${level} file=${file},line=${line}::${message}"
    else
        echo "[${level}] ${file}:${line}: ${message}"
    fi
}

summary() {
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        echo "$1" >> "$GITHUB_STEP_SUMMARY"
    fi
}

block() {
    annotate error "$1" "$2" "$3"
    summary "- **BLOCK** \`$1\` — $3"
    block_count=$((block_count + 1))
}

review() {
    annotate warning "$1" "$2" "$3"
    summary "- REVIEW \`$1\` — $3"
    review_count=$((review_count + 1))
}

added_lines() {
    git diff -U0 --no-color --diff-filter=d "${BASE_REF}...${HEAD_REF}" -- "$@" 2>/dev/null |
        awk '
            /^\+\+\+ b\// { file = substr($0, 7); next }
            /^\+\+\+ /    { file = "";            next }
            /^@@ / {
                match($0, /\+[0-9]+/)
                lineno = substr($0, RSTART + 1, RLENGTH - 1) + 0
                next
            }
            /^\+/ && file != "" {
                print file "\t" lineno "\t" substr($0, 2)
                lineno++
            }
        '
}

changed() {
    git diff --name-only "${BASE_REF}...${HEAD_REF}" -- "$@" 2>/dev/null || true
}

echo "Reviewing dependency changes in ${BASE_REF}...${HEAD_REF}"
echo

# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------
while IFS=$'\t' read -r file lineno content; do
    [ -n "$file" ] || continue

    # A replace directive silently swaps a module for something else. Pointing
    # one at a fork or a local path is how a trusted import name ends up
    # resolving to somebody else's code.
    if echo "$content" | grep -qE '^[[:space:]]*replace[[:space:]]|^[[:space:]]*[^ ]+[[:space:]]+=>[[:space:]]'; then
        block "$file" "$lineno" \
            "go.mod replace directive: '$(echo "$content" | sed 's/^[[:space:]]*//')' — confirm where this module now resolves to"
    fi

    # New requires are worth naming in the summary so a reviewer sees the list
    # without expanding go.sum.
    # Bash treats tab as IFS whitespace and collapses runs of it, so the
    # indentation a require block has in the file is gone by the time the line
    # reaches here. Anchoring on leading whitespace would match nothing.
    if echo "$content" | grep -qE '^[[:space:]]*[a-z0-9.-]+\.[a-z]{2,}/[^ ]+ v[0-9]'; then
        dep=$(echo "$content" | awk '{print $1 " " $2}')
        review "$file" "$lineno" "new Go module: ${dep}"
    fi
done < <(added_lines 'go.mod')

# ---------------------------------------------------------------------------
# Rust
# ---------------------------------------------------------------------------
while IFS=$'\t' read -r file lineno content; do
    [ -n "$file" ] || continue

    # A git or path dependency bypasses crates.io entirely: no version, no
    # published artifact, and whatever the branch happens to contain today.
    if echo "$content" | grep -qE '(^|[^A-Za-z])git[[:space:]]*=[[:space:]]*"'; then
        block "$file" "$lineno" \
            "Cargo git dependency bypasses crates.io: $(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    if echo "$content" | grep -qE '(^|[^A-Za-z])path[[:space:]]*=[[:space:]]*"\.\./\.\.'; then
        block "$file" "$lineno" \
            "Cargo path dependency reaching outside the repository: $(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # build.rs runs at compile time, on every machine that builds this,
    # including CI and every contributor's laptop.
    if echo "$content" | grep -qE '^[[:space:]]*\[build-dependencies\]'; then
        review "$file" "$lineno" "adds [build-dependencies]; these run code at build time"
    fi
done < <(added_lines '*/Cargo.toml' 'Cargo.toml')

for f in $(changed '*/build.rs' 'build.rs'); do
    [ -n "$f" ] || continue
    review "$f" 1 "build.rs runs during compilation on every machine that builds this project"
done

# ---------------------------------------------------------------------------
# npm
#
# Lifecycle scripts execute on `npm install`, before anyone has run a test or
# read the code. This is the single most exploited hook in the ecosystem, and
# none of this project's packages has ever needed one.
# ---------------------------------------------------------------------------
while IFS=$'\t' read -r file lineno content; do
    [ -n "$file" ] || continue

    if echo "$content" | grep -qE '"(preinstall|postinstall|install|prepare|prepublish|preuninstall|postuninstall)"[[:space:]]*:'; then
        block "$file" "$lineno" \
            "npm lifecycle script runs on install: $(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # A dependency pointing at a tarball, a git host or a filesystem path is
    # not coming from the registry the lockfile protects.
    if echo "$content" | grep -qE '"[^"]+"[[:space:]]*:[[:space:]]*"(https?:|git\+|github:|file:)'; then
        block "$file" "$lineno" \
            "dependency resolved from outside the npm registry: $(echo "$content" | sed 's/^[[:space:]]*//')"
    fi
done < <(added_lines '*package.json')

# ---------------------------------------------------------------------------
# Lockfile consistency
#
# A manifest edited without its lockfile means CI resolved something nobody
# recorded, and the next run may resolve something else again.
# ---------------------------------------------------------------------------
check_lockfile() {
    local manifest="$1" lockfile="$2"
    if [ -n "$(changed "$manifest")" ] && [ -z "$(changed "$lockfile")" ]; then
        review "$manifest" 1 "changed without updating ${lockfile}"
    fi
}

check_lockfile 'go.mod' 'go.sum'
check_lockfile 'simulator/Cargo.toml' 'simulator/Cargo.lock'

for pkg in $(changed '*package.json'); do
    [ -n "$pkg" ] || continue
    lock="$(dirname "$pkg")/package-lock.json"
    if [ -f "$lock" ] && [ -z "$(changed "$lock")" ]; then
        review "$pkg" 1 "changed without updating $(basename "$lock")"
    fi
done

# ---------------------------------------------------------------------------
# Lockfile movement
#
# This is the shape a registry compromise takes on the way in. Nobody writes
# the malicious code into this repository — a version resolves somewhere new,
# the lockfile records it, and the payload arrives on whatever machine runs
# npm install next. The diff looks like routine maintenance.
#
# So: a lockfile that moves without its package.json moving is worth saying out
# loud, and a package resolved from anywhere other than the npm registry is
# worth stopping for.
# ---------------------------------------------------------------------------
while IFS=$'\t' read -r file lineno content; do
    [ -n "$file" ] || continue

    if echo "$content" | grep -qE '"resolved"[[:space:]]*:[[:space:]]*"' &&
       ! echo "$content" | grep -qE '"resolved"[[:space:]]*:[[:space:]]*"https://registry\.npmjs\.org/'; then
        block "$file" "$lineno" \
            "package resolved from outside the npm registry: $(echo "$content" | sed 's/^[[:space:]]*//')"
    fi
done < <(added_lines '*package-lock.json')

for lock in $(changed '*package-lock.json'); do
    [ -n "$lock" ] || continue
    dir="$(dirname "$lock")"
    manifest="${dir}/package.json"
    [ "$dir" = "." ] && manifest="package.json"

    if [ -z "$(changed "$manifest")" ]; then
        moved=$(git diff --numstat "${BASE_REF}...${HEAD_REF}" -- "$lock" 2>/dev/null | awk '{print $1}')
        review "$lock" 1 \
            "${moved:-0} line(s) of dependency movement with no change to ${manifest}; check which versions moved and why"
    fi
done

# ---------------------------------------------------------------------------
# Result
# ---------------------------------------------------------------------------
echo
summary ""
if [ "$block_count" -gt 0 ]; then
    summary "**${block_count} blocking finding(s), ${review_count} to review.**"
    echo "FAILED: ${block_count} blocking finding(s), ${review_count} for review"
    exit 1
fi

if [ "$review_count" -gt 0 ]; then
    summary "**${review_count} dependency change(s) for a reviewer.** None blocking."
    echo "PASSED with ${review_count} change(s) for review"
    exit 0
fi

summary "No dependency manifests changed."
echo "PASSED: no dependency changes"
