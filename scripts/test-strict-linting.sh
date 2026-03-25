#!/usr/bin/env bash
# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0


# Copyright 2026 Erst Users

# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

# Copyright (c) 2025 ERST Contributors


# SPDX-License-Identifier: Apache-2.0
#
# Test script to verify strict linting configuration
# This script creates temporary files with linting issues to ensure they are caught

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

