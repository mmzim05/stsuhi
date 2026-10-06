#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESKTOP_FILE="$SCRIPT_DIR/stsuhi.desktop"
INSTALL_DIR="$HOME/.local/bin"
DESKTOP_DIR="$HOME/.local/share/applications"

echo "Installing Stsuhi SSH Manager..."

# Go toolchain required to build - not something this script installs itself,
# since doing so would mean picking a package manager / version on the
# user's behalf.
if ! command -v go &> /dev/null; then
    echo "Error: Go toolchain not found."
    echo "Install it first, e.g.: sudo pacman -S go"
    exit 1
fi

mkdir -p "$INSTALL_DIR"
mkdir -p "$DESKTOP_DIR"

echo "Fetching Go module dependencies..."
( cd "$SCRIPT_DIR" && go mod download )

echo "Building stsuhi..."
( cd "$SCRIPT_DIR" && go build -o "$INSTALL_DIR/stsuhi" . )
chmod +x "$INSTALL_DIR/stsuhi"

# Install desktop entry, pointed at the real install path. Uses
# xdg-terminal-exec with Omarchy's TUI.tile app-id convention (same as e.g.
# Docker's and Disk Usage's desktop entries) so Hyprland's window rules
# treat it like the other TUI apps, and so it shows up searchable by name
# in Omarchy's app launcher.
if [ ! -f "$DESKTOP_FILE" ]; then
    echo "Error: Desktop file not found at $DESKTOP_FILE"
    exit 1
fi

echo "Installing desktop entry..."
sed "s|__STSUHI_BIN__|$INSTALL_DIR/stsuhi|" "$DESKTOP_FILE" > "$DESKTOP_DIR/stsuhi.desktop"
chmod +x "$DESKTOP_DIR/stsuhi.desktop"

# Update desktop database so the launcher picks up the new entry immediately
if command -v update-desktop-database &> /dev/null; then
    update-desktop-database "$DESKTOP_DIR"
fi

echo ""
echo "✓ Installation complete!"
echo ""
echo "The application has been installed to: $INSTALL_DIR/stsuhi"
echo "Desktop entry created at: $DESKTOP_DIR/stsuhi.desktop"
echo ""
echo "You can now:"
echo "  - Search 'stsuhi' in your Omarchy app launcher"
echo "  - Run 'stsuhi' from the terminal (if ~/.local/bin is in your PATH)"
echo ""

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo "Note: $HOME/.local/bin is not in your PATH."
    echo "Add this line to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# --- 1Password integration (optional) ---
# stsuhi works fully without this - local passwords/keys/SSH-agent
# connections don't need it. This only matters if you want a connection
# to fetch its username/password (and optionally address/port) from
# 1Password instead of storing them locally.
echo ""
echo "1Password integration (optional)"
if command -v op &> /dev/null; then
    echo "  1Password CLI (op) is already installed."
elif command -v pacman &> /dev/null; then
    read -p "  Install 1Password + 1Password CLI now via pacman? [y/N] " -n 1 -r REPLY
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        sudo pacman -S --needed 1password 1password-cli
    else
        echo "  Skipped. Install later with: sudo pacman -S 1password 1password-cli"
    fi
else
    echo "  'op' not found and no pacman to install it with - skipping."
    echo "  Get it from https://1password.com/downloads/command-line/ if you want it."
fi

cat <<'GUIDE'

  Whether or not you just installed it, two manual steps are still
  needed before stsuhi can actually reach 1Password:

    1. Add yourself to the "onepassword" group (lets the CLI/SSH-agent
       socket under ~/.1password/ be read by your user), then re-login
       or reboot for it to take effect:
         sudo usermod -aG onepassword $USER

    2. In the 1Password desktop app: Settings -> Developer, enable
       "Integrate with 1Password CLI".

  Verify with: op vault list
  (if that fails with "account is not signed in" or hangs, re-check
  step 2 - that's an op/1Password state issue, not stsuhi's)

  Then, in stsuhi: Settings -> "1Password setup instructions" has the
  full walkthrough, including how to format a connection's 1Password
  item so its username/password (and optionally address/port) get used.
GUIDE
