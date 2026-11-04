package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/config"
)

// SettingsOption represents a settings menu option
type SettingsOption int

// SettingsOption values, in menu order.
const (
	SettingsOnePasswordEnable SettingsOption = iota
	SettingsOnePasswordChange
	SettingsOnePasswordDisable
	SettingsTheme
	SettingsOnePasswordHelp
	SettingsBack
)

// Settings is the model for the settings screen
type Settings struct {
	config           *config.Config
	keyringAvailable bool
	selectedOption   SettingsOption
}

// NewSettings creates a new settings screen. The 1Password unlock options
// only appear without a keyring: with one, the master key lives there.
func NewSettings(cfg *config.Config, keyringAvailable bool) Settings {
	s := Settings{config: cfg, keyringAvailable: keyringAvailable}
	for s.selectedOption < SettingsBack &&
		!s.isOptionEnabled(s.selectedOption) {
		s.selectedOption++
	}
	return s
}

// Init initializes the settings screen
func (s Settings) Init() tea.Cmd {
	return nil
}

// Update moves the selection
func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			s.navigateUp()
		case "down", "j":
			s.navigateDown()
		}
	}
	return s, nil
}

// navigateUp moves selection up, skipping hidden options
func (s *Settings) navigateUp() {
	for opt := s.selectedOption - 1; opt >= 0; opt-- {
		if s.isOptionEnabled(opt) {
			s.selectedOption = opt
			return
		}
	}
}

// navigateDown moves selection down, skipping hidden options
func (s *Settings) navigateDown() {
	for opt := s.selectedOption + 1; opt <= SettingsBack; opt++ {
		if s.isOptionEnabled(opt) {
			s.selectedOption = opt
			return
		}
	}
}

// isOptionEnabled reports whether opt is shown
func (s *Settings) isOptionEnabled(opt SettingsOption) bool {
	usesOnePassword := s.config.Unlock.UsesOnePassword()
	switch opt {
	case SettingsOnePasswordEnable:
		return !s.keyringAvailable && !usesOnePassword
	case SettingsOnePasswordChange, SettingsOnePasswordDisable:
		return !s.keyringAvailable && usesOnePassword
	case SettingsTheme, SettingsOnePasswordHelp, SettingsBack:
		return true
	default:
		return false
	}
}

// View renders the settings screen
func (s Settings) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Master key: "))
	switch {
	case s.keyringAvailable:
		b.WriteString(successStyle.Render("stored in the system keyring"))
	case s.config.Unlock.UsesOnePassword():
		b.WriteString(successStyle.Render("read from 1Password item " +
			s.config.Unlock.OnePasswordItemName))
	default:
		b.WriteString(subtitleStyle.Render(
			"no keyring found, asked for when needed"))
	}
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Theme: "))
	b.WriteString(subtitleStyle.Render(ThemeLabel(s.config.Theme)))
	b.WriteString("\n\n")

	b.WriteString(subtitleStyle.Render("Options:"))
	b.WriteString("\n")

	options := []struct {
		opt   SettingsOption
		label string
	}{
		{SettingsOnePasswordEnable, "Unlock with 1Password"},
		{SettingsOnePasswordChange, "Change 1Password item"},
		{SettingsOnePasswordDisable, "Stop unlocking with 1Password"},
		{SettingsTheme, "Change theme"},
		{SettingsOnePasswordHelp, "1Password setup instructions"},
		{SettingsBack, "Back to connections"},
	}
	for _, opt := range options {
		if !s.isOptionEnabled(opt.opt) {
			continue
		}
		if opt.opt == s.selectedOption {
			b.WriteString(labelStyle.Render("→ " + opt.label))
		} else {
			b.WriteString("  " + opt.label)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(
		"↑/↓ or j/k: navigate • enter: select • esc: back"))
	return b.String()
}

// GetSelectedOption returns the currently selected option
func (s Settings) GetSelectedOption() SettingsOption {
	return s.selectedOption
}
