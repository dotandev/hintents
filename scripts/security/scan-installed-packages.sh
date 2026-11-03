#!/usr/bin/env bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Scans INSTALLED npm packages for the payloads used in registry compromises.
#
# This covers an attack the diff scanner structurally cannot see. When a
# maintainer account is phished and a malicious version is published, the
# payload ships inside the package — an api.js, an index.mjs, a bundle.js —
# and arrives on a machine during `npm install`. Nothing about it appears in
# any pull request to this repository. The lockfile moves a patch version, or
# a caret range resolves somewhere new, and the code is simply there.
#
# Recent examples of exactly this shape:
#
#   - chalk / debug / ansi-styles (Sept 2025): maintainer phished, a crypto
#     address swapper injected into packages with ~2bn weekly downloads. The
#     payload hooked window.fetch and XMLHttpRequest and rewrote wallet
#     addresses in flight.
#   - Shai-Hulud (Sept 2025, and again Nov 2025): a self-replicating worm that
#     harvested credentials with TruffleHog, pushed them to attacker-controlled
#     endpoints, and republished itself through every package the compromised
#     maintainer could publish to.
#
# Run this AFTER installing with --ignore-scripts, so an install-time payload
# has not already executed by the time anything looks at it.
#
# Usage:
#   scripts/security/scan-installed-packages.sh [dir...]
#
# With no arguments, scans every directory in the repository that has a
# package-lock.json.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
ALLOWLIST="${REPO_ROOT}/.github/security-allowlist.txt"

# Packages this project accepts as needing an install script. Everything else
# that wants to run code on install is treated as new and blocked.
EXPECTED_SCRIPTS="${REPO_ROOT}/.github/expected-install-scripts.txt"

block_count=0
review_count=0

annotate() {
    local level="$1" file="$2" message="$3"
    if [ -n "${GITHUB_ACTIONS:-}" ]; then
        echo "::${level} file=${file}::${message}"
    else
        echo "[${level}] ${file}: ${message}"
    fi
}

summary() {
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        echo "$1" >> "$GITHUB_STEP_SUMMARY"
    fi
}

is_allowlisted() {
    local check_id="$1" subject="$2"
    [ -f "$ALLOWLIST" ] || return 1
    grep -qE "^[[:space:]]*${check_id}[[:space:]]+${subject}[[:space:]]*$" "$ALLOWLIST" 2>/dev/null
}

report() {
    local severity="$1" check_id="$2" subject="$3" message="$4"

    if is_allowlisted "$check_id" "$subject"; then
        return 0
    fi

    if [ "$severity" = "BLOCK" ]; then
        annotate error "$subject" "[${check_id}] ${message}"
        summary "- **BLOCK** \`${check_id}\` ${subject} — ${message}"
        block_count=$((block_count + 1))
    else
        annotate warning "$subject" "[${check_id}] ${message}"
        summary "- REVIEW \`${check_id}\` ${subject} — ${message}"
        review_count=$((review_count + 1))
    fi
}

# Which trees to scan.
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

# Only source that can execute. Scanning .map, .md or .json would multiply the
# work for no signal.
#
# -I skips binary files. Compiled native addons are a real attack surface, but
# grepping a .node binary for JavaScript patterns only ever produces noise —
# every Next.js install ships next-swc, and random bytes in it match a hex
# wallet address. Those need a different tool, not this one.
CODE_EXTENSIONS=(-I --include='*.js' --include='*.mjs' --include='*.cjs' --include='*.ts')

# ---------------------------------------------------------------------------
# Indicators
#
# Each pattern below is something no legitimate dependency of a Stellar SDK
# does. They are drawn from how the recent registry compromises actually
# behaved, not from a generic malware list.
# ---------------------------------------------------------------------------

# Endpoints used to receive stolen data. Request-capture services exist to be
# disposable, which is why this family of attack keeps reaching for them.
EXFIL_ENDPOINTS='webhook\.site|\.oast\.fun|\.oastify\.com|\.interact\.sh|requestbin\.(com|net)|pipedream\.net|\.burpcollaborator\.net|discord(app)?\.com/api/webhooks|api\.telegram\.org/bot'

# Credential harvesting. Shai-Hulud shipped TruffleHog to sweep the host for
# anything that looked like a token before exfiltrating it.
CRED_HARVEST='trufflehog|TruffleHog|\.aws/credentials|\.npmrc["'"'"'`][[:space:]]*\)|GITHUB_TOKEN[^A-Za-z_].*(fetch|request|axios|https?\.)|JSON\.stringify[[:space:]]*\([[:space:]]*process\.env[[:space:]]*\)'

# The chalk/debug payload: take over the browser's network primitives so every
# response can be rewritten on the way through, then swap wallet addresses.
# A Stellar address in a package that has nothing to do with Stellar is the
# same trick aimed at this ecosystem.
ADDRESS_SWAP='XMLHttpRequest\.prototype\.open[[:space:]]*=|window\.fetch[[:space:]]*=[[:space:]]*(function|async|\()|window\.ethereum|0x[a-fA-F0-9]{40}|G[A-Z2-7]{55}'

echo "Scanning installed packages for registry-compromise payloads"
echo

for target in "${TARGETS[@]}"; do
    modules="${REPO_ROOT}/${target}/node_modules"
    [ "$target" = "." ] && modules="${REPO_ROOT}/node_modules"

    if [ ! -d "$modules" ]; then
        echo "  ${target}: no node_modules, nothing installed to scan"
        continue
    fi

    count=$(find "$modules" -maxdepth 2 -name package.json 2>/dev/null | wc -l | tr -d ' ')
    echo "::group::${target} (${count} package(s))"

    # -----------------------------------------------------------------------
    # Install-time execution
    #
    # Only preinstall, install and postinstall run on a consumer installing
    # from the registry. `prepare` is a publish-time hook — npm runs it before
    # packing a tarball, and on install only for git or local dependencies —
    # so treating it as dangerous flags acorn, mime and ts-node and teaches
    # everyone to ignore this check.
    #
    # These trees currently declare none of the three, so one appearing means
    # a new package brought it in. That is worth stopping on: an install
    # script runs before any code is read or any test is run.
    # -----------------------------------------------------------------------
    while IFS= read -r manifest; do
        [ -n "$manifest" ] || continue

        pkg="${manifest#"$modules"/}"
        pkg="${pkg%/package.json}"
        # A nested dependency appears as a/node_modules/b; the name that
        # matters is the last segment, scope included.
        name="${pkg##*node_modules/}"

        if grep -qE "^[[:space:]]*${name}[[:space:]]*(#.*)?$" "$EXPECTED_SCRIPTS" 2>/dev/null; then
            continue
        fi

        hook=$(grep -oE '"(preinstall|postinstall|install)"[[:space:]]*:' "$manifest" 2>/dev/null | head -1 | tr -d '":')
        report BLOCK install-hook "${target}/node_modules/${pkg}" \
            "declares a ${hook} script that runs on npm install and is not in .github/expected-install-scripts.txt"
    done < <(find "$modules" -maxdepth 4 -name package.json 2>/dev/null |
             xargs grep -lE '"(preinstall|postinstall|install)"[[:space:]]*:[[:space:]]*"' 2>/dev/null || true)

    # -----------------------------------------------------------------------
    # Exfiltration endpoints
    # -----------------------------------------------------------------------
    while IFS= read -r hit; do
        [ -n "$hit" ] || continue
        report BLOCK exfil-endpoint "${hit#"$REPO_ROOT"/}" \
            "contains a request-capture or chat-webhook endpoint used to receive stolen data"
    done < <(grep -rlE "$EXFIL_ENDPOINTS" "${CODE_EXTENSIONS[@]}" "$modules" 2>/dev/null || true)

    # -----------------------------------------------------------------------
    # Credential harvesting
    # -----------------------------------------------------------------------
    while IFS= read -r hit; do
        [ -n "$hit" ] || continue
        report BLOCK credential-harvest "${hit#"$REPO_ROOT"/}" \
            "reads credential stores or serialises the environment for sending"
    done < <(grep -rlE "$CRED_HARVEST" "${CODE_EXTENSIONS[@]}" "$modules" 2>/dev/null || true)

    # -----------------------------------------------------------------------
    # Network interception and address swapping
    #
    # REVIEW rather than BLOCK: mocking libraries and analytics SDKs patch
    # fetch for honest reasons, and this repository installs several.
    # -----------------------------------------------------------------------
    while IFS= read -r hit; do
        [ -n "$hit" ] || continue
        report REVIEW address-swap "${hit#"$REPO_ROOT"/}" \
            "overrides a network primitive or embeds wallet address patterns; confirm what it rewrites"
    done < <(grep -rlE "$ADDRESS_SWAP" "${CODE_EXTENSIONS[@]}" "$modules" 2>/dev/null || true)

    echo "::endgroup::"
done

echo
summary ""
if [ "$block_count" -gt 0 ]; then
    summary "**${block_count} blocking finding(s) in installed packages, ${review_count} to review.**"
    echo "FAILED: ${block_count} blocking finding(s), ${review_count} for review"
    exit 1
fi

if [ "$review_count" -gt 0 ]; then
    summary "**${review_count} installed package(s) for a reviewer.** None blocking."
    echo "PASSED with ${review_count} finding(s) for review"
    exit 0
fi

summary "No registry-compromise payloads found in installed packages."
echo "PASSED: nothing flagged"
