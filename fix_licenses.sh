# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

# Standard Apache 2.0 Header
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

# The FULL list of 45 failing files (including the new gasmodel ones)
FILES=(
"internal/decoder/ci_test.go"
"internal/telemetry/telemetry_test.go"
"internal/telemetry/telemetry.go"
"internal/trace/navigation_test.go"
"internal/trace/navigation.go"
"internal/trace/viewer.go"
"internal/daemon/server.go"
"internal/daemon/server_test.go"
"internal/cmd/generate_test.go"
"internal/cmd/auth_debug.go"
"internal/cmd/trace.go"
"internal/cmd/daemon.go"
"internal/localization/localizer.go"
"internal/localization/messages.go"
"internal/localization/localizer_test.go"
"internal/gasmodel/types.go"
"internal/gasmodel/validator.go"
"internal/gasmodel/parser_test.go"
"internal/gasmodel/parser.go"
"internal/ipc/types.go"
"internal/authtrace/types.go"
"internal/authtrace/tracker_test.go"
"internal/authtrace/custom_auth.go"
"internal/authtrace/reporter.go"
"internal/authtrace/tracker.go"
"internal/updater/checker_integration_test.go"
"internal/testgen/generator.go"
"internal/testgen/templates.go"
"internal/simulator/time_travel_test.go"
"internal/simulator/profiling_test.go"
"internal/simulator/interface_test.go"
"main.go"
"test/integration/otel_integration.go"
"test/generate_sample_trace.go"
"simulator/src/theme/ansi.rs"
"simulator/src/theme/loader.rs"
"simulator/src/theme/mod.rs"
"simulator/src/theme/theme.rs"
"simulator/src/cli/trace_viewer.rs"
"simulator/src/cli/mod.rs"
"simulator/src/config/paths.rs"
"simulator/src/config/mod.rs"
"simulator/src/ipc/validate.rs"
"simulator/src/ipc/mod.rs"
"src/ipc/types.rs"
)

echo "Starting license fix..."

for FILE in "${FILES[@]}"; do
    if [ -f "$FILE" ]; then
        # Check if the file has the header (checking for "Licensed under")
        if ! grep -q "Licensed under the Apache License" "$FILE"; then
            echo "Adding header to $FILE"
            # Create temp file with header + original content
            echo -e "$HEADER\n\n$(cat $FILE)" > "$FILE"
        else
            echo "Skipping $FILE (Header already present)"
        fi
    else
        echo "Warning: $FILE not found, skipping."
    fi
done

echo "All files processed."