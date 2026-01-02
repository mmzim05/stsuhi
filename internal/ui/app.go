package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/models"
	"stsuhi/internal/ssh"
	"stsuhi/internal/storage"
)

// AppState represents the current state of the application
type AppState int

const (
	StateFirstBoot AppState = iota
	StateFirstBootConfirm
	StateLogin
	StateList
	StateAdd
	StateEdit
	StateDelete
	StateConnecting
	StateQuitting
)

// App is the main application model
type App struct {
	state           AppState
	storage         storage.Storage
	masterPassword  string
	connections     []*models.Connection
	list            ConnectionList
	form            ConnectionForm
	passwordInput   PasswordInput
	confirmPassword PasswordInput
	deleteTarget    *models.Connection
	err             string
	width           int
	height          int
}

// NewApp creates a new application instance
func NewApp(store storage.Storage) *App {
	return &App{
		storage: store,
		width:   80,
		height:  24,
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	// Check if this is first boot
	isSet, err := a.storage.IsMasterPasswordSet()
	if err != nil {
		a.err = fmt.Sprintf("Failed to check master password: %v", err)
		a.state = StateQuitting
		return tea.Quit
	}

	if !isSet {
		// First boot - setup master password
		a.state = StateFirstBoot
		a.passwordInput = NewPasswordInput("Create Master Password:", "Enter a strong password")
		return a.passwordInput.Init()
	}

	// Login required
	a.state = StateLogin
	a.passwordInput = NewPasswordInput("Master Password:", "Enter your master password")
	return a.passwordInput.Init()
}

// Update updates the application state
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.state == StateList && !a.list.IsEmpty() {
			a.list.SetSize(a.width, a.height)
		}
		return a, nil

	case tea.KeyMsg:
		switch a.state {
		case StateFirstBoot:
			return a.updateFirstBoot(msg)
		case StateFirstBootConfirm:
			return a.updateFirstBootConfirm(msg)
		case StateLogin:
			return a.updateLogin(msg)
		case StateList:
			return a.updateList(msg)
		case StateAdd, StateEdit:
			return a.updateForm(msg)
		case StateDelete:
			return a.updateDelete(msg)
		}
	}

	return a, nil
}

// View renders the application
func (a *App) View() string {
	if a.state == StateQuitting {
		return ""
	}

	switch a.state {
	case StateFirstBoot:
		return a.viewFirstBoot()
	case StateFirstBootConfirm:
		return a.viewFirstBootConfirm()
	case StateLogin:
		return a.viewLogin()
	case StateList:
		return a.viewList()
	case StateAdd, StateEdit:
		return a.viewForm()
	case StateDelete:
		return a.viewDelete()
	case StateConnecting:
		return a.viewConnecting()
	default:
		return "Unknown state"
	}
}

// First Boot State
func (a *App) updateFirstBoot(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateQuitting
		return a, tea.Quit
	case "enter":
		password := a.passwordInput.Value()
		if len(password) < 8 {
			a.err = "Password must be at least 8 characters"
			return a, nil
		}
		a.masterPassword = password
		a.state = StateFirstBootConfirm
		a.confirmPassword = NewPasswordInput("Confirm Master Password:", "Re-enter your password")
		return a, a.confirmPassword.Init()
	}

	var cmd tea.Cmd
	a.passwordInput, cmd = a.passwordInput.Update(msg)
	return a, cmd
}

func (a *App) viewFirstBoot() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Connection Manager - First Boot"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Welcome! Let's set up your master password."))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("This password will encrypt all your SSH connections."))
	b.WriteString("\n\n")
	b.WriteString(a.passwordInput.View())
	b.WriteString("\n\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render("enter: continue • esc: quit"))
	return b.String()
}

// First Boot Confirm State
func (a *App) updateFirstBootConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateFirstBoot
		a.passwordInput = NewPasswordInput("Create Master Password:", "Enter a strong password")
		a.err = ""
		return a, a.passwordInput.Init()
	case "enter":
		confirmPassword := a.confirmPassword.Value()
		if confirmPassword != a.masterPassword {
			a.err = "Passwords do not match"
			a.state = StateFirstBoot
			a.passwordInput = NewPasswordInput("Create Master Password:", "Enter a strong password")
			return a, a.passwordInput.Init()
		}

		// Save master password
		if err := a.storage.SetMasterPassword(a.masterPassword); err != nil {
			a.err = fmt.Sprintf("Failed to save master password: %v", err)
			return a, nil
		}

		// Initialize storage
		if err := a.storage.InitStorage(); err != nil {
			a.err = fmt.Sprintf("Failed to initialize storage: %v", err)
			return a, nil
		}

		// Load connections (empty on first boot)
		a.connections = []*models.Connection{}
		a.list = NewConnectionList(a.connections)
		a.list.SetSize(a.width, a.height)
		a.state = StateList
		return a, nil
	}

	var cmd tea.Cmd
	a.confirmPassword, cmd = a.confirmPassword.Update(msg)
	return a, cmd
}

func (a *App) viewFirstBootConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Connection Manager - First Boot"))
	b.WriteString("\n\n")
	b.WriteString(a.confirmPassword.View())
	b.WriteString("\n\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render("enter: confirm • esc: back"))
	return b.String()
}

// Login State
func (a *App) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateQuitting
		return a, tea.Quit
	case "enter":
		password := a.passwordInput.Value()
		valid, err := a.storage.VerifyMasterPassword(password)
		if err != nil {
			a.err = fmt.Sprintf("Failed to verify password: %v", err)
			return a, nil
		}
		if !valid {
			a.err = "Incorrect password"
			a.passwordInput.SetValue("")
			return a, nil
		}

		a.masterPassword = password
		a.err = ""

		// Load connections
		connections, err := a.storage.ListConnections(a.masterPassword)
		if err != nil {
			a.err = fmt.Sprintf("Failed to load connections: %v", err)
			return a, nil
		}

		a.connections = connections
		a.list = NewConnectionList(a.connections)
		a.list.SetSize(a.width, a.height)
		a.state = StateList
		return a, nil
	}

	var cmd tea.Cmd
	a.passwordInput, cmd = a.passwordInput.Update(msg)
	return a, cmd
}

func (a *App) viewLogin() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Connection Manager"))
	b.WriteString("\n\n")
	b.WriteString(a.passwordInput.View())
	b.WriteString("\n\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render("enter: login • esc: quit"))
	return b.String()
}

// List State
func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.state = StateQuitting
		return a, tea.Quit
	case "a":
		a.state = StateAdd
		a.form = NewConnectionForm(nil)
		return a, a.form.Init()
	case "e":
		if !a.list.IsEmpty() {
			selected := a.list.SelectedConnection()
			a.state = StateEdit
			a.form = NewConnectionForm(selected)
			return a, a.form.Init()
		}
	case "d":
		if !a.list.IsEmpty() {
			a.deleteTarget = a.list.SelectedConnection()
			a.state = StateDelete
			return a, nil
		}
	case "enter":
		if !a.list.IsEmpty() {
			selected := a.list.SelectedConnection()
			return a, a.connectSSH(selected)
		}
	}

	var cmd tea.Cmd
	a.list, cmd = a.list.Update(msg)
	return a, cmd
}

func (a *App) viewList() string {
	if a.list.IsEmpty() {
		return EmptyView()
	}
	return a.list.View()
}

// Form State (Add/Edit)
func (a *App) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateList
		return a, nil
	case "enter":
		conn, err := a.form.ToConnection()
		if err != nil {
			return a, nil
		}

		if err := a.storage.SaveConnection(conn, a.masterPassword); err != nil {
			a.form.SetError(fmt.Sprintf("Failed to save: %v", err))
			return a, nil
		}

		// Reload connections
		a.reloadConnections()
		a.state = StateList
		return a, nil
	}

	var cmd tea.Cmd
	a.form, cmd = a.form.Update(msg)
	return a, cmd
}

func (a *App) viewForm() string {
	return a.form.View()
}

// Delete State
func (a *App) updateDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "n":
		a.state = StateList
		return a, nil
	case "y":
		if err := a.storage.DeleteConnection(a.deleteTarget.ID); err != nil {
			a.err = fmt.Sprintf("Failed to delete: %v", err)
		}
		a.reloadConnections()
		a.state = StateList
		return a, nil
	}
	return a, nil
}

func (a *App) viewDelete() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Are you sure you want to delete '%s'?", a.deleteTarget.ServerName))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y: yes • n: no"))
	return dialogBoxStyle.Render(b.String())
}

// Connecting State
func (a *App) viewConnecting() string {
	return successStyle.Render("Connecting to SSH...")
}

// Helper functions
func (a *App) reloadConnections() {
	connections, err := a.storage.ListConnections(a.masterPassword)
	if err != nil {
		a.err = fmt.Sprintf("Failed to reload connections: %v", err)
		return
	}
	a.connections = connections
	a.list.UpdateConnections(a.connections)
}

func (a *App) connectSSH(conn *models.Connection) tea.Cmd {
	return func() tea.Msg {
		// Show connecting state briefly
		a.state = StateConnecting

		// Execute SSH connection
		if err := ssh.Connect(conn); err != nil {
			a.err = fmt.Sprintf("Connection failed: %v", err)
		}

		// Return to list view
		a.state = StateList
		return nil
	}
}
