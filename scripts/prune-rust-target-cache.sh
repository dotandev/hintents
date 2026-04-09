#!/usr/bin/env bash

set -euo pipefail

target_dir="${1:-simulator/target}"

if [[ ! -d "$target_dir" ]]; then
  echo "No Rust target directory to prune at $target_dir"
  exit 0
fi

echo "Pruning Rust target cache under $target_dir"

rm -rf \
  "$target_dir/tmp" \
  "$target_dir/doc" \
  "$target_dir/package" \
  "$target_dir/.future-incompat-report.json"

for profile_dir in "$target_dir"/{debug,release}; do
  [[ -d "$profile_dir" ]] || continue

  # Incremental and scratch outputs grow quickly but do not materially help cache reuse.
  rm -rf "$profile_dir/incremental" "$profile_dir/examples"

  if [[ -d "$profile_dir/deps" ]]; then
    find "$profile_dir/deps" -type f \( -name "*.d" -o -name "*.tmp" \) -delete
  fi

  if [[ -d "$profile_dir/build" ]]; then
    find "$profile_dir/build" -mindepth 2 -maxdepth 2 -type d -name out -exec rm -rf {} +
  fi
done
