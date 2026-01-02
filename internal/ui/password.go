package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// PasswordInput is a model for password input
type PasswordInput struct {
	textInput textinput.Model
	label     string
}

// NewPasswordInput creates a new password input
func NewPasswordInput(label, placeholder string) PasswordInput {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Focus()

	return PasswordInput{
		textInput: ti,
		label:     label,
	}
}

// Init initializes the password input
func (p PasswordInput) Init() tea.Cmd {
	return textinput.Blink
}

// Update updates the password input
func (p PasswordInput) Update(msg tea.Msg) (PasswordInput, tea.Cmd) {
	var cmd tea.Cmd
	p.textInput, cmd = p.textInput.Update(msg)
	return p, cmd
}

// View renders the password input
func (p PasswordInput) View() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render(p.label))
	b.WriteString("\n")
	b.WriteString(p.textInput.View())
	return b.String()
}

// Value returns the current password value
func (p PasswordInput) Value() string {
	return p.textInput.Value()
}

// SetValue sets the password value
func (p *PasswordInput) SetValue(value string) {
	p.textInput.SetValue(value)
}

// Focus focuses the password input
func (p *PasswordInput) Focus() tea.Cmd {
	return p.textInput.Focus()
}

// Blur blurs the password input
func (p *PasswordInput) Blur() {
	p.textInput.Blur()
}
