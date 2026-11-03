package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/autologin"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/keyring"
	"github.com/mmzim05/stsuhi/internal/models"
	"github.com/mmzim05/stsuhi/internal/onepassword"
	"github.com/mmzim05/stsuhi/internal/storage"
	"github.com/mmzim05/stsuhi/internal/theme"
)

// AppState represents the current state of the application
type AppState int

const (
	StateFirstBoot AppState = iota // create a master password (triggered on demand, not at boot)
	StateFirstBootConfirm
	StateLogin // enter an existing master password (triggered on demand)
	StateAutoLoginPrompt
	StateAutoLoginSetup
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
	autoLoginSetup    AutoLoginSetup
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
	if a.masterPassword == "" {
		a.tryCachedMasterPassword() // silent, best-effort - never blocks startup
	}

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

// tryCachedMasterPassword attempts to silently recover the master password
// from the auto-login backend (OS keyring or 1Password). It does nothing
// unless auto-login is enabled, and it honors the security rules: once a
// rule fires (reboot, lock, idle), the cached copy is dropped and the next
// manual login re-caches it. Never prompts; returns whether
// a.masterPassword is now set.
func (a *App) tryCachedMasterPassword() bool {
	if a.masterPassword != "" {
		return true
	}
	if !a.config.AutoLogin.Enabled {
		return false
	}

	backend := a.config.AutoLogin.EffectiveBackend()
	if expired, _ := autologin.ShouldClearAutoLogin(a.config); expired {
		if backend == config.BackendKeyring && a.keyringAvailable {
			if err := keyring.DeletePassword(); err != nil {
				a.err = fmt.Sprintf("Failed to clear auto-login: %v", err)
			}
		}
		return false
	}

	var password string
	var err error
	switch backend {
	case config.BackendOnePassword:
		if !onepassword.IsAvailable() {
			return false
		}
		password, err = onepassword.GetPassword(a.config.AutoLogin.OnePasswordItemName)
	default:
		if !a.keyringAvailable {
			return false
		}
		password, err = keyring.GetPassword()
	}
	if err != nil {
		return false
	}

	valid, err := a.storage.VerifyMasterPassword(password)
	if err != nil || !valid {
		return false
	}
	a.masterPassword = password
	a.saveLoginMetadata()
	return true
}

// saveLoginMetadata records boot ID, session ID and time of this login, so
// the auto-login security rules can tell when to forget the password
func (a *App) saveLoginMetadata() {
	autologin.UpdateLoginMetadata(a.config)
	if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
		a.err = fmt.Sprintf("Failed to save config: %v", err)
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
// verified: it caches the password for auto-login the same way a
// successful login always has, then carries out whatever was pending.
func (a *App) afterAuthSuccess() (tea.Model, tea.Cmd) {
	if a.config.AutoLogin.Enabled {
		if a.config.AutoLogin.EffectiveBackend() == config.BackendKeyring && a.keyringAvailable {
			if err := keyring.SavePassword(a.masterPassword); err != nil {
				a.err = fmt.Sprintf("Failed to cache password for auto-login: %v", err)
			}
		}
		a.saveLoginMetadata()
	}

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
		case StateAutoLoginPrompt:
			return a.updateAutoLoginPrompt(msg)
		case StateAutoLoginSetup:
			return a.updateAutoLoginSetup(msg)
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

	switch a.state {
	case StateFirstBoot:
		return a.viewFirstBoot()
	case StateFirstBootConfirm:
		return a.viewFirstBootConfirm()
	case StateLogin:
		return a.viewLogin()
	case StateAutoLoginPrompt:
		return a.viewAutoLoginPrompt()
	case StateAutoLoginSetup:
		return a.viewAutoLoginSetup()
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
	b.WriteString(subtitleStyle.Render("This password encrypts the password/key you just entered."))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Connections using only 1Password or an SSH agent never need one."))
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
	b.WriteString(helpStyle.Render("enter: unlock • esc: cancel"))
	return b.String()
}

// List State
func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.state = StateQuitting
		return a, tea.Quit
	case "s":
		a.settings = NewSettings(a.config)
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
				if a.masterPassword == "" && !a.tryCachedMasterPassword() {
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

		if conn.HasLocalSecret() && a.masterPassword == "" && !a.tryCachedMasterPassword() {
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

// Auto-Login Prompt State
func (a *App) updateAutoLoginPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "n":
		// No, maybe later
		a.state = StateList
		return a, nil
	case "y":
		// Yes, configure auto-login
		a.autoLoginSetup = NewAutoLoginSetup(a.config, a.masterPassword)
		a.state = StateAutoLoginSetup
		return a, nil
	case "d":
		// Don't ask again
		a.config.AutoLogin.DontAskAgain = true
		config.Save(a.storage.GetConfigPath(), a.config)
		a.state = StateList
		return a, nil
	}
	return a, nil
}

func (a *App) viewAutoLoginPrompt() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Enable Auto-Login?"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Would you like to enable auto-login?"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Your master password can be stored in your system keyring, or unlocked via 1Password."))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("[Y]"))
	b.WriteString(" Yes, configure auto-login\n")
	b.WriteString(labelStyle.Render("[N]"))
	b.WriteString(" No, ask me later\n")
	b.WriteString(labelStyle.Render("[D]"))
	b.WriteString(" Don't ask again\n\n")
	b.WriteString(helpStyle.Render("y: yes • n: no • d: don't ask"))
	return b.String()
}

// Auto-Login Setup State
func (a *App) updateAutoLoginSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		a.state = StateList
		return a, nil
	case "enter":
		// Validate and save
		if err := a.autoLoginSetup.Validate(); err != nil {
			a.autoLoginSetup.SetError(err.Error())
			return a, nil
		}

		backend := a.autoLoginSetup.GetBackend()
		if backend == config.BackendOnePassword {
			// Nothing to write to 1Password - the user manages that item
			// themselves. Just remember which item to read from.
			a.config.AutoLogin.OnePasswordItemName = a.autoLoginSetup.GetItemName()
		} else {
			// Save password to keyring
			if err := keyring.SavePassword(a.masterPassword); err != nil {
				a.autoLoginSetup.SetError(fmt.Sprintf("Failed to save to keyring: %v", err))
				return a, nil
			}
		}

		// Save config
		a.config.AutoLogin.Enabled = true
		a.config.AutoLogin.Backend = backend
		a.config.AutoLogin.SecurityRules = a.autoLoginSetup.GetRules()
		autologin.UpdateLoginMetadata(a.config)
		if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
			a.autoLoginSetup.SetError(fmt.Sprintf("Failed to save config: %v", err))
			return a, nil
		}

		a.state = StateList
		return a, nil
	}

	var cmd tea.Cmd
	a.autoLoginSetup, cmd = a.autoLoginSetup.Update(msg)
	return a, cmd
}

func (a *App) viewAutoLoginSetup() string {
	return a.autoLoginSetup.View()
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
		case SettingsEnable:
			// Enable auto-login - go to setup. Needs a master password
			// first if one hasn't been created/entered yet this session.
			if a.masterPassword == "" && !a.tryCachedMasterPassword() {
				return a.beginMasterPasswordPrompt(pendingNone, StateSettings)
			}
			a.autoLoginSetup = NewAutoLoginSetup(a.config, a.masterPassword)
			a.state = StateAutoLoginSetup
			return a, nil
		case SettingsReconfigure:
			a.autoLoginSetup = NewAutoLoginSetup(a.config, a.masterPassword)
			a.state = StateAutoLoginSetup
			return a, nil
		case SettingsDisable:
			// Disable auto-login
			if err := autologin.ClearAutoLogin(a.config); err != nil {
				a.err = err.Error()
			}
			if err := config.Save(a.storage.GetConfigPath(), a.config); err != nil {
				a.err = fmt.Sprintf("Failed to save config: %v", err)
			}
			a.settings = NewSettings(a.config) // Refresh settings view
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
		a.settings = NewSettings(a.config)
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
		if a.masterPassword == "" && !a.tryCachedMasterPassword() {
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
