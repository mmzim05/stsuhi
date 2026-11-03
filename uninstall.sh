#!/usr/bin/env bash
set -euo pipefail

readonly BIN_PATH="${HOME}/.local/bin/stsuhi"
readonly DESKTOP_DIR="${HOME}/.local/share/applications"
readonly DESKTOP_FILE="${DESKTOP_DIR}/stsuhi.desktop"

remove_file() {
  local path="$1"
  if [[ -f "$path" ]]; then
    rm -- "$path"
    echo "Removed ${path}"
  else
    echo "Not found: ${path} (already removed?)"
  fi
}

main() {
  echo "Uninstalling stsuhi..."
  remove_file "$BIN_PATH"
  remove_file "$DESKTOP_FILE"
  if command -v update-desktop-database &> /dev/null; then
    update-desktop-database "$DESKTOP_DIR" 2> /dev/null || true
  fi
  echo ""
  echo "Your connection data in ~/.config/stsuhi/ was kept."
  echo "To remove it too, run: rm -rf ~/.config/stsuhi/"
}

main "$@"
