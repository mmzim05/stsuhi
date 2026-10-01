package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/theme"
)

type themeOption struct {
	label string
	value string // config.Theme value to store; "" = adaptive default
}

// ThemePicker lets the user choose stsuhi's color theme: the adaptive
// built-in default, Omarchy's live desktop theme, or one pinned Omarchy
// theme by name. The full Omarchy catalog is only offered when Omarchy is
// actually installed on this machine.
type ThemePicker struct {
	options  []themeOption
	selected int
}

// NewThemePicker builds the picker, pre-selecting whichever option matches
// the currently configured theme value.
func NewThemePicker(current string) ThemePicker {
	options := []themeOption{
		{label: "Adaptive (default)", value: ""},
	}
	if theme.Available() {
		options = append(options, themeOption{label: "Follow Omarchy (current desktop theme)", value: "omarchy"})
		for _, info := range theme.List() {
			options = append(options, themeOption{label: info.DisplayName, value: "omarchy:" + info.Slug})
		}
	}

	selected := 0
	for i, opt := range options {
		if opt.value == current {
			selected = i
			break
		}
	}

	return ThemePicker{options: options, selected: selected}
}

// Init initializes the theme picker
func (p ThemePicker) Init() tea.Cmd { return nil }

// Update updates the theme picker
func (p ThemePicker) Update(msg tea.Msg) (ThemePicker, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if p.selected > 0 {
				p.selected--
			}
		case "down", "j":
			if p.selected < len(p.options)-1 {
				p.selected++
			}
		}
	}
	return p, nil
}

// View renders the theme picker
func (p ThemePicker) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose Theme"))
	b.WriteString("\n\n")
	if !theme.Available() {
		b.WriteString(subtitleStyle.Render("Omarchy not found on this machine - only the adaptive default is available."))
		b.WriteString("\n\n")
	}
	for i, opt := range p.options {
		if i == p.selected {
			b.WriteString(labelStyle.Render("→ " + opt.label))
		} else {
			b.WriteString(subtitleStyle.Render("  " + opt.label))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ or j/k: navigate • enter: apply • esc: cancel"))
	return b.String()
}

// SelectedValue returns the config.Theme value for the highlighted option
func (p ThemePicker) SelectedValue() string {
	return p.options[p.selected].value
}

// ThemeLabel describes cfgTheme for display (e.g. in Settings), without
// requiring the caller to know the "omarchy:<slug>" encoding.
func ThemeLabel(cfgTheme string) string {
	switch {
	case cfgTheme == "":
		return "Adaptive (default)"
	case cfgTheme == "omarchy":
		return "Follow Omarchy (current)"
	case strings.HasPrefix(cfgTheme, "omarchy:"):
		return theme.DisplayName(strings.TrimPrefix(cfgTheme, "omarchy:"))
	default:
		return "Adaptive (default)"
	}
}
