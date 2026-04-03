#!/usr/bin/env bash
set -euo pipefail

desktop_file="${HOME}/.local/share/applications/erst-protocol.desktop"
helper_script="${HOME}/.local/share/erst/erst-protocol-handler"

if [[ ! -f "${desktop_file}" ]]; then
  echo "missing desktop file: ${desktop_file}" >&2
  exit 1
fi

if [[ ! -x "${helper_script}" ]]; then
  echo "missing helper script: ${helper_script}" >&2
  exit 1
fi

grep -F "MimeType=x-scheme-handler/erst;" "${desktop_file}" >/dev/null
grep -F "Exec=${helper_script} %u" "${desktop_file}" >/dev/null

if [[ -n "${ERST_BINARY:-}" ]]; then
  grep -F "${ERST_BINARY}" "${helper_script}" >/dev/null
fi

default_handler="$(xdg-mime query default x-scheme-handler/erst)"
if [[ "${default_handler}" != "erst-protocol.desktop" ]]; then
  echo "xdg-mime returned ${default_handler}, expected erst-protocol.desktop" >&2
  exit 1
fi

echo "linux protocol registration verified"