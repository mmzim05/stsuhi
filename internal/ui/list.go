package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"stsuhi/internal/models"
)

// connectionItem wraps a Connection for use in the list
type connectionItem struct {
	conn *models.Connection
}

func (i connectionItem) Title() string {
	if i.conn.Locked {
		return "🔒 Locked"
	}
	return i.conn.ServerName
}

func (i connectionItem) Description() string {
	if i.conn.Locked {
		return "password-protected - select to unlock"
	}

	authMethod := "password"
	if i.conn.PasskeyPath != "" || i.conn.HasPasskey {
		authMethod = "key"
	}
	if i.conn.UseSSHAgent {
		authMethod = "agent"
	}
	if i.conn.UseOnePassword {
		authMethod = "1password"
	}
	user := i.conn.User
	if user == "" && i.conn.UseOnePassword {
		user = "?"
	}
	return fmt.Sprintf("%s@%s:%d [%s]", user, i.conn.Address, i.conn.Port, authMethod)
}

func (i connectionItem) FilterValue() string {
	return i.conn.ServerName + " " + i.conn.User + " " + i.conn.Address
}

// connectionDelegate renders connectionItem rows as a full-width
// highlighted bar when selected, instead of patching bubbles' default
// delegate styles (which discards its border/padding setup and causes text
// to shift horizontally between selected and unselected rows - the bug
// this replaces). Both branches below use a 2-column-wide prefix so rows
// never shift, only their color does.
type connectionDelegate struct{}

func (connectionDelegate) Height() int                         { return 2 }
func (connectionDelegate) Spacing() int                        { return 1 }
func (connectionDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (connectionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(connectionItem)
	width := m.Width()
	if !ok || width <= 0 {
		return
	}

	const prefix = "  "
	const selectedPrefix = "▌ "

	textWidth := width - len(prefix)
	title := ansi.Truncate(item.Title(), textWidth, "…")
	desc := ansi.Truncate(item.Description(), textWidth, "…")

	titleStyle, descStyle := listNormalTitleStyle, listNormalDescStyle
	rowPrefix := prefix
	if index == m.Index() {
		titleStyle, descStyle = listSelectedTitleStyle, listSelectedDescStyle
		rowPrefix = selectedPrefix
	}

	fmt.Fprintf(w, "%s\n%s",
		titleStyle.Width(width).Render(rowPrefix+title),
		descStyle.Width(width).Render(rowPrefix+desc),
	)
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

	l := list.New(items, connectionDelegate{}, 0, 0)
	l.Title = "SSH Connections"
	l.Styles.Title = titleStyle
	l.Styles.FilterPrompt = labelStyle
	l.Styles.PaginationStyle = subtitleStyle
	l.Styles.StatusBar = subtitleStyle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false) // we render our own help line with the app's real keymap

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
