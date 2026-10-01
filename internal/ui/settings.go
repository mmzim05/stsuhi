package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/config"
)

// SettingsOption represents a settings menu option
type SettingsOption int

const (
	SettingsEnable SettingsOption = iota
	SettingsReconfigure
	SettingsDisable
	SettingsTheme
	SettingsBack
)

// Settings is the model for the settings screen
type Settings struct {
	config         *config.Config
	selectedOption SettingsOption
}

// NewSettings creates a new settings screen
func NewSettings(cfg *config.Config) Settings {
	// Start at first available option
	selected := SettingsEnable
	if cfg.AutoLogin.Enabled {
		// If auto-login is enabled, skip the enable option
		selected = SettingsReconfigure
	}
	return Settings{
		config:         cfg,
		selectedOption: selected,
	}
}

// Init initializes the settings screen
func (s Settings) Init() tea.Cmd {
	return nil
}

// Update updates the settings screen
func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			s.navigateUp()
		case "down", "j":
			s.navigateDown()
		}
	}

	return s, nil
}

// navigateUp moves selection up, skipping disabled options
func (s *Settings) navigateUp() {
	for {
		if s.selectedOption <= 0 {
			return
		}
		s.selectedOption--
		if s.isOptionEnabled(s.selectedOption) {
			return
		}
	}
}

// navigateDown moves selection down, skipping disabled options
func (s *Settings) navigateDown() {
	for {
		if s.selectedOption >= SettingsBack {
			return
		}
		s.selectedOption++
		if s.isOptionEnabled(s.selectedOption) {
			return
		}
	}
}

// isOptionEnabled checks if an option should be enabled
func (s *Settings) isOptionEnabled(opt SettingsOption) bool {
	switch opt {
	case SettingsEnable:
		return !s.config.AutoLogin.Enabled
	case SettingsReconfigure, SettingsDisable:
		return s.config.AutoLogin.Enabled
	case SettingsTheme, SettingsBack:
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

	// Auto-login status
	b.WriteString(labelStyle.Render("Auto-Login Status: "))
	if s.config.AutoLogin.Enabled {
		backendLabel := "OS Keyring"
		if s.config.AutoLogin.EffectiveBackend() == config.BackendOnePassword {
			backendLabel = "1Password"
		}
		b.WriteString(successStyle.Render(fmt.Sprintf("Enabled (%s)", backendLabel)))
	} else {
		b.WriteString(subtitleStyle.Render("Disabled"))
	}
	b.WriteString("\n\n")

	// If auto-login is enabled, show current rules
	if s.config.AutoLogin.Enabled {
		b.WriteString(subtitleStyle.Render("Current Security Rules:"))
		b.WriteString("\n")

		rules := s.config.AutoLogin.SecurityRules
		if rules.NeverClear {
			b.WriteString("  • Never clear - always persist\n")
		} else {
			if rules.ClearOnReboot {
				b.WriteString("  • Clear after system reboot\n")
			}
			if rules.ClearOnLock {
				b.WriteString("  • Clear after logout/lock\n")
			}
			if rules.ClearOnIdle {
				b.WriteString(fmt.Sprintf("  • Clear after %d hours idle\n", rules.IdleTimeoutHours))
			}
		}
		b.WriteString("\n")
	}

	// Theme
	b.WriteString(labelStyle.Render("Theme: "))
	b.WriteString(subtitleStyle.Render(ThemeLabel(s.config.Theme)))
	b.WriteString("\n\n")

	// Menu options
	b.WriteString(subtitleStyle.Render("Options:"))
	b.WriteString("\n")

	options := []struct {
		opt     SettingsOption
		label   string
		enabled bool
	}{
		{SettingsEnable, "Enable auto-login", !s.config.AutoLogin.Enabled},
		{SettingsReconfigure, "Reconfigure auto-login", s.config.AutoLogin.Enabled},
		{SettingsDisable, "Disable auto-login", s.config.AutoLogin.Enabled},
		{SettingsTheme, "Change theme", true},
		{SettingsBack, "Back to connections", true},
	}

	for _, opt := range options {
		if !opt.enabled {
			continue // Don't show disabled options
		}

		prefix := "  "
		if opt.opt == s.selectedOption {
			prefix = "→ "
			b.WriteString(labelStyle.Render(prefix + opt.label))
		} else {
			b.WriteString(prefix + opt.label)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ or j/k: navigate • enter: select • esc: back"))

	return b.String()
}

// GetSelectedOption returns the currently selected option
func (s Settings) GetSelectedOption() SettingsOption {
	return s.selectedOption
}
