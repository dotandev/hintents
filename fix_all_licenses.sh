#!/bin/bash

HEADER="// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

"

echo "Scanning ALL .go and .rs files for missing headers..."

# Find all .go and .rs files, ignoring hidden folders
find . -type f \( -name "*.go" -o -name "*.rs" \) -not -path "*/.*" -not -path "*/vendor/*" -not -path "*/target/*" | while read -r FILE; do
    if ! grep -q "Copyright 2025 Erst Users" "$FILE"; then
        echo "🔧 Fixing: $FILE"
        # Read the header into a temp file
        printf '%s' "$HEADER" > "$FILE.new"
        # Append the original file content
        cat "$FILE" >> "$FILE.new"
        # Replace the original file
        mv "$FILE.new" "$FILE"
    fi
done
echo "✅ Done."
