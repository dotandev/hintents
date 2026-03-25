#!/usr/bin/env bash
# fix_all_licenses.sh
# Audits .go, .rs, .sh, .js/.ts files for license headers.
# Usage:
#   ./fix_all_licenses.sh           # fix mode (adds headers in-place)
#   ./fix_all_licenses.sh --check   # check mode (fails CI if missing, no changes)

set -euo pipefail

CHECK_MODE=false
if [[ "${1:-}" == "--check" ]]; then
  CHECK_MODE=true
fi

# License headers
GO_RS_HEADER="// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0"

SH_HEADER="# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0"

JS_TS_HEADER="// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0"

# Helper: check or fix one file
check_or_fix() {
  local file="$1"
  local header="$2"
  local first_line_pat="$3"

  if head -n 1 "$file" | grep -qF "$first_line_pat"; then
    return 0
  fi

  if [[ $CHECK_MODE == true ]]; then
    echo "[missing] $file"
    return 1
  else
    echo "[fixed] $file"
    printf '%s\n\n' "$header" | cat - "$file" > "$file.tmp"
    mv "$file.tmp" "$file"
    return 0
  fi
}

# Find files (respects .gitignore when possible)
list_files() {
  local ext="$1"
  if git rev-parse --is-inside-work-tree &>/dev/null; then
    git ls-files | grep -E "\\.${ext}$" || true
  else
    find . -type f -name "*.$ext" \
      -not -path "*/.*" \
      -not -path "*/target/*" \
      -not -path "*/node_modules/*" \
      -not -path "*/vendor/*"
  fi
}

MISSING=0

echo "=== License Header Audit ==="

echo "Go & Rust files:"
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  check_or_fix "$f" "$GO_RS_HEADER" "// Copyright" || MISSING=$((MISSING + 1))
done < <(list_files go; list_files rs)

echo ""
echo "Shell scripts:"
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  # Skip this script itself
  [[ "$(realpath "$f")" == "$(realpath "$0")" ]] && continue
  if head -n 1 "$f" | grep -q '^#!'; then
    if sed -n '2p' "$f" | grep -qF "# Copyright"; then
      continue
    fi
    if [[ $CHECK_MODE == true ]]; then
      echo "[missing] $f"
      MISSING=$((MISSING + 1))
    else
      echo "[fixed] $f"
      {
        sed -n '1p' "$f"
        printf '%s\n\n' "$SH_HEADER"
        tail -n +2 "$f"
      } > "$f.tmp"
      mv "$f.tmp" "$f"
    fi
  else
    check_or_fix "$f" "$SH_HEADER" "# Copyright" || MISSING=$((MISSING + 1))
  fi
done < <(list_files sh)

echo ""
echo "JavaScript/TypeScript files:"
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  check_or_fix "$f" "$JS_TS_HEADER" "// Copyright" || MISSING=$((MISSING + 1))
done < <({ list_files js; list_files ts; list_files mjs; list_files cjs; })

echo ""
echo "=== Summary ==="
if [[ $MISSING -gt 0 ]]; then
  echo "FAIL: $MISSING file(s) missing license headers."
  if [[ $CHECK_MODE == true ]]; then
    echo "Run './fix_all_licenses.sh' to fix them locally."
  fi
  exit 1
else
  echo "OK: All audited files have proper headers."
fi
