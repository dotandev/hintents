#!/usr/bin/env bash
set -euo pipefail

app_path="${HOME}/Applications/Erst Protocol.app"
plist_path="${app_path}/Contents/Info.plist"
helper_script="${app_path}/Contents/MacOS/erst-protocol-handler"
lsregister="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

if [[ ! -f "${plist_path}" ]]; then
  echo "missing plist: ${plist_path}" >&2
  exit 1
fi

if [[ ! -x "${helper_script}" ]]; then
  echo "missing helper script: ${helper_script}" >&2
  exit 1
fi

grep -F '<key>CFBundleURLSchemes</key>' "${plist_path}" >/dev/null
grep -F '<string>erst</string>' "${plist_path}" >/dev/null

if [[ -n "${ERST_BINARY:-}" ]]; then
  grep -F "${ERST_BINARY}" "${helper_script}" >/dev/null
fi

if [[ -x "${lsregister}" ]]; then
  "${lsregister}" -dump | grep -F "${app_path}" >/dev/null
  "${lsregister}" -dump | grep -F 'erst' >/dev/null
fi

echo "macOS protocol registration verified"