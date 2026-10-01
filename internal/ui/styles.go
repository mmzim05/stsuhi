package ui

import (
	"github.com/charmbracelet/lipgloss"
	"stsuhi/internal/theme"
)

// active is the currently applied theme. Package-level so every style
// below can be rebuilt from it without threading a Theme through every
// model - see SetTheme.
var active theme.Theme

var (
	titleStyle     lipgloss.Style
	subtitleStyle  lipgloss.Style
	mutedTextStyle lipgloss.Style
	labelStyle     lipgloss.Style
	errorStyle     lipgloss.Style
	successStyle   lipgloss.Style
	helpStyle      lipgloss.Style
	dialogBoxStyle lipgloss.Style

	// Connection list row styles (see connectionDelegate in list.go).
	listNormalTitleStyle   lipgloss.Style
	listNormalDescStyle    lipgloss.Style
	listSelectedTitleStyle lipgloss.Style
	listSelectedDescStyle  lipgloss.Style
)

func init() {
	SetTheme(theme.Default())
}

// SetTheme applies t as the active theme, rebuilding every style used
// across the TUI from it. Safe to call again at runtime - e.g. when the
// user picks a different theme in Settings - to switch live: every View()
// reads these package-level styles fresh on each render.
func SetTheme(t theme.Theme) {
	active = t

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.BrightForeground).
		Background(t.Accent).
		Padding(0, 1).
		MarginBottom(1)

	// Regular readable body text - descriptions, unfocused labels,
	// instructions. Deliberately NOT MutedForeground: that grey is reserved
	// for the help footer below, where being de-emphasized is the point.
	// Most of the app's text is body text like this, so it needs to read
	// as normal, not dim.
	subtitleStyle = lipgloss.NewStyle().Foreground(t.Foreground)
	mutedTextStyle = subtitleStyle

	labelStyle = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)

	errorStyle = lipgloss.NewStyle().Foreground(t.Red).Bold(true).MarginTop(1)
	successStyle = lipgloss.NewStyle().Foreground(t.Green).Bold(true).MarginTop(1)

	helpStyle = lipgloss.NewStyle().Foreground(t.MutedForeground).MarginTop(1)

	dialogBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 2).
		MarginTop(1)

	listNormalTitleStyle = lipgloss.NewStyle().Foreground(t.Foreground)
	listNormalDescStyle = lipgloss.NewStyle().Foreground(t.Foreground)
	listSelectedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.BrightForeground).Background(t.Selection)
	listSelectedDescStyle = lipgloss.NewStyle().Foreground(t.Accent).Background(t.Selection)
}
