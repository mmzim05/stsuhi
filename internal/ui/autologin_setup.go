package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/config"
)

// AutoLoginSetup is the model for the auto-login setup screen
type AutoLoginSetup struct {
	rules          config.SecurityRules
	selectedRule   int
	idleHoursInput textinput.Model
	err            string
}

// NewAutoLoginSetup creates a new auto-login setup screen
func NewAutoLoginSetup() AutoLoginSetup {
	ti := textinput.New()
	ti.Placeholder = "24"
	ti.CharLimit = 3
	ti.Width = 10
	ti.SetValue("24")

	return AutoLoginSetup{
		rules: config.SecurityRules{
			ClearOnReboot:    true,
			ClearOnLock:      false,
			ClearOnIdle:      false,
			IdleTimeoutHours: 24,
			NeverClear:       false,
		},
		selectedRule:   0,
		idleHoursInput: ti,
	}
}

// Init initializes the auto-login setup
func (a AutoLoginSetup) Init() tea.Cmd {
	return nil
}

// Update updates the auto-login setup
func (a AutoLoginSetup) Update(msg tea.Msg) (AutoLoginSetup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if a.selectedRule > 0 {
				a.selectedRule--
			}
			return a, nil
		case "down", "j":
			if a.selectedRule < 4 {
				a.selectedRule++
			}
			return a, nil
		case " ":
			// Toggle selected checkbox
			a.toggleRule()
			return a, nil
		default:
			// Update idle hours input if that's selected
			if a.selectedRule == 2 {
				var cmd tea.Cmd
				a.idleHoursInput, cmd = a.idleHoursInput.Update(msg)
				// Parse and update hours
				if val := a.idleHoursInput.Value(); val != "" {
					if hours, err := strconv.Atoi(val); err == nil && hours > 0 {
						a.rules.IdleTimeoutHours = hours
					}
				}
				return a, cmd
			}
		}
	}

	return a, nil
}

// toggleRule toggles the selected security rule
func (a *AutoLoginSetup) toggleRule() {
	switch a.selectedRule {
	case 0:
		a.rules.ClearOnReboot = !a.rules.ClearOnReboot
	case 1:
		a.rules.ClearOnLock = !a.rules.ClearOnLock
	case 2:
		a.rules.ClearOnIdle = !a.rules.ClearOnIdle
	case 3:
		a.rules.NeverClear = !a.rules.NeverClear
		// If "never clear" is selected, uncheck other rules
		if a.rules.NeverClear {
			a.rules.ClearOnReboot = false
			a.rules.ClearOnLock = false
			a.rules.ClearOnIdle = false
		}
	}

	// If any other rule is selected, uncheck "never clear"
	if a.selectedRule != 3 && (a.rules.ClearOnReboot || a.rules.ClearOnLock || a.rules.ClearOnIdle) {
		a.rules.NeverClear = false
	}
}

// View renders the auto-login setup screen
func (a AutoLoginSetup) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Auto-Login Security"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("When should auto-login be cleared?"))
	b.WriteString("\n\n")

	// Rule options
	rules := []struct {
		label   string
		checked bool
		desc    string
	}{
		{"After system reboot", a.rules.ClearOnReboot, "Re-authenticate after computer restart"},
		{"After user logout/lock screen", a.rules.ClearOnLock, "Re-authenticate after locking or logging out"},
		{"After idle timeout", a.rules.ClearOnIdle, "Re-authenticate after inactivity"},
		{"Never - always persist", a.rules.NeverClear, "Auto-login never expires (least secure)"},
	}

	for i, rule := range rules {
		checkbox := "[ ]"
		if rule.checked {
			checkbox = "[✓]"
		}

		prefix := "  "
		if i == a.selectedRule {
			prefix = "→ "
			b.WriteString(labelStyle.Render(prefix + checkbox + " " + rule.label))
		} else {
			b.WriteString(subtitleStyle.Render(prefix + checkbox + " " + rule.label))
		}

		b.WriteString("\n")
		if i < 3 {
			b.WriteString(mutedTextStyle.Render("    " + rule.desc))
			b.WriteString("\n")
		}

		// Show idle timeout input if idle rule is selected
		if i == 2 && a.selectedRule == 2 {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("    Timeout hours: "))
			b.WriteString(a.idleHoursInput.View())
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("↑/↓: navigate • space: toggle • enter: save • esc: back"))

	return b.String()
}

// GetRules returns the configured security rules
func (a AutoLoginSetup) GetRules() config.SecurityRules {
	return a.rules
}

// SetError sets an error message
func (a *AutoLoginSetup) SetError(err string) {
	a.err = err
}

// Validate validates the configuration
func (a AutoLoginSetup) Validate() error {
	// At least one rule must be selected
	if !a.rules.ClearOnReboot && !a.rules.ClearOnLock && !a.rules.ClearOnIdle && !a.rules.NeverClear {
		return fmt.Errorf("please select at least one security rule")
	}

	// If idle timeout is selected, validate hours
	if a.rules.ClearOnIdle && a.rules.IdleTimeoutHours <= 0 {
		return fmt.Errorf("idle timeout hours must be greater than 0")
	}

	return nil
}

// Helper style for muted text
var mutedTextStyle = subtitleStyle
