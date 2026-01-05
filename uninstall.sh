#!/bin/bash

set -e

INSTALL_DIR="$HOME/.local/bin"
DESKTOP_DIR="$HOME/.local/share/applications"
BINARY="$INSTALL_DIR/stsuhi"
DESKTOP_FILE="$DESKTOP_DIR/stsuhi.desktop"

echo "Uninstalling Stsuhi SSH Manager..."

# Remove binary
if [ -f "$BINARY" ]; then
    echo "Removing binary from $BINARY..."
    rm "$BINARY"
else
    echo "Binary not found at $BINARY (already removed?)"
fi

# Remove desktop file
if [ -f "$DESKTOP_FILE" ]; then
    echo "Removing desktop entry from $DESKTOP_FILE..."
    rm "$DESKTOP_FILE"
else
    echo "Desktop file not found at $DESKTOP_FILE (already removed?)"
fi

# Update desktop database
if command -v update-desktop-database &> /dev/null; then
    update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
fi

echo ""
echo "✓ Uninstallation complete!"
echo ""
echo "Note: Your SSH connection data in ~/.config/stsuhi/ has been preserved."
echo "To completely remove all data, run:"
echo "  rm -rf ~/.config/stsuhi/"
