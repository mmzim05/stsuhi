package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/onepassword"
)

// Row indices within the setup screen. Row 1 (item name) is only navigable
// when backend == config.BackendOnePassword.
const (
	rowBackend = iota
	rowItemName
	rowClearOnReboot
	rowClearOnLock
	rowClearOnIdle
	rowNeverClear
	rowCount
)

// AutoLoginSetup is the model for the auto-login setup screen
type AutoLoginSetup struct {
	rules          config.SecurityRules
	backend        string
	masterPassword string
	selectedRule   int
	idleHoursInput textinput.Model
	itemNameInput  textinput.Model
	err            string
}

// NewAutoLoginSetup creates a new auto-login setup screen. It prefills the
// backend and 1Password item name from cfg (used when reconfiguring an
// already-enabled auto-login), and keeps masterPassword around so the
// 1Password item can be validated against it before saving.
func NewAutoLoginSetup(cfg *config.Config, masterPassword string) AutoLoginSetup {
	ti := textinput.New()
	ti.Placeholder = "24"
	ti.CharLimit = 3
	ti.Width = 10
	ti.SetValue("24")

	itemName := textinput.New()
	itemName.Placeholder = onepassword.DefaultItemName
	itemName.CharLimit = 100
	itemName.Width = 30
	if cfg.AutoLogin.OnePasswordItemName != "" {
		itemName.SetValue(cfg.AutoLogin.OnePasswordItemName)
	} else {
		itemName.SetValue(onepassword.DefaultItemName)
	}

	return AutoLoginSetup{
		rules: config.SecurityRules{
			ClearOnReboot:    true,
			ClearOnLock:      false,
			ClearOnIdle:      false,
			IdleTimeoutHours: 24,
			NeverClear:       false,
		},
		backend:        cfg.AutoLogin.EffectiveBackend(),
		masterPassword: masterPassword,
		selectedRule:   rowBackend,
		idleHoursInput: ti,
		itemNameInput:  itemName,
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
			a.navigateUp()
			return a, nil
		case "down", "j":
			a.navigateDown()
			return a, nil
		case " ":
			// Toggle selected checkbox / cycle backend
			a.toggleRow()
			return a, nil
		default:
			// Forward typing to whichever input is active
			if a.selectedRule == rowItemName && a.backend == config.BackendOnePassword {
				var cmd tea.Cmd
				a.itemNameInput, cmd = a.itemNameInput.Update(msg)
				return a, cmd
			}
			if a.selectedRule == rowClearOnIdle {
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

// itemNameRowActive reports whether the item-name row is currently navigable.
func (a *AutoLoginSetup) itemNameRowActive() bool {
	return a.backend == config.BackendOnePassword
}

// navigateUp moves selection up, skipping the item-name row when inactive
func (a *AutoLoginSetup) navigateUp() {
	for {
		if a.selectedRule <= rowBackend {
			return
		}
		a.selectedRule--
		if a.selectedRule != rowItemName || a.itemNameRowActive() {
			return
		}
	}
}

// navigateDown moves selection down, skipping the item-name row when inactive
func (a *AutoLoginSetup) navigateDown() {
	for {
		if a.selectedRule >= rowCount-1 {
			return
		}
		a.selectedRule++
		if a.selectedRule != rowItemName || a.itemNameRowActive() {
			return
		}
	}
}

// toggleRow handles space on whichever row is selected
func (a *AutoLoginSetup) toggleRow() {
	switch a.selectedRule {
	case rowBackend:
		if a.backend == config.BackendOnePassword {
			a.backend = config.BackendKeyring
		} else {
			a.backend = config.BackendOnePassword
		}
	case rowClearOnReboot:
		a.rules.ClearOnReboot = !a.rules.ClearOnReboot
	case rowClearOnLock:
		a.rules.ClearOnLock = !a.rules.ClearOnLock
	case rowClearOnIdle:
		a.rules.ClearOnIdle = !a.rules.ClearOnIdle
	case rowNeverClear:
		a.rules.NeverClear = !a.rules.NeverClear
		// If "never clear" is selected, uncheck other rules
		if a.rules.NeverClear {
			a.rules.ClearOnReboot = false
			a.rules.ClearOnLock = false
			a.rules.ClearOnIdle = false
		}
	}

	// If any other rule is selected, uncheck "never clear"
	if a.selectedRule != rowNeverClear && (a.rules.ClearOnReboot || a.rules.ClearOnLock || a.rules.ClearOnIdle) {
		a.rules.NeverClear = false
	}
}

// View renders the auto-login setup screen
func (a AutoLoginSetup) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Auto-Login Security"))
	b.WriteString("\n\n")

	// Unlock method row
	b.WriteString(subtitleStyle.Render("Unlock method:"))
	b.WriteString("\n")
	backendLabel := "OS Keyring"
	if a.backend == config.BackendOnePassword {
		backendLabel = "1Password"
	}
	prefix := "  "
	if a.selectedRule == rowBackend {
		prefix = "→ "
		b.WriteString(labelStyle.Render(prefix + "[ " + backendLabel + " ]  (space to switch)"))
	} else {
		b.WriteString(prefix + "[ " + backendLabel + " ]")
	}
	b.WriteString("\n")

	if a.backend == config.BackendOnePassword {
		prefix = "  "
		if a.selectedRule == rowItemName {
			prefix = "→ "
			b.WriteString(labelStyle.Render(prefix + "1Password item name: "))
		} else {
			b.WriteString(subtitleStyle.Render(prefix + "1Password item name: "))
		}
		b.WriteString(a.itemNameInput.View())
		b.WriteString("\n")
		b.WriteString(mutedTextStyle.Render("    Looks for a field labeled \"password\" on that item."))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(subtitleStyle.Render("When should auto-login be cleared?"))
	b.WriteString("\n\n")

	// Rule options
	rules := []struct {
		row     int
		label   string
		checked bool
		desc    string
	}{
		{rowClearOnReboot, "After system reboot", a.rules.ClearOnReboot, "Re-authenticate after computer restart"},
		{rowClearOnLock, "After user logout/lock screen", a.rules.ClearOnLock, "Re-authenticate after locking or logging out"},
		{rowClearOnIdle, "After idle timeout", a.rules.ClearOnIdle, "Re-authenticate after inactivity"},
		{rowNeverClear, "Never - always persist", a.rules.NeverClear, "Auto-login never expires (least secure)"},
	}

	for _, rule := range rules {
		checkbox := "[ ]"
		if rule.checked {
			checkbox = "[✓]"
		}

		prefix := "  "
		if rule.row == a.selectedRule {
			prefix = "→ "
			b.WriteString(labelStyle.Render(prefix + checkbox + " " + rule.label))
		} else {
			b.WriteString(subtitleStyle.Render(prefix + checkbox + " " + rule.label))
		}

		b.WriteString("\n")
		if rule.row != rowNeverClear {
			b.WriteString(mutedTextStyle.Render("    " + rule.desc))
			b.WriteString("\n")
		}

		// Show idle timeout input if idle rule is selected
		if rule.row == rowClearOnIdle && a.selectedRule == rowClearOnIdle {
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

// GetBackend returns the selected auto-login backend
func (a AutoLoginSetup) GetBackend() string {
	return a.backend
}

// GetItemName returns the entered 1Password item name
func (a AutoLoginSetup) GetItemName() string {
	if name := strings.TrimSpace(a.itemNameInput.Value()); name != "" {
		return name
	}
	return onepassword.DefaultItemName
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

	if a.backend == config.BackendOnePassword {
		itemName := a.GetItemName()
		if err := onepassword.ValidateItemName(itemName); err != nil {
			return err
		}
		if !onepassword.IsAvailable() {
			return fmt.Errorf("1password CLI (op) not found in PATH")
		}
		password, err := onepassword.GetPassword(itemName)
		if err != nil {
			return fmt.Errorf("couldn't read item %q: %w", itemName, err)
		}
		if password != a.masterPassword {
			return fmt.Errorf("1password item %q does not match your stsuhi master password", itemName)
		}
	}

	return nil
}
