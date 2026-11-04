package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/onepassword"
)

// OnePasswordUnlock is the screen that sets up reading the master password
// from a 1Password item, for machines without a keyring
type OnePasswordUnlock struct {
	itemNameInput  textinput.Model
	masterPassword string
	err            string
}

// NewOnePasswordUnlock creates the setup screen, prefilled with the item
// name from cfg. masterPassword is kept to check the item against it.
func NewOnePasswordUnlock(cfg *config.Config, masterPassword string) OnePasswordUnlock {
	itemName := textinput.New()
	itemName.Placeholder = onepassword.DefaultItemName
	itemName.CharLimit = 100
	itemName.Width = 30
	itemName.SetValue(onepassword.DefaultItemName)
	if cfg.Unlock.OnePasswordItemName != "" {
		itemName.SetValue(cfg.Unlock.OnePasswordItemName)
	}
	itemName.Focus()
	return OnePasswordUnlock{
		itemNameInput:  itemName,
		masterPassword: masterPassword,
	}
}

// Init starts the cursor blinking in the item name input
func (o OnePasswordUnlock) Init() tea.Cmd {
	return textinput.Blink
}

// Update passes keys to the item name input
func (o OnePasswordUnlock) Update(msg tea.Msg) (OnePasswordUnlock, tea.Cmd) {
	var cmd tea.Cmd
	o.itemNameInput, cmd = o.itemNameInput.Update(msg)
	return o, cmd
}

// View renders the setup screen
func (o OnePasswordUnlock) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Unlock with 1Password"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render(
		"This computer has no keyring, so stsuhi asks for your master"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(
		"password. It can read it from a 1Password item instead."))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("1Password item name: "))
	b.WriteString(o.itemNameInput.View())
	b.WriteString("\n")
	b.WriteString(mutedTextStyle.Render(
		"Looks for a field labeled \"password\" on that item."))
	b.WriteString("\n\n")
	if o.err != "" {
		b.WriteString(errorStyle.Render(o.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render("enter: save • esc: back"))
	return b.String()
}

// ItemName returns the entered item name, or the default when blank
func (o OnePasswordUnlock) ItemName() string {
	if name := strings.TrimSpace(o.itemNameInput.Value()); name != "" {
		return name
	}
	return onepassword.DefaultItemName
}

// SetError sets the message shown under the input
func (o *OnePasswordUnlock) SetError(err string) {
	o.err = err
}

// Validate reads the item now and checks it holds the master password, so
// a wrong item name shows up here instead of failing silently later
func (o OnePasswordUnlock) Validate() error {
	itemName := o.ItemName()
	if err := onepassword.ValidateItemName(itemName); err != nil {
		return err
	}
	if !onepassword.IsAvailable() {
		return errors.New("1password CLI (op) not found in PATH")
	}
	password, err := onepassword.GetPassword(itemName)
	if err != nil {
		return fmt.Errorf("couldn't read item %q: %w", itemName, err)
	}
	if password != o.masterPassword {
		return fmt.Errorf(
			"1password item %q does not match your stsuhi master password",
			itemName)
	}
	return nil
}
