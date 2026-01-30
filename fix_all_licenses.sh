#!/bin/bash

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

echo "Scanning ALL .go and .rs files for missing headers..."

# Find all .go and .rs files, ignoring hidden folders
find . -type f \( -name "*.go" -o -name "*.rs" \) -not -path "*/.*" | while read -r FILE; do
    if ! grep -q "Licensed under the Apache License" "$FILE"; then
        echo "🔧 Fixing: $FILE"
        echo -e "$HEADER\n\n$(cat "$FILE")" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    fi
done
echo "✅ Done."
