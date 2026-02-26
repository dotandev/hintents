# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash
for file in $(find . -name "*.go" -type f); do
    perl -i -pe 's{"github\.com/stellar/go/}{"github.com/stellar/go-stellar-sdk/}g if /^import/ .. /^\)/' "$file"
done
