#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

remove_dir_contents() {
  local dir="$1"
  if [[ -d "$dir" ]]; then
    echo "Cleaning directory contents: $dir"
    find "$dir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  fi
}

remove_path() {
  local path="$1"
  if [[ -e "$path" ]]; then
    echo "Removing path: $path"
    rm -rf "$path"
  fi
}

if [[ -n "${RUNNER_TEMP:-}" ]]; then
  remove_dir_contents "$RUNNER_TEMP"
fi

remove_path "$repo_root/simulator/target/tmp"
remove_path "$repo_root/simulator/target/doc"
remove_path "$repo_root/simulator/target/package"

for profile_dir in "$repo_root"/simulator/target/{debug,release}; do
  [[ -d "$profile_dir" ]] || continue
  remove_path "$profile_dir/incremental"
  remove_path "$profile_dir/examples"
  find "$profile_dir/deps" -type f \( -name "*.d" -o -name "*.tmp" \) -delete 2>/dev/null || true
done
