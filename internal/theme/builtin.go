package theme

import "github.com/charmbracelet/lipgloss"

// builtinDark and builtinLight are the fallback palette used when no
// Omarchy theme is configured or available. Deliberately not a copy of any
// named preset (Catppuccin, Dracula, etc.) so it never gets confused with
// one of the real Omarchy themes offered alongside it.
var builtinDark = Theme{
	Name: "Default Dark", Mode: "dark",
	Background: "#101418", DarkBackground: "#0b0e11", LighterBackground: "#1c2228",
	Foreground: "#d8dee9", MutedForeground: "#7a8797", BrightForeground: "#ffffff",
	Accent: "#5ec4ff", Selection: "#22303c", Muted: "#3a4552",
	Red: "#ff6b6b", Orange: "#ffa94d", Yellow: "#ffd43b", Green: "#69db7c",
	Cyan: "#66d9e8", Blue: "#5ec4ff", Magenta: "#cc99ff",
}

var builtinLight = Theme{
	Name: "Default Light", Mode: "light",
	Background: "#f7f8fa", DarkBackground: "#eef0f3", LighterBackground: "#e4e7eb",
	Foreground: "#24292f", MutedForeground: "#57606a", BrightForeground: "#000000",
	Accent: "#0969da", Selection: "#ddf4ff", Muted: "#8b949e",
	Red: "#cf222e", Orange: "#bc4c00", Yellow: "#9a6700", Green: "#1a7f37",
	Cyan: "#0598bc", Blue: "#0969da", Magenta: "#8250df",
}

// Default returns the built-in theme, picking dark or light to match the
// terminal's detected background. Used whenever no Omarchy theme is
// configured, or Omarchy isn't available on the machine at all.
func Default() Theme {
	if lipgloss.HasDarkBackground() {
		return builtinDark
	}
	return builtinLight
}
