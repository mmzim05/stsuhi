#!/usr/bin/env bash
set -euo pipefail

readonly REPO="mmzim05/stsuhi"
readonly INSTALL_DIR="${HOME}/.local/bin"
readonly DESKTOP_DIR="${HOME}/.local/share/applications"
readonly BIN_PATH="${INSTALL_DIR}/stsuhi"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR="$script_dir"

die() {
  echo "Error: $*" >&2
  exit 1
}

# Prints the release asset suffix for this machine, e.g. "linux-amd64".
detect_platform() {
  local os
  local arch
  os="$(uname -s)"
  arch="$(uname -m)"
  if [[ "$os" != "Linux" ]]; then
    die "stsuhi only supports Linux (found ${os})."
  fi
  case "$arch" in
    x86_64 | amd64) echo "linux-amd64" ;;
    aarch64 | arm64) echo "linux-arm64" ;;
    *) die "no prebuilt binary for ${arch}; install Go and re-run." ;;
  esac
}

build_from_source() {
  echo "Building stsuhi from source with $(go version)..."
  (cd "$SCRIPT_DIR" && go build -trimpath -ldflags "-s -w" \
    -o "$BIN_PATH" .)
}

download_release() {
  local platform
  local url
  local tmp_file
  platform="$(detect_platform)"
  url="https://github.com/${REPO}/releases/latest/download/stsuhi-${platform}"
  if ! command -v curl &> /dev/null; then
    die "need either Go (to build) or curl (to download)."
  fi
  echo "Go not found; downloading prebuilt binary for ${platform}..."
  tmp_file="$(mktemp)"
  if ! curl -fsSL -o "$tmp_file" "$url"; then
    rm -f "$tmp_file"
    die "download failed: ${url}"
  fi
  install -m 0755 "$tmp_file" "$BIN_PATH"
  rm -f "$tmp_file"
}

install_binary() {
  mkdir -p "$INSTALL_DIR"
  if command -v go &> /dev/null && [[ -f "${SCRIPT_DIR}/go.mod" ]]; then
    build_from_source
  else
    download_release
  fi
  chmod 0755 "$BIN_PATH"
}

# Adds a launcher entry only on machines with a desktop session.
install_desktop_entry() {
  local exec_line
  local terminal
  if [[ -z "${XDG_CURRENT_DESKTOP:-}" && -z "${DISPLAY:-}" \
    && -z "${WAYLAND_DISPLAY:-}" ]]; then
    return
  fi
  # xdg-terminal-exec with the TUI.tile app-id makes Omarchy/Hyprland
  # tile it like other TUI apps; elsewhere the launcher opens a terminal.
  if command -v xdg-terminal-exec &> /dev/null; then
    exec_line="xdg-terminal-exec --app-id=TUI.tile -e ${BIN_PATH}"
    terminal="false"
  else
    exec_line="$BIN_PATH"
    terminal="true"
  fi
  mkdir -p "$DESKTOP_DIR"
  cat > "${DESKTOP_DIR}/stsuhi.desktop" << EOF
[Desktop Entry]
Version=1.0
Name=stsuhi
Comment=TUI SSH connection manager
Exec=${exec_line}
Terminal=${terminal}
Type=Application
Icon=utilities-terminal
StartupNotify=true
Categories=Network;RemoteAccess;
Keywords=ssh;remote;terminal;connection;
EOF
  if command -v update-desktop-database &> /dev/null; then
    update-desktop-database "$DESKTOP_DIR" || true
  fi
  echo "Desktop entry: ${DESKTOP_DIR}/stsuhi.desktop"
}

print_path_hint() {
  if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
    echo ""
    echo "Note: ${INSTALL_DIR} is not in your PATH. Add this to your shell rc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
}

print_onepassword_hint() {
  if command -v op &> /dev/null; then
    return
  fi
  echo ""
  echo "Optional: install the 1Password CLI (op) to pull credentials from"
  echo "1Password. See https://developer.1password.com/docs/cli/get-started/"
}

main() {
  echo "Installing stsuhi..."
  install_binary
  echo "Installed: ${BIN_PATH}"
  install_desktop_entry
  print_path_hint
  print_onepassword_hint
  echo ""
  echo "Done. Run 'stsuhi' to start."
}

main "$@"
