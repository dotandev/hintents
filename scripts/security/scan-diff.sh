#!/usr/bin/env bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Scans the lines a change ADDS for patterns associated with malicious code.
#
# Only added lines are examined. The repository holds 2,000+ commits of
# existing code, so scanning the whole tree would bury a real finding under
# pre-existing matches and the check would be switched off within a week.
#
# Findings come in two severities:
#
#   BLOCK   Unambiguous. Exits non-zero and fails the build.
#   REVIEW  A heuristic that needs a human. Annotated on the pull request and
#           written to the job summary, but does not fail the build.
#
# The split is deliberate. This repository legitimately embeds base64 XDR,
# shells out to a simulator binary and talks to Stellar RPC endpoints, so
# heuristics tuned tightly enough to block would cry wolf constantly. Their job
# is to put a reviewer's eye on the right twenty lines, not to be the gate.
#
# Usage:
#   scripts/security/scan-diff.sh [base-ref] [head-ref]
#
# Defaults to origin/main...HEAD so contributors can run it before pushing.

set -euo pipefail

BASE_REF="${1:-${BASE_REF:-origin/main}}"
HEAD_REF="${2:-${HEAD_REF:-HEAD}}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
ALLOWLIST="${REPO_ROOT}/.github/security-allowlist.txt"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

block_count=0
review_count=0

# GitHub renders ::error and ::warning as annotations on the diff itself, which
# is where a reviewer is already looking. Outside Actions they are just lines.
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

# Waivers are keyed by check and file rather than by line, so they survive the
# code moving around. A waiver is a change to a tracked file, so adding one is
# itself visible in review — which is the point: suppressing a security check
# should cost a conversation, not a quiet edit.
is_allowlisted() {
    local check_id="$1" file="$2"
    [ -f "$ALLOWLIST" ] || return 1
    grep -qE "^[[:space:]]*${check_id}[[:space:]]+${file}[[:space:]]*$" "$ALLOWLIST"
}

report() {
    local severity="$1" check_id="$2" file="$3" line="$4" message="$5"

    if is_allowlisted "$check_id" "$file"; then
        return 0
    fi

    if [ "$severity" = "BLOCK" ]; then
        annotate error "$file" "$line" "[${check_id}] ${message}"
        summary "- **BLOCK** \`${check_id}\` ${file}:${line} — ${message}"
        block_count=$((block_count + 1))
    else
        annotate warning "$file" "$line" "[${check_id}] ${message}"
        summary "- REVIEW \`${check_id}\` ${file}:${line} — ${message}"
        review_count=$((review_count + 1))
    fi
}

# emit <severity> <check-id> <message>
#
# Reads "path<TAB>line<TAB>content" on stdin and reports each one. Checks are
# written as a single awk or grep pass that selects lines and pipes them here,
# so a large diff costs a handful of processes rather than one per line.
emit() {
    local severity="$1" check_id="$2" message="$3"
    local file lineno content
    while IFS=$'\t' read -r file lineno content; do
        [ -n "$file" ] || continue
        report "$severity" "$check_id" "$file" "$lineno" "$message"
    done
}

# added_lines <pathspec...>
#
# Emits "path<TAB>lineno<TAB>content" for every line the diff adds. -U0 drops
# context lines, so inside a hunk every '+' line is genuinely new and the
# counter tracks the post-change file. Deleted lines carry no new line number
# and are skipped.
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

changed_files() {
    git diff --name-only --diff-filter=d "${BASE_REF}...${HEAD_REF}" -- "$@" 2>/dev/null || true
}

# Source files only. Fixtures, generated declarations and vendored trees match
# these patterns constantly and tell a reviewer nothing.
SOURCE_PATHS=(
    '*.go' '*.rs' '*.ts' '*.tsx' '*.js' '*.jsx' '*.mjs' '*.cjs' '*.sh' '*.py'
    ':(exclude)*_test.go'
    ':(exclude)*.d.ts'
    ':(exclude)*/testdata/*'
    ':(exclude)tests/*'
    ':(exclude)test/*'
    ':(exclude)*/node_modules/*'
    ':(exclude)simulator/target/*'
)

SRC="$WORK/source.tsv"
ALL="$WORK/all.tsv"
WORKFLOWS="$WORK/workflows.tsv"

added_lines "${SOURCE_PATHS[@]}" > "$SRC"
added_lines '*' > "$ALL"
added_lines '.github/workflows/*' '.github/actions/*' > "$WORKFLOWS"

echo "Scanning ${BASE_REF}...${HEAD_REF} for malicious-code patterns"
echo "$(wc -l < "$ALL" | tr -d ' ') added line(s), $(wc -l < "$SRC" | tr -d ' ') of them in source files"
echo

# A diff line is emitted as path<TAB>line<TAB>content, but source content is
# full of tabs of its own — Go indents with them — so awk's $3 stops at the
# first one. Every filter below rebuilds the content field as everything after
# the second tab. Getting this wrong silently matches nothing, which is the
# worst way for a security check to fail.
AWK_CONTENT='function content(line,   p, r, q) {
    p = index(line, "\t")
    r = substr(line, p + 1)
    q = index(r, "\t")
    return substr(r, q + 1)
}'

# ---------------------------------------------------------------------------
# 1. Key material reaching an output sink
#
# The worst outcome for a Stellar SDK is a signing key or HSM PIN leaving the
# process. This repository handles Ed25519 private keys, PKCS#11 PINs and seed
# material, and has an open issue for exactly this class of leak, so a line
# that both names key material and hands it to a logger, formatter or network
# call is worth a reviewer's time every time.
# ---------------------------------------------------------------------------
KEY_MATERIAL='(secret|private)[_-]?key|seed[_-]?phrase|mnemonic|passphrase|signing[_-]?key|privkey|[_-]pin[^a-z]|pin[_-]'
OUTPUT_SINK='fmt\.(Print|Sprint|Fprint)|log\.|logger\.|slog\.|println!|eprintln!|print!|tracing::(info|warn|error|debug|trace)|console\.(log|error|warn)|http\.(Post|Get|NewRequest)|reqwest::|\.Write\(|os\.Stdout|os\.Stderr'

emit REVIEW key-material-sink \
    "names key material and passes it to an output sink; confirm the value is redacted" \
    < <(KEY_MATERIAL="$KEY_MATERIAL" OUTPUT_SINK="$OUTPUT_SINK" awk "
        $AWK_CONTENT
        { c = content(\$0) }
        tolower(c) ~ ENVIRON[\"KEY_MATERIAL\"] && c ~ ENVIRON[\"OUTPUT_SINK\"]
    " "$SRC")

# ---------------------------------------------------------------------------
# 2. Obfuscation and dynamic execution
#
# Decoding a blob and running it is the shape almost every injected payload
# takes, and no legitimate change here needs it. Long encoded literals on their
# own are NOT flagged: XDR envelopes are base64 and appear throughout this
# codebase, so that pattern would be pure noise.
# ---------------------------------------------------------------------------
DYNAMIC_EXEC='\beval[[:space:]]*\(|new[[:space:]]+Function[[:space:]]*\(|\batob[[:space:]]*\(|\bexec[[:space:]]*\([[:space:]]*(atob|Buffer\.from|decodeURIComponent)'
SHELL_EXEC='exec\.Command[[:space:]]*\([[:space:]]*"(sh|bash|zsh|cmd|powershell)"|Command::new[[:space:]]*\([[:space:]]*"(sh|bash|zsh)"|child_process|subprocess\.(Popen|run|call)\(.*shell[[:space:]]*=[[:space:]]*True'

emit BLOCK dynamic-exec "evaluates code at runtime; nothing in this project needs this" \
    < <(DYNAMIC_EXEC="$DYNAMIC_EXEC" awk "
        $AWK_CONTENT
        content(\$0) ~ ENVIRON[\"DYNAMIC_EXEC\"]
    " "$SRC")

emit REVIEW shell-exec "spawns a shell; confirm no part of the command line comes from input" \
    < <(SHELL_EXEC="$SHELL_EXEC" awk "
        $AWK_CONTENT
        content(\$0) ~ ENVIRON[\"SHELL_EXEC\"]
    " "$SRC")

# ---------------------------------------------------------------------------
# 3. Network egress to an unexpected host
#
# Exfiltration needs somewhere to send to. The allowlist covers the hosts this
# project legitimately reaches: the Stellar networks, the language registries,
# and the places its own site, docs and source live.
# ---------------------------------------------------------------------------
ALLOWED_HOSTS='stellar\.org|stellar\.expert|erstt\.xyz|github\.com|githubusercontent\.com|crates\.io|docs\.rs|golang\.org|pkg\.go\.dev|npmjs\.(org|com)|nodejs\.org|apache\.org|opensource\.org|mintlify\.(app|com)|localhost|127\.0\.0\.1|0\.0\.0\.0|example\.(com|org)|w3\.org|json-schema\.org'

while IFS=$'\t' read -r file lineno host; do
    [ -n "$file" ] || continue
    report REVIEW unexpected-egress "$file" "$lineno" \
        "new request to ${host}, which is not a host this project normally talks to"
done < <(ALLOWED_HOSTS="$ALLOWED_HOSTS" awk "
    $AWK_CONTENT
    {
        allowed = ENVIRON[\"ALLOWED_HOSTS\"]
        p = index(\$0, \"\t\")
        file = substr(\$0, 1, p - 1)
        r = substr(\$0, p + 1)
        q = index(r, \"\t\")
        lineno = substr(r, 1, q - 1)

        line = content(\$0)
        while (match(line, /https?:\/\/[A-Za-z0-9._~:\/?#@!\$&()*+,;=%-]+/)) {
            url = substr(line, RSTART, RLENGTH)
            line = substr(line, RSTART + RLENGTH)

            host = url
            sub(/^https?:\/\//, \"\", host)
            sub(/[\/:?#].*\$/, \"\", host)

            if (host !~ (\"^(\" allowed \")\$\") && host !~ (\"\\\\.(\" allowed \")\$\"))
                print file \"\t\" lineno \"\t\" host
        }
    }
" "$SRC")

# ---------------------------------------------------------------------------
# 4. Workflow tampering
#
# A pull request that edits CI runs code with the repository's credentials.
# These are the standard route from "contributor" to "holds your secrets".
# ---------------------------------------------------------------------------
emit BLOCK pull-request-target "pull_request_target runs fork code with repository secrets" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /^[[:space:]]*pull_request_target[[:space:]]*:/
    " "$WORKFLOWS")

# An expression expanded straight into a shell body lets anything the author
# controls — a branch name, a pull request title — become shell.
emit REVIEW workflow-expression \
    "interpolates event data; if this reaches a run: block it is shell injection, pass it through env instead" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /\\\$\\{\\{[[:space:]]*(github\.event|github\.head_ref|inputs)\./
    " "$WORKFLOWS")

emit REVIEW unpinned-action \
    "action pinned to a moving branch; whoever owns it can change what runs here" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*[^.\/].*@(master|main)[[:space:]]*\$/
    " "$WORKFLOWS")

for wf in $(changed_files '.github/workflows/*' '.github/security-allowlist.txt'); do
    [ -n "$wf" ] || continue
    report REVIEW workflow-changed "$wf" 1 \
        "changes CI or its waivers; review this before reviewing the rest of the diff"
done

# ---------------------------------------------------------------------------
# 5. Credentials committed to the repository
#
# These run over every added line, tests and documentation included: a real key
# is just as live in a fixture as in production code, and nothing on the
# outside can tell the difference. The formats below are distinctive enough to
# be certain about — a Stellar secret seed is an S followed by 55 base32
# characters and nothing else looks like that.
#
# The PEM header only asks for a look, because the redaction code and the HSM
# documentation both quote it on purpose.
# ---------------------------------------------------------------------------
emit BLOCK stellar-secret-seed \
    "a Stellar secret seed; if this key is real it is compromised the moment it is pushed" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /(^|[^A-Z2-7])S[A-Z2-7]{55}([^A-Z2-7]|\$)/
    " "$ALL")

emit BLOCK aws-access-key "an AWS access key id" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /(^|[^A-Z0-9])AKIA[0-9A-Z]{16}([^0-9A-Z]|\$)/
    " "$ALL")

emit BLOCK github-token "a GitHub personal access token" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /gh[pousr]_[A-Za-z0-9]{36,}/
    " "$ALL")

emit REVIEW pem-private-key \
    "a PEM private key header; confirm this is a pattern or a fixture, not a real key" \
    < <(awk "
        $AWK_CONTENT
        content(\$0) ~ /^-----BEGIN [A-Z ]*PRIVATE KEY-----/
    " "$ALL")

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
    summary "**${review_count} finding(s) for a reviewer.** None blocking."
    echo "PASSED with ${review_count} finding(s) for review"
    exit 0
fi

summary "No malicious-code patterns found in the added lines."
echo "PASSED: nothing flagged"
