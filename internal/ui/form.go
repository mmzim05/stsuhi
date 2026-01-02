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
	fieldCount
)

// ConnectionForm is a model for the add/edit connection form
type ConnectionForm struct {
	inputs       []textinput.Model
	focusedField int
	err          string
	isEdit       bool
	originalConn *models.Connection
}

// NewConnectionForm creates a new connection form
func NewConnectionForm(conn *models.Connection) ConnectionForm {
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

	form := ConnectionForm{
		inputs:       inputs,
		focusedField: 0,
		isEdit:       conn != nil,
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
		}
	}

	// Update focused input
	var cmd tea.Cmd
	f.inputs[f.focusedField], cmd = f.inputs[f.focusedField].Update(msg)
	cmds = append(cmds, cmd)

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

	if f.err != "" {
		b.WriteString(errorStyle.Render("Error: " + f.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("tab/shift+tab: navigate • enter: save • esc: cancel"))

	return b.String()
}

// nextField moves focus to the next field
func (f *ConnectionForm) nextField() {
	f.inputs[f.focusedField].Blur()
	f.focusedField = (f.focusedField + 1) % fieldCount
	f.inputs[f.focusedField].Focus()
}

// prevField moves focus to the previous field
func (f *ConnectionForm) prevField() {
	f.inputs[f.focusedField].Blur()
	f.focusedField--
	if f.focusedField < 0 {
		f.focusedField = fieldCount - 1
	}
	f.inputs[f.focusedField].Focus()
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
	conn.User = strings.TrimSpace(f.inputs[fieldUser].Value())
	conn.Address = strings.TrimSpace(f.inputs[fieldAddress].Value())
	conn.Password = f.inputs[fieldPassword].Value()
	conn.PasskeyPath = strings.TrimSpace(f.inputs[fieldPasskeyPath].Value())

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
