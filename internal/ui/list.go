package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"stsuhi/internal/models"
)

// connectionItem wraps a Connection for use in the list
type connectionItem struct {
	conn *models.Connection
}

func (i connectionItem) Title() string {
	return i.conn.ServerName
}

func (i connectionItem) Description() string {
	authMethod := "password"
	if i.conn.PasskeyPath != "" {
		authMethod = "key"
	}
	return fmt.Sprintf("%s@%s:%d [%s]", i.conn.User, i.conn.Address, i.conn.Port, authMethod)
}

func (i connectionItem) FilterValue() string {
	return i.conn.ServerName + " " + i.conn.User + " " + i.conn.Address
}

// ConnectionList is a model for the connection list view
type ConnectionList struct {
	list     list.Model
	selected *models.Connection
}

// NewConnectionList creates a new connection list
func NewConnectionList(connections []*models.Connection) ConnectionList {
	items := make([]list.Item, len(connections))
	for i, conn := range connections {
		items[i] = connectionItem{conn: conn}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(mutedColor)

	l := list.New(items, delegate, 0, 0)
	l.Title = "SSH Connections"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return ConnectionList{
		list: l,
	}
}

// Init initializes the connection list
func (cl ConnectionList) Init() tea.Cmd {
	return nil
}

// Update updates the connection list
func (cl ConnectionList) Update(msg tea.Msg) (ConnectionList, tea.Cmd) {
	var cmd tea.Cmd
	cl.list, cmd = cl.list.Update(msg)
	return cl, cmd
}

// View renders the connection list
func (cl ConnectionList) View() string {
	var b strings.Builder
	b.WriteString(cl.list.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ or j/k: navigate • enter: connect • a: add • e: edit • d: delete • s: settings • q: quit"))
	return b.String()
}

// SelectedConnection returns the currently selected connection
func (cl ConnectionList) SelectedConnection() *models.Connection {
	item := cl.list.SelectedItem()
	if item == nil {
		return nil
	}
	return item.(connectionItem).conn
}

// SetSize sets the size of the list
func (cl *ConnectionList) SetSize(width, height int) {
	cl.list.SetSize(width, height-4) // Reserve space for help text
}

// UpdateConnections updates the list with new connections
func (cl *ConnectionList) UpdateConnections(connections []*models.Connection) {
	items := make([]list.Item, len(connections))
	for i, conn := range connections {
		items[i] = connectionItem{conn: conn}
	}
	cl.list.SetItems(items)
}

// IsEmpty returns true if the list is empty
func (cl ConnectionList) IsEmpty() bool {
	return len(cl.list.Items()) == 0
}

// EmptyView returns a view for when there are no connections
func EmptyView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Connection Manager"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("No connections yet"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press 'a' to add your first connection"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press 'q' to quit"))
	return lipgloss.Place(
		80, 20,
		lipgloss.Center, lipgloss.Center,
		b.String(),
	)
}
