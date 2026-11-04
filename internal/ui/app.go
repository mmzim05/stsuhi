package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/crypto"
	"github.com/mmzim05/stsuhi/internal/keyring"
	"github.com/mmzim05/stsuhi/internal/models"
	"github.com/mmzim05/stsuhi/internal/onepassword"
	"github.com/mmzim05/stsuhi/internal/storage"
	"github.com/mmzim05/stsuhi/internal/theme"
)

// AppState represents the current state of the application
type AppState int

// AppState values, one per screen. Update and View dispatch on them.
const (
	StateFirstBoot AppState = iota // create a master password (triggered on demand, not at boot)
	StateFirstBootConfirm
	StateLogin        // enter an existing master password (triggered on demand)
	StateResetConfirm // forget a lost master password
	StateOnePasswordUnlock
	StateSettings
	StateOnePasswordHelp // 1Password setup instructions, purely informational
	StateThemePicker
	StateList
	StateAdd
	StateEdit
	StateDelete
	StateConnecting
	StateQuitting
)

// pendingAction records what to do once a master password becomes
// available, since obtaining one is now a detour (StateFirstBoot/
// StateLogin) off whatever the user was actually doing, not a boot-time
// gate. See beginMasterPasswordPrompt and afterAuthSuccess.
type pendingAction int

const (
	pendingNone pendingAction = iota
	pendingSaveConnection
	pendingConnect
	pendingEdit
)

// App is the main application model
type App struct {
	state             AppState
	storage           storage.Storage
	config            *config.Config
	masterPassword    string
	connections       []*models.Connection
	list              ConnectionList
	form              ConnectionForm
	passwordInput     PasswordInput
	confirmPassword   PasswordInput
	onePasswordUnlock OnePasswordUnlock
	settings          Settings
	themePicker       ThemePicker
	deleteTarget      *models.Connection
	pendingConnection *models.Connection // Connection to establish after TUI exits
	err               string
	width             int
	height            int
	keyringAvailable  bool

	// Lazy master-password flow state - see beginMasterPasswordPrompt.
	pendingAction      pendingAction
	pendingFormConn    *models.Connection
	pendingConnectConn *models.Connection
	pendingEditConn    *models.Connection
	pendingReturnState AppState
}

// NewApp creates a new application instance
func NewApp(store storage.Storage, cfg *config.Config) *App {
	return &App{
		storage:          store,
		config:           cfg,
		width:            80,
		height:           24,
		keyringAvailable: keyring.IsAvailable(),
	}
}

// NewAppWithPassword creates a new application instance with a pre-authenticated master password
func NewAppWithPassword(store storage.Storage, cfg *config.Config, masterPassword string) *App {
	return &App{
		storage:          store,
		config:           cfg,
		masterPassword:   masterPassword,
		width:            80,
		height:           24,
		keyringAvailable: keyring.IsAvailable(),
	}
}

// Init initializes the application. There is no master-password gate here
// any more: connections are listed as metadata only (see
// storage.Storage.ListConnections), which never requires one. A master
// password is only ever asked for later, on demand, by
// beginMasterPasswordPrompt - when saving a connection with a typed
// password/passkey, or connecting to one that already has one stored.
func (a *App) Init() tea.Cmd {
	a.unlock() // silent, best-effort - never blocks startup

	connections, err := a.storage.ListConnections(a.masterPassword)
	if err != nil {
		a.err = fmt.Sprintf("Failed to load connections: %v", err)
		connections = []*models.Connection{}
	}
	a.connections = connections
	a.list = NewConnectionList(a.connections)
	a.list.SetSize(a.width, a.height)
	a.state = StateList
	return nil
}

// unlock gets the master password without asking, if it can. On a
// machine with a keyring and no master password yet, it creates a random
// one and keeps it there, so the user never sees a password prompt.
// Otherwise it reads the keyring, then the 1Password item if that's set
// up. Returns whether a.masterPassword is now set.
func (a *App) unlock() bool {
	if a.masterPassword != "" {
		return true
	}
	isSet, err := a.storage.IsMasterPasswordSet()
	if err != nil {
		a.err = fmt.Sprintf("Failed to check master password: %v", err)
		return false
	}
	if !isSet {
		return a.keyringAvailable && a.createKeyringKey()
	}

	if a.keyringAvailable {
		password, err := keyring.GetPassword(a.storage.KeyringAccount())
		switch {
		case err == nil:
			if a.tryMasterPassword(password) {
				return true
			}
		case !errors.Is(err, keyring.ErrNotFound):
			a.err = fmt.Sprintf("Failed to read keyring: %v", err)
		}
	}

	if a.config.Unlock.UsesOnePassword() && onepassword.IsAvailable() {
		// A locked or signed-out op just fails here, and the password
		// prompt that follows is the fallback, so the error isn't shown.
		password, err := onepassword.GetPassword(
			a.config.Unlock.OnePasswordItemName)
		if err == nil && a.tryMasterPassword(password) {
			a.rememberInKeyring()
			return true
		}
	}
	return false
}

// tryMasterPassword sets a.masterPassword if password is the right one
func (a *App) tryMasterPassword(password string) bool {
	valid, err := a.storage.VerifyMasterPassword(password)
	if err != nil {
		a.err = fmt.Sprintf("Failed to verify master password: %v", err)
		return false
	}
	if valid {
		a.masterPassword = password
	}
	return valid
}

// createKeyringKey makes a random master password and stores it in the
// keyring. The keyring is written first: a master.key without its key
// would lock every new secret behind a password nobody knows.
func (a *App) createKeyringKey() bool {
	key, err := crypto.NewRandomKey()
	if err != nil {
		a.err = fmt.Sprintf("Failed to create master key: %v", err)
		return false
	}
	if err := keyring.SavePassword(a.storage.KeyringAccount(), key); err != nil {
		a.err = fmt.Sprintf("Failed to save master key: %v", err)
		return false
	}
	if err := a.storage.SetMasterPassword(key); err != nil {
		a.err = fmt.Sprintf("Failed to save master key: %v", err)
		return false
	}
	a.masterPassword = key
	return true
}

// rememberInKeyring stores a typed or 1Password-supplied master password
// in the keyring, so this machine never asks for it again
func (a *App) rememberInKeyring() {
	if !a.keyringAvailable {
		return
	}
	err := keyring.SavePassword(a.storage.KeyringAccount(), a.masterPassword)
	if err != nil {
		a.err = fmt.Sprintf("Failed to save master key to keyring: %v", err)
	}
}

// beginMasterPasswordPrompt transitions to the password-entry/creation
// flow, remembering what to do once a password is available (action) and
// where to return to if the user cancels (returnState).
func (a *App) beginMasterPasswordPrompt(action pendingAction, returnState AppState) (tea.Model, tea.Cmd) {
	a.pendingAction = action
	a.pendingReturnState = returnState
	a.err = ""

	isSet, err := a.storage.IsMasterPasswordSet()
	if err != nil {
		a.err = fmt.Sprintf("Failed to check master password: %v", err)
		a.pendingAction = pendingNone
		a.state = returnState
		return a, nil
	}

	if isSet {
		a.state = StateLogin
		a.passwordInput = NewPasswordInput("Master Password:", "Enter your master password")
		return a, a.passwordInput.Init()
	}

	a.state = StateFirstBoot
	a.passwordInput = NewPasswordInput("Create Master Password:", "Enter a strong password")
	return a, a.passwordInput.Init()
}

// cancelMasterPasswordPrompt abandons whatever triggered
// beginMasterPasswordPrompt and returns to where the user was.
func (a *App) cancelMasterPasswordPrompt() (tea.Model, tea.Cmd) {
	a.pendingAction = pendingNone
	a.pendingFormConn = nil
	a.pendingConnectConn = nil
	a.pendingEditConn = nil
	a.state = a.pendingReturnState
	return a, nil
}

// afterAuthSuccess runs once a master password has just been created or
// typed: it keeps it in the keyring when there is one, then carries out
// whatever was pending.
func (a *App) afterAuthSuccess() (tea.Model, tea.Cmd) {
	a.rememberInKeyring()

	switch a.pendingAction {
	case pendingSaveConnection:
		conn := a.pendingFormConn
		a.pendingAction = pendingNone
		a.pendingFormConn = nil
		if err := a.storage.SaveConnection(conn, a.masterPassword); err != nil {
			a.err = fmt.Sprintf("Failed to save: %v", err)
			a.state = StateList
			return a, nil
		}
		a.reloadConnections()
		a.state = StateList
		return a, nil

	case pendingConnect:
		target := a.pendingConnectConn
		a.pendingAction = pendingNone
		a.pendingConnectConn = nil
		full, err := a.storage.LoadConnection(target.ID, a.masterPassword)
		if err != nil {
			a.err = fmt.Sprintf("Failed to decrypt connection: %v", err)
			a.state = StateList
			return a, nil
		}
		return a, a.connectSSH(full)

	case pendingEdit:
		target := a.pendingEditConn
		a.pendingAction = pendingNone
		a.pendingEditConn = nil
		full, err := a.storage.LoadConnection(target.ID, a.masterPassword)
		if err != nil {
			a.err = fmt.Sprintf("Failed to decrypt connection: %v", err)
			a.state = StateList
			return a, nil
		}
		a.state = StateEdit
		a.form = NewConnectionForm(full)
		return a, a.form.Init()

	default:
		a.state = StateList
		return a, nil
	}
}

// Update updates the application state
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.list.SetSize(a.width, a.height)
		return a, nil

	case tea.KeyMsg:
		switch a.state {
		case StateFirstBoot:
			return a.updateFirstBoot(msg)
		case StateFirstBootConfirm:
			return a.updateFirstBootConfirm(msg)
		case StateLogin:
			return a.updateLogin(msg)
		case StateResetConfirm:
			return a.updateResetConfirm(msg)
		case StateOnePasswordUnlock:
			return a.updateOnePasswordUnlock(msg)
		case StateSettings:
			return a.updateSettings(msg)
		case StateOnePasswordHelp:
			return a.updateOnePasswordHelp(msg)
		case StateThemePicker:
			return a.updateThemePicker(msg)
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
	// Center every screen in the real terminal. Place aligns each line on
	// its own, so pad the lines to one width first to keep the screen's
	// left edge straight. Content larger than the window is left as is.
	content := a.screen()
	block := lipgloss.NewStyle().
		Width(lipgloss.Width(content)).
		Render(content)
	return lipgloss.Place(a.width, a.height,
		lipgloss.Center, lipgloss.Center, block)
}

// screen renders the current state's content, before centering
func (a *App) screen() string {
	switch a.state {
	case StateFirstBoot:
		return a.viewFirstBoot()
	case StateFirstBootConfirm:
		return a.viewFirstBootConfirm()
	case StateLogin:
		return a.viewLogin()
	case StateResetConfirm:
		return a.viewResetConfirm()
	case StateOnePasswordUnlock:
		return a.onePasswordUnlock.View()
	case StateSettings:
		return a.viewSettings()
	case StateOnePasswordHelp:
		return viewOnePasswordInfoBody()
	case StateThemePicker:
		return a.viewThemePicker()
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

// First Boot State (create a master password) - reached only via
// beginMasterPasswordPrompt now, never at startup.
func (a *App) updateFirstBoot(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return a.cancelMasterPasswordPrompt()
	case "enter":
		password := a.passwordInput.Value()
		if len(password) < 8 {
			a.err = "Password must be at least 8 characters"
			return a, nil
		}
		a.masterPassword = password
		a.err = ""
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
	b.WriteString(titleStyle.Render("Create Master Password"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render(
		"No system keyring was found, so stsuhi needs a master password"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(
		"to encrypt the server password or key you just entered."))
	b.WriteString("\n\n")
	b.WriteString(a.passwordInput.View())
	b.WriteString("\n\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render("enter: continue • esc: cancel"))
	return b.String()
}

// First Boot Confirm State
func (a *App) updateFirstBootConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return a.cancelMasterPasswordPrompt()
	case "enter":
		confirmPassword := a.confirmPassword.Value()
		if confirmPassword != a.masterPassword {
			a.err = "Passwords do not match"
			a.state = StateFirstBoot
			a.passwordInput = NewPasswordInput("Create Master Password:", "Enter a strong password")
			return a, a.passwordInput.Init()
		}

		if err := a.storage.SetMasterPassword(a.masterPassword); err != nil {
			a.err = fmt.Sprintf("Failed to save master password: %v", err)
			return a, nil
		}
		if err := a.storage.InitStorage(); err != nil {
			a.err = fmt.Sprintf("Failed to initialize storage: %v", err)
			return a, nil
		}

		return a.afterAuthSuccess()
	}

	var cmd tea.Cmd
	a.confirmPassword, cmd = a.confirmPassword.Update(msg)
	return a, cmd
}

func (a *App) viewFirstBootConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create Master Password"))
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

// Login State (enter an existing master password) - reached only via
// beginMasterPasswordPrompt now, never at startup.
func (a *App) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return a.cancelMasterPasswordPrompt()
	case "ctrl+r":
		a.err = ""
		a.state = StateResetConfirm
		return a, nil
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
		return a.afterAuthSuccess()
	}

	var cmd tea.Cmd
	a.passwordInput, cmd = a.passwordInput.Update(msg)
	return a, cmd
}

func (a *App) viewLogin() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Master Password Required"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Needed to read or write this connection's saved password/key."))
	b.WriteString("\n\n")
	b.WriteString(a.passwordInput.View())
	b.WriteString("\n\n")
	if a.err != "" {
		b.WriteString(errorStyle.Render(a.err))
		b.WriteString("\n\n")
	}
	b.WriteString(helpStyle.Render(
		"enter: unlock • ctrl+r: forgot it, reset • esc: cancel"))
	return b.String()
}

// Reset Confirm State - forget a lost master password. Reached from the
// login prompt; mainly for a keyring that lost its generated key, which
// nobody could type.
func (a *App) updateResetConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "n":
		a.state = StateLogin
		return a, nil
	case "y":
		return a.resetMasterPassword()
	}
	return a, nil
}

// sealedConnectionCount counts connections whose secret a reset deletes
func (a *App) sealedConnectionCount() int {
	count := 0
	for _, conn := range a.connections {
		if conn.HasStoredSecret {
			count++
		}
	}
	return count
}

func (a *App) viewResetConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Reset Master Password"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf(
		"This permanently deletes %d connection(s) with a saved password\n"+
			"or key: they can't be decrypted without the old master password.\n"+
			"Connections without saved secrets are kept.",
		a.sealedConnectionCount()))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y: delete and reset • n: back"))
	return dialogBoxStyle.Render(b.String())
}

// resetMasterPassword deletes the sealed connections and the master key,
// then carries on with a fresh key. Only a new connection still being
// saved survives as a pending action: connect and edit targets were
// sealed, so they are gone.
func (a *App) resetMasterPassword() (tea.Model, tea.Cmd) {
	if err := a.storage.ResetMasterPassword(); err != nil {
		a.err = fmt.Sprintf("Failed to reset: %v", err)
		a.state = StateLogin
		return a, nil
	}
	if a.keyringAvailable {
		err := keyring.DeletePassword(a.storage.KeyringAccount())
		if err != nil {
			a.err = fmt.Sprintf("Failed to clear old key: %v", err)
		}
	}
	a.config.Unlock = config.UnlockConfig{}
	if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
		a.err = fmt.Sprintf("Failed to save config: %v", err)
	}
	a.reloadConnections()

	a.pendingConnectConn = nil
	a.pendingEditConn = nil
	if a.pendingAction != pendingSaveConnection {
		a.pendingAction = pendingNone
		a.state = StateList
		return a, nil
	}
	if a.unlock() {
		return a.afterAuthSuccess()
	}
	return a.beginMasterPasswordPrompt(pendingSaveConnection,
		a.pendingReturnState)
}

// List State
func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.state = StateQuitting
		return a, tea.Quit
	case "s":
		a.settings = NewSettings(a.config, a.keyringAvailable)
		a.state = StateSettings
		return a, nil
	case "a":
		a.state = StateAdd
		a.form = NewConnectionForm(nil)
		return a, a.form.Init()
	case "e":
		if !a.list.IsEmpty() {
			selected := a.list.SelectedConnection()
			if selected.Locked {
				if !a.unlock() {
					a.pendingEditConn = selected
					return a.beginMasterPasswordPrompt(pendingEdit, StateList)
				}
				full, err := a.storage.LoadConnection(selected.ID, a.masterPassword)
				if err != nil {
					a.err = fmt.Sprintf("Failed to decrypt connection: %v", err)
					return a, nil
				}
				selected = full
			}
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
	formState := a.state
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateList
		return a, nil
	case "enter":
		conn, err := a.form.ToConnection()
		if err != nil {
			return a, nil
		}

		if conn.HasLocalSecret() && !a.unlock() {
			a.pendingFormConn = conn
			return a.beginMasterPasswordPrompt(pendingSaveConnection, formState)
		}

		if err := a.storage.SaveConnection(conn, a.masterPassword); err != nil {
			a.form.SetError(fmt.Sprintf("Failed to save: %v", err))
			return a, nil
		}

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

// 1Password Unlock State
func (a *App) updateOnePasswordUnlock(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateSettings
		return a, nil
	case "enter":
		if err := a.onePasswordUnlock.Validate(); err != nil {
			a.onePasswordUnlock.SetError(err.Error())
			return a, nil
		}
		// Nothing is written to 1Password: the user keeps that item.
		a.config.Unlock = config.UnlockConfig{
			Enabled:             true,
			Backend:             config.BackendOnePassword,
			OnePasswordItemName: a.onePasswordUnlock.ItemName(),
		}
		if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
			a.onePasswordUnlock.SetError(
				fmt.Sprintf("Failed to save config: %v", err))
			return a, nil
		}
		a.settings = NewSettings(a.config, a.keyringAvailable)
		a.state = StateSettings
		return a, nil
	}

	var cmd tea.Cmd
	a.onePasswordUnlock, cmd = a.onePasswordUnlock.Update(msg)
	return a, cmd
}

// Settings State
func (a *App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateList
		return a, nil
	case "enter":
		option := a.settings.GetSelectedOption()
		switch option {
		case SettingsOnePasswordEnable, SettingsOnePasswordChange:
			// The item is checked against the master password, so that
			// has to be known first.
			if !a.unlock() {
				return a.beginMasterPasswordPrompt(pendingNone, StateSettings)
			}
			a.onePasswordUnlock = NewOnePasswordUnlock(a.config,
				a.masterPassword)
			a.state = StateOnePasswordUnlock
			return a, a.onePasswordUnlock.Init()
		case SettingsOnePasswordDisable:
			a.config.Unlock = config.UnlockConfig{}
			if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
				a.err = fmt.Sprintf("Failed to save config: %v", err)
			}
			a.settings = NewSettings(a.config, a.keyringAvailable)
			return a, nil
		case SettingsOnePasswordHelp:
			a.state = StateOnePasswordHelp
			return a, nil
		case SettingsTheme:
			a.themePicker = NewThemePicker(a.config.Theme)
			a.state = StateThemePicker
			return a, nil
		case SettingsBack:
			a.state = StateList
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.settings, cmd = a.settings.Update(msg)
	return a, cmd
}

func (a *App) viewSettings() string {
	return a.settings.View()
}

// One-Password Help State - static setup instructions, purely
// informational, no side effects.
func (a *App) updateOnePasswordHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "enter":
		a.state = StateSettings
		return a, nil
	}
	return a, nil
}

// Theme Picker State
func (a *App) updateThemePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateSettings
		return a, nil
	case "enter":
		a.config.Theme = a.themePicker.SelectedValue()
		SetTheme(theme.Resolve(a.config.Theme))
		if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
			a.err = fmt.Sprintf("Failed to save config: %v", err)
		}
		a.settings = NewSettings(a.config, a.keyringAvailable)
		a.state = StateSettings
		return a, nil
	}

	var cmd tea.Cmd
	a.themePicker, cmd = a.themePicker.Update(msg)
	return a, cmd
}

func (a *App) viewThemePicker() string {
	return a.themePicker.View()
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

// connectSSH stores conn as the pending SSH connection (established by
// main, after the TUI exits) if it's ready to connect with as-is, or
// detours through beginMasterPasswordPrompt first if it has a stored
// secret this session doesn't have the master password for yet.
func (a *App) connectSSH(conn *models.Connection) tea.Cmd {
	if conn.Locked {
		if !a.unlock() {
			a.pendingConnectConn = conn
			_, cmd := a.beginMasterPasswordPrompt(pendingConnect, StateList)
			return cmd
		}
		full, err := a.storage.LoadConnection(conn.ID, a.masterPassword)
		if err != nil {
			a.err = fmt.Sprintf("Failed to decrypt connection: %v", err)
			return nil
		}
		conn = full
	}

	a.pendingConnection = conn
	a.state = StateQuitting
	return tea.Quit
}

// GetPendingConnection returns the connection to establish (if any) after TUI exits
func (a *App) GetPendingConnection() *models.Connection {
	return a.pendingConnection
}

// GetMasterPassword returns the master password (for preserving authentication across restarts)
func (a *App) GetMasterPassword() string {
	return a.masterPassword
}
