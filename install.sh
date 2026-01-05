#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/stsuhi"
DESKTOP_FILE="$SCRIPT_DIR/stsuhi.desktop"
INSTALL_DIR="$HOME/.local/bin"
DESKTOP_DIR="$HOME/.local/share/applications"

echo "Installing Stsuhi SSH Manager..."

# Create directories if they don't exist
mkdir -p "$INSTALL_DIR"
mkdir -p "$DESKTOP_DIR"

# Copy binary
if [ ! -f "$BINARY" ]; then
    echo "Error: Binary file not found at $BINARY"
    exit 1
fi

echo "Copying binary to $INSTALL_DIR/stsuhi..."
cp "$BINARY" "$INSTALL_DIR/stsuhi"
chmod +x "$INSTALL_DIR/stsuhi"

# Update .desktop file with correct path and copy it
if [ ! -f "$DESKTOP_FILE" ]; then
    echo "Error: Desktop file not found at $DESKTOP_FILE"
    exit 1
fi

echo "Installing desktop entry..."
sed "s|Exec=.*|Exec=$INSTALL_DIR/stsuhi|" "$DESKTOP_FILE" > "$DESKTOP_DIR/stsuhi.desktop"
chmod +x "$DESKTOP_DIR/stsuhi.desktop"

# Update desktop database
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
echo "  - Launch it from your application menu"
echo "  - Run 'stsuhi' from the terminal (if ~/.local/bin is in your PATH)"
echo ""

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo "Note: $HOME/.local/bin is not in your PATH."
    echo "Add this line to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi
