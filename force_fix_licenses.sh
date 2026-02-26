# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

# The Correct Header (Standard Apache 2.0 with (c))
HEADER="// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License."

echo "Force-fixing headers in all Go and Rust files..."

# Find all .go and .rs files
find . -type f \( -name "*.go" -o -name "*.rs" \) -not -path "*/.*" | while read -r FILE; do
    
    # 1. Read the file into a temp variable, skipping the old header lines
    # We assume code starts with 'package' (Go) or 'use/mod/fn' (Rust) or just generic strip
    # Safer method: Strip leading comments (//) until the first non-comment line
    
    CONTENT=$(cat "$FILE")
    
    # Check if it's a Go file
    if [[ "$FILE" == *.go ]]; then
        # Remove all lines from the top until we hit "package"
        # We keep the "package" line and everything after
        CLEAN_CONTENT=$(awk '/^package/ {p=1} p' "$FILE")
    else
        # For Rust, it's harder to guess where code starts (could be use, mod, fn)
        # So we just strip the specific Apache header lines if they exist to avoid duplicates
        # Then we overwrite.
        CLEAN_CONTENT=$(grep -vE "^// Copyright|^// Licensed|^// you may|^//      http|^// Unless|^// distributed|^// See the|^// limitations" "$FILE")
        # Remove empty lines at the top (optional, but clean)
        CLEAN_CONTENT=$(echo "$CLEAN_CONTENT" | sed '/./,$!d')
    fi

    # 2. Write the New Header + Clean Content
    # We verify CLEAN_CONTENT is not empty to avoid wiping files
    if [ -n "$CLEAN_CONTENT" ]; then
        echo -e "$HEADER\n\n$CLEAN_CONTENT" > "$FILE"
    else
        echo "[WARN] Warning: Could not detect code start for $FILE. Skipping."
    fi

done

echo "[OK] All files force-updated."
