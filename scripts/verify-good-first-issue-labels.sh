#!/usr/bin/env bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Verify at least 10 open issues have the "good first issue" label (read-only, no token).
# Usage: ./scripts/verify-good-first-issue-labels.sh

set -e

REPO="${GITHUB_REPO:-dotandev/hintents}"
LABEL="good%20first%20issue"
REQUIRED=10

count=$(curl -s -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/${REPO}/issues?state=open&labels=${LABEL}&per_page=100" \
  | python3 -c "
import json, sys
data = json.load(sys.stdin)
# Exclude pull requests
issues = [i for i in data if 'pull_request' not in i]
print(len(issues))
")

echo "Issues with label \"good first issue\": ${count} (required: ≥${REQUIRED})"
if [ "$count" -ge "$REQUIRED" ]; then
  echo "OK – Success criteria met."
  exit 0
else
  echo "Not yet – Apply the label to more issues (see docs/community/LABELS_AUDIT.md)."
  exit 1
fi
