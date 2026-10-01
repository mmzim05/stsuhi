package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/models"
)

const (
	fieldServerName = iota
	fieldUser
	fieldAddress
	fieldPort
	fieldPassword
	fieldPasskeyPath
	fieldUseSSHAgent
	fieldSSHAgentSocket
	fieldUseOnePassword
	fieldOnePasswordVault
	fieldOnePasswordItem
	fieldCount
)

// ConnectionForm is a model for the add/edit connection form
type ConnectionForm struct {
	inputs          []textinput.Model
	focusedField    int
	err             string
	isEdit          bool
	originalConn    *models.Connection
	useSSHAgent     bool
	useOnePassword  bool
	onePasswordMode bool
}

// NewConnectionForm creates a new connection form. onePasswordMode restricts
// the form to server name/address/port plus a 1Password item reference -
// used app-wide when config.OnePasswordMode is on, since there is no master
// password to encrypt a locally-stored username/password/key with.
func NewConnectionForm(conn *models.Connection, onePasswordMode bool) ConnectionForm {
	inputs := make([]textinput.Model, fieldCount)

	// Server Name
	inputs[fieldServerName] = textinput.New()
	inputs[fieldServerName].Placeholder = "My Server"
	inputs[fieldServerName].Focus()
	inputs[fieldServerName].CharLimit = 50

	// User
	inputs[fieldUser] = textinput.New()
	inputs[fieldUser].Placeholder = "root"
	inputs[fieldUser].CharLimit = 32

	// Address
	inputs[fieldAddress] = textinput.New()
	inputs[fieldAddress].Placeholder = "192.168.1.1 or example.com"
	inputs[fieldAddress].CharLimit = 253

	// Port
	inputs[fieldPort] = textinput.New()
	inputs[fieldPort].Placeholder = "22"
	inputs[fieldPort].CharLimit = 5

	// Password
	inputs[fieldPassword] = textinput.New()
	inputs[fieldPassword].Placeholder = "password (optional if using key)"
	inputs[fieldPassword].EchoMode = textinput.EchoPassword
	inputs[fieldPassword].EchoCharacter = '•'

	// Passkey Path
	inputs[fieldPasskeyPath] = textinput.New()
	inputs[fieldPasskeyPath].Placeholder = "/home/user/.ssh/id_rsa (optional)"
	inputs[fieldPasskeyPath].CharLimit = 256

	// SSH Agent Socket (only used when useSSHAgent is toggled on)
	inputs[fieldSSHAgentSocket] = textinput.New()
	inputs[fieldSSHAgentSocket].Placeholder = "~/.1password/agent.sock (optional, defaults to SSH_AUTH_SOCK)"
	inputs[fieldSSHAgentSocket].CharLimit = 256

	// 1Password Vault (only used when useOnePassword is toggled on)
	inputs[fieldOnePasswordVault] = textinput.New()
	inputs[fieldOnePasswordVault].Placeholder = "Private (optional, searches all vaults if blank)"
	inputs[fieldOnePasswordVault].CharLimit = 100

	// 1Password Item (only used when useOnePassword is toggled on)
	inputs[fieldOnePasswordItem] = textinput.New()
	inputs[fieldOnePasswordItem].Placeholder = "item name storing username + password"
	inputs[fieldOnePasswordItem].CharLimit = 100

	form := ConnectionForm{
		inputs:          inputs,
		focusedField:    0,
		isEdit:          conn != nil,
		onePasswordMode: onePasswordMode,
		useOnePassword:  onePasswordMode,
	}

	// Populate fields if editing
	if conn != nil {
		form.originalConn = conn
		inputs[fieldServerName].SetValue(conn.ServerName)
		inputs[fieldUser].SetValue(conn.User)
		inputs[fieldAddress].SetValue(conn.Address)
		inputs[fieldPort].SetValue(strconv.Itoa(conn.Port))
		inputs[fieldPassword].SetValue(conn.Password)
		inputs[fieldPasskeyPath].SetValue(conn.PasskeyPath)
		inputs[fieldSSHAgentSocket].SetValue(conn.SSHAgentSocket)
		inputs[fieldOnePasswordVault].SetValue(conn.OnePasswordVault)
		inputs[fieldOnePasswordItem].SetValue(conn.OnePasswordItem)
		form.useSSHAgent = conn.UseSSHAgent
		if !onePasswordMode {
			form.useOnePassword = conn.UseOnePassword
		}
	} else {
		inputs[fieldPort].SetValue("22")
	}

	return form
}

// Init initializes the form
func (f ConnectionForm) Init() tea.Cmd {
	return textinput.Blink
}

// Update updates the form
func (f ConnectionForm) Update(msg tea.Msg) (ConnectionForm, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			f.nextField()
			return f, nil
		case "shift+tab", "up":
			f.prevField()
			return f, nil
		case " ":
			switch f.focusedField {
			case fieldUseSSHAgent:
				f.useSSHAgent = !f.useSSHAgent
				return f, nil
			case fieldUseOnePassword:
				f.useOnePassword = !f.useOnePassword
				return f, nil
			}
		}
	}

	// Update focused input (toggle fields are checkboxes, not text inputs)
	if !isToggleField(f.focusedField) {
		var cmd tea.Cmd
		f.inputs[f.focusedField], cmd = f.inputs[f.focusedField].Update(msg)
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

// View renders the form
func (f ConnectionForm) View() string {
	var b strings.Builder

	if f.isEdit {
		b.WriteString(titleStyle.Render("Edit Connection"))
	} else {
		b.WriteString(titleStyle.Render("Add New Connection"))
	}
	b.WriteString("\n\n")

	if f.onePasswordMode {
		b.WriteString(mutedTextStyle.Render("1Password mode: username/password come from 1Password, not stored here."))
		b.WriteString("\n\n")

		f.renderField(&b, fieldServerName, "Server Name:")
		b.WriteString("\n\n")
		f.renderField(&b, fieldAddress, "Address:")
		b.WriteString("\n\n")
		f.renderField(&b, fieldPort, "Port:")
		b.WriteString("\n\n")
		f.renderField(&b, fieldOnePasswordVault, "1Password Vault:")
		b.WriteString("\n\n")
		f.renderField(&b, fieldOnePasswordItem, "1Password Item:")
		b.WriteString("\n")
		b.WriteString(mutedTextStyle.Render("    Looks up \"username\" and \"password\" fields on that item at connect time."))
		b.WriteString("\n\n")
	} else {
		labels := []string{
			"Server Name:",
			"User:",
			"Address:",
			"Port:",
			"Password:",
			"Passkey Path:",
		}

		for i, label := range labels {
			if i == f.focusedField {
				b.WriteString(labelStyle.Render("→ " + label))
			} else {
				b.WriteString(subtitleStyle.Render("  " + label))
			}
			b.WriteString("\n")
			b.WriteString(f.inputs[i].View())
			b.WriteString("\n\n")
		}

		f.renderToggle(&b, fieldUseSSHAgent, "Use SSH agent (1Password)", f.useSSHAgent)
		b.WriteString("\n\n")

		if f.useSSHAgent {
			f.renderField(&b, fieldSSHAgentSocket, "Agent Socket:")
			b.WriteString("\n\n")
		}

		f.renderToggle(&b, fieldUseOnePassword, "Fetch username/password from 1Password", f.useOnePassword)
		b.WriteString("\n\n")

		if f.useOnePassword {
			f.renderField(&b, fieldOnePasswordVault, "1Password Vault:")
			b.WriteString("\n\n")
			f.renderField(&b, fieldOnePasswordItem, "1Password Item:")
			b.WriteString("\n")
			b.WriteString(mutedTextStyle.Render("    Looks up \"username\" and \"password\" fields on that item at connect time."))
			b.WriteString("\n\n")
		}
	}

	if f.err != "" {
		b.WriteString(errorStyle.Render("Error: " + f.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("tab/shift+tab: navigate • space: toggle • enter: save • esc: cancel"))

	return b.String()
}

// isToggleField reports whether a field index is a checkbox rather than a
// text input.
func isToggleField(field int) bool {
	return field == fieldUseSSHAgent || field == fieldUseOnePassword
}

// fieldActive reports whether a field is currently navigable: toggle fields
// always are, but the fields they gate are only reachable while enabled.
func (f *ConnectionForm) fieldActive(field int) bool {
	if f.onePasswordMode {
		switch field {
		case fieldServerName, fieldAddress, fieldPort, fieldOnePasswordVault, fieldOnePasswordItem:
			return true
		default:
			return false
		}
	}
	switch field {
	case fieldSSHAgentSocket:
		return f.useSSHAgent
	case fieldOnePasswordVault, fieldOnePasswordItem:
		return f.useOnePassword
	default:
		return true
	}
}

// blurCurrent blurs the currently focused text input, if it is one
func (f *ConnectionForm) blurCurrent() {
	if !isToggleField(f.focusedField) {
		f.inputs[f.focusedField].Blur()
	}
}

// focusCurrent focuses the currently focused text input, if it is one
func (f *ConnectionForm) focusCurrent() {
	if !isToggleField(f.focusedField) {
		f.inputs[f.focusedField].Focus()
	}
}

// nextField moves focus to the next active field
func (f *ConnectionForm) nextField() {
	f.blurCurrent()
	for {
		f.focusedField = (f.focusedField + 1) % fieldCount
		if f.fieldActive(f.focusedField) {
			break
		}
	}
	f.focusCurrent()
}

// prevField moves focus to the previous active field
func (f *ConnectionForm) prevField() {
	f.blurCurrent()
	for {
		f.focusedField--
		if f.focusedField < 0 {
			f.focusedField = fieldCount - 1
		}
		if f.fieldActive(f.focusedField) {
			break
		}
	}
	f.focusCurrent()
}

// renderToggle writes a checkbox row for a boolean field
func (f *ConnectionForm) renderToggle(b *strings.Builder, field int, label string, checked bool) {
	box := "[ ]"
	if checked {
		box = "[✓]"
	}
	row := label + ": " + box
	if f.focusedField == field {
		b.WriteString(labelStyle.Render("→ " + row + "  (space to toggle)"))
	} else {
		b.WriteString(subtitleStyle.Render("  " + row))
	}
}

// renderField writes a labeled text input row
func (f *ConnectionForm) renderField(b *strings.Builder, field int, label string) {
	if f.focusedField == field {
		b.WriteString(labelStyle.Render("→ " + label))
	} else {
		b.WriteString(subtitleStyle.Render("  " + label))
	}
	b.WriteString("\n")
	b.WriteString(f.inputs[field].View())
}

// ToConnection converts form values to a Connection model
func (f *ConnectionForm) ToConnection() (*models.Connection, error) {
	var conn *models.Connection

	if f.isEdit && f.originalConn != nil {
		conn = f.originalConn
	} else {
		conn = models.NewConnection()
	}

	conn.ServerName = strings.TrimSpace(f.inputs[fieldServerName].Value())
	conn.Address = strings.TrimSpace(f.inputs[fieldAddress].Value())
	conn.OnePasswordVault = strings.TrimSpace(f.inputs[fieldOnePasswordVault].Value())
	conn.OnePasswordItem = strings.TrimSpace(f.inputs[fieldOnePasswordItem].Value())

	if f.onePasswordMode {
		conn.User = ""
		conn.Password = ""
		conn.PasskeyPath = ""
		conn.UseSSHAgent = false
		conn.SSHAgentSocket = ""
		conn.UseOnePassword = true
	} else {
		conn.User = strings.TrimSpace(f.inputs[fieldUser].Value())
		conn.Password = f.inputs[fieldPassword].Value()
		conn.PasskeyPath = strings.TrimSpace(f.inputs[fieldPasskeyPath].Value())
		conn.UseSSHAgent = f.useSSHAgent
		conn.SSHAgentSocket = strings.TrimSpace(f.inputs[fieldSSHAgentSocket].Value())
		conn.UseOnePassword = f.useOnePassword
	}

	portStr := strings.TrimSpace(f.inputs[fieldPort].Value())
	if portStr == "" {
		conn.Port = 22
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			f.err = "Port must be a number"
			return nil, err
		}
		conn.Port = port
	}

	if err := conn.Validate(); err != nil {
		f.err = err.Error()
		return nil, err
	}

	return conn, nil
}

// SetError sets an error message
func (f *ConnectionForm) SetError(err string) {
	f.err = err
}
