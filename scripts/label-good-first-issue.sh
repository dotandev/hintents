#!/usr/bin/env bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Add "good first issue" label to issues listed in docs/community/LABELS_AUDIT.md.
# Requires GITHUB_TOKEN with repo scope (issues: write).
# Usage: GITHUB_TOKEN=xxx ./scripts/label-good-first-issue.sh
# Override repo: GITHUB_REPO=owner/repo GITHUB_TOKEN=xxx ./scripts/label-good-first-issue.sh

set -e

REPO="${GITHUB_REPO:-dotandev/hintents}"
LABEL="good first issue"
# Issue numbers from LABELS_AUDIT.md (same list as doc)
ISSUES=(32 81 84 86 87 114 116 130 131 162)

if [ -z "${GITHUB_TOKEN}" ]; then
  echo "Error: GITHUB_TOKEN is not set. Export it with repo scope (issues: write)."
  exit 1
fi

echo "Adding label \"${LABEL}\" to ${#ISSUES[@]} issues in ${REPO}..."
for num in "${ISSUES[@]}"; do
  resp=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${REPO}/issues/${num}/labels" \
    -d "[\"${LABEL}\"]")
  code=$(echo "$resp" | tail -n1)
  body=$(echo "$resp" | sed '$d')
  if [ "$code" = "200" ] || [ "$code" = "201" ]; then
    echo "  #${num} OK"
  else
    echo "  #${num} HTTP ${code} ${body}"
  fi
done
echo "Done."
