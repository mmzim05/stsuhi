// Package theme defines the semantic color palette stsuhi's UI is built
// from, and where that palette comes from: a built-in light/dark-adaptive
// default, or an Omarchy desktop theme (see omarchy.go).
package theme

import "github.com/charmbracelet/lipgloss"

// Theme is the full set of semantic colors the ui package's styles are
// built from. Every field is a concrete color - "adaptive" (light/dark
// terminal detection) happens once, in Default(), by picking which of two
// concrete built-in themes to return, not per-field.
type Theme struct {
	Name string
	Mode string // "dark" or "light"

	Background        lipgloss.Color
	DarkBackground    lipgloss.Color
	LighterBackground lipgloss.Color

	Foreground       lipgloss.Color
	MutedForeground  lipgloss.Color
	BrightForeground lipgloss.Color

	Accent    lipgloss.Color // primary interactive/highlight color
	Selection lipgloss.Color // selected-row background
	Muted     lipgloss.Color // borders, dim chrome

	Red, Orange, Yellow, Green, Cyan, Blue, Magenta lipgloss.Color
}
