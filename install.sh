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
