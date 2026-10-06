package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"stsuhi/internal/crypto"
	"stsuhi/internal/models"
)

const (
	ConnectionsDirName = "connections"
	MasterKeyFileName  = "master.key"
	ConfigFileName     = "config.json"
	ConnectionFileExt  = ".json"
)

// ErrMasterPasswordRequired is returned by LoadConnection when a connection
// has a locally-encrypted secret but no master password was supplied to
// decrypt it.
var ErrMasterPasswordRequired = errors.New("master password required to read this connection's saved secret")

// FileStorage implements Storage interface using per-connection files.
// Each connection's non-secret fields (server name, address, port, auth
// toggles, 1Password reference) are always plaintext JSON, so listing,
// browsing, and renaming never need a master password. Only a manually
// entered password/passkey path/sudo password is encrypted, and only then
// is a master password required - see secretBundle and onDiskConnection.
type FileStorage struct {
	storageDir     string
	connectionsDir string
	masterKeyPath  string
}

// NewFileStorage creates a new FileStorage instance rooted at ~/.config/stsuhi
func NewFileStorage() (*FileStorage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	return NewFileStorageAt(filepath.Join(homeDir, ".config", "stsuhi")), nil
}

// NewFileStorageAt creates a FileStorage instance rooted at an arbitrary
// directory, bypassing ~/.config/stsuhi. Used by stsuhi's -debug flag to run
// against a throwaway directory without touching real connections/config.
func NewFileStorageAt(storageDir string) *FileStorage {
	return &FileStorage{
		storageDir:     storageDir,
		connectionsDir: filepath.Join(storageDir, ConnectionsDirName),
		masterKeyPath:  filepath.Join(storageDir, MasterKeyFileName),
	}
}

// InitStorage creates the storage directory structure
func (fs *FileStorage) InitStorage() error {
	if err := os.MkdirAll(fs.storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}
	if err := os.MkdirAll(fs.connectionsDir, 0700); err != nil {
		return fmt.Errorf("failed to create connections directory: %w", err)
	}
	return nil
}

// IsMasterPasswordSet checks if a master password has been configured
func (fs *FileStorage) IsMasterPasswordSet() (bool, error) {
	_, err := os.Stat(fs.masterKeyPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check master key: %w", err)
	}
	return true, nil
}

// SetMasterPassword stores the hashed master password
func (fs *FileStorage) SetMasterPassword(password string) error {
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := os.WriteFile(fs.masterKeyPath, []byte(hash), 0600); err != nil {
		return fmt.Errorf("failed to write master key: %w", err)
	}

	return nil
}

// VerifyMasterPassword checks if the provided password matches the stored hash
func (fs *FileStorage) VerifyMasterPassword(password string) (bool, error) {
	hashBytes, err := os.ReadFile(fs.masterKeyPath)
	if err != nil {
		return false, fmt.Errorf("failed to read master key: %w", err)
	}

	return crypto.VerifyPassword(password, string(hashBytes)), nil
}

func (fs *FileStorage) connectionFilePath(id string) string {
	return filepath.Join(fs.connectionsDir, id+ConnectionFileExt)
}

// onDiskConnection is the on-disk shape of a connection file. Fields that
// are never sensitive (auth-method toggles, the 1Password reference, a
// marker for which secret kind this is) are always plaintext. ServerName/
// User/Address/Port are plaintext ONLY when there's no local secret to
// protect (e.g. a 1Password/SSH-agent-only connection, where those fields
// aren't sensitive in the first place); the moment a connection has a
// locally-stored Password/PasskeyPath/SudoPassword, those four join it
// inside EncryptedSecret instead, and these plaintext copies are omitted
// entirely - see secretBundle and SaveConnection.
type onDiskConnection struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	UseSSHAgent      bool      `json:"use_ssh_agent,omitempty"`
	SSHAgentSocket   string    `json:"ssh_agent_socket,omitempty"`
	UseOnePassword   bool      `json:"use_one_password,omitempty"`
	OnePasswordVault string    `json:"one_password_vault,omitempty"`
	OnePasswordItem  string    `json:"one_password_item,omitempty"`

	// HasPasskey records, in the clear, whether the encrypted secret (if
	// any) includes a passkey path - purely so the connection list can
	// show "key" vs "password" even while locked.
	HasPasskey bool `json:"has_passkey,omitempty"`

	// Present only when EncryptedSecret is empty - see the type doc above.
	ServerName string `json:"server_name,omitempty"`
	User       string `json:"user,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`

	// EncryptedSecret is crypto.Encrypt(secretBundle JSON, masterPassword).
	EncryptedSecret string `json:"encrypted_secret,omitempty"`
}

// secretBundle is the JSON blob encrypted into onDiskConnection.
// EncryptedSecret - the only thing ever encrypted, and (whenever it's
// used at all) the only place ServerName/User/Address/Port end up for a
// connection with a local secret.
type secretBundle struct {
	ServerName   string `json:"server_name,omitempty"`
	User         string `json:"user,omitempty"`
	Address      string `json:"address,omitempty"`
	Port         int    `json:"port,omitempty"`
	Password     string `json:"password,omitempty"`
	PasskeyPath  string `json:"passkey_path,omitempty"`
	SudoPassword string `json:"sudo_password,omitempty"`
}

func (b secretBundle) emptySecret() bool {
	return b.Password == "" && b.PasskeyPath == "" && b.SudoPassword == ""
}

// SaveConnection seals ServerName/User/Address/Port together with
// Password/PasskeyPath/SudoPassword under masterPassword whenever any of
// the latter are set; otherwise every field is written in the clear.
// Sealing or unsealing a connection (adding or removing its last local
// secret) both fall out of this naturally - the current fields decide
// which shape gets written, nothing is carried forward from the previous
// save. Editing a sealed connection therefore always needs masterPassword,
// even for a pure rename: there is no way to change a field inside the
// seal without it.
func (fs *FileStorage) SaveConnection(conn *models.Connection, masterPassword string) error {
	if err := conn.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	conn.Update()

	od := onDiskConnection{
		ID:               conn.ID,
		CreatedAt:        conn.CreatedAt,
		UpdatedAt:        conn.UpdatedAt,
		UseSSHAgent:      conn.UseSSHAgent,
		SSHAgentSocket:   conn.SSHAgentSocket,
		UseOnePassword:   conn.UseOnePassword,
		OnePasswordVault: conn.OnePasswordVault,
		OnePasswordItem:  conn.OnePasswordItem,
	}

	bundle := secretBundle{
		Password:     conn.Password,
		PasskeyPath:  conn.PasskeyPath,
		SudoPassword: conn.SudoPassword,
	}

	if !bundle.emptySecret() {
		if masterPassword == "" {
			return fmt.Errorf("saving a password/passkey/sudo password: %w", ErrMasterPasswordRequired)
		}
		bundle.ServerName = conn.ServerName
		bundle.User = conn.User
		bundle.Address = conn.Address
		bundle.Port = conn.Port

		jsonData, err := json.Marshal(bundle)
		if err != nil {
			return fmt.Errorf("failed to marshal secret: %w", err)
		}
		encrypted, err := crypto.Encrypt(jsonData, masterPassword)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret: %w", err)
		}
		od.EncryptedSecret = encrypted
		od.HasPasskey = conn.PasskeyPath != ""
	} else {
		od.ServerName = conn.ServerName
		od.User = conn.User
		od.Address = conn.Address
		od.Port = conn.Port
	}

	data, err := json.MarshalIndent(od, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}
	if err := os.WriteFile(fs.connectionFilePath(conn.ID), data, 0600); err != nil {
		return fmt.Errorf("failed to write connection file: %w", err)
	}

	return nil
}

func (fs *FileStorage) readOnDisk(id string) (onDiskConnection, error) {
	var od onDiskConnection
	data, err := os.ReadFile(fs.connectionFilePath(id))
	if err != nil {
		return od, err
	}
	if err := json.Unmarshal(data, &od); err != nil {
		return od, err
	}
	return od, nil
}

// toModel builds the in-memory Connection for od. If od has a sealed
// secret, masterPassword is required to decrypt ServerName/User/Address/
// Port along with it; without it (or if it's wrong), the returned
// Connection is Locked with placeholder fields, and an error is returned
// alongside it - callers that only need to know a connection exists (e.g.
// listing) can use the Connection and ignore the error.
func toModel(od onDiskConnection, masterPassword string) (*models.Connection, error) {
	conn := &models.Connection{
		ID:               od.ID,
		CreatedAt:        od.CreatedAt,
		UpdatedAt:        od.UpdatedAt,
		UseSSHAgent:      od.UseSSHAgent,
		SSHAgentSocket:   od.SSHAgentSocket,
		UseOnePassword:   od.UseOnePassword,
		OnePasswordVault: od.OnePasswordVault,
		OnePasswordItem:  od.OnePasswordItem,
		HasPasskey:       od.HasPasskey,
		HasStoredSecret:  od.EncryptedSecret != "",
	}

	if od.EncryptedSecret == "" {
		conn.ServerName = od.ServerName
		conn.User = od.User
		conn.Address = od.Address
		conn.Port = od.Port
		return conn, nil
	}

	lock := func(err error) (*models.Connection, error) {
		conn.Locked = true
		conn.ServerName = "Locked"
		return conn, err
	}

	if masterPassword == "" {
		return lock(ErrMasterPasswordRequired)
	}

	jsonData, err := crypto.Decrypt(od.EncryptedSecret, masterPassword)
	if err != nil {
		return lock(fmt.Errorf("failed to decrypt connection: %w", err))
	}
	var bundle secretBundle
	if err := json.Unmarshal(jsonData, &bundle); err != nil {
		return lock(fmt.Errorf("failed to unmarshal connection: %w", err))
	}
	conn.ServerName = bundle.ServerName
	conn.User = bundle.User
	conn.Address = bundle.Address
	conn.Port = bundle.Port
	conn.Password = bundle.Password
	conn.PasskeyPath = bundle.PasskeyPath
	conn.SudoPassword = bundle.SudoPassword
	return conn, nil
}

// LoadConnection loads a connection by ID, decrypting it if it's sealed
// and masterPassword is supplied and correct. See toModel.
func (fs *FileStorage) LoadConnection(id string, masterPassword string) (*models.Connection, error) {
	od, err := fs.readOnDisk(id)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("connection not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read connection file: %w", err)
	}
	return toModel(od, masterPassword)
}

// ListConnections loads every connection. Sealed connections are decrypted
// opportunistically if masterPassword is already available (e.g. an
// already-unlocked session); otherwise they come back Locked rather than
// causing an error, so browsing the list never forces a master-password
// prompt by itself.
func (fs *FileStorage) ListConnections(masterPassword string) ([]*models.Connection, error) {
	entries, err := os.ReadDir(fs.connectionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*models.Connection{}, nil
		}
		return nil, fmt.Errorf("failed to read connections directory: %w", err)
	}

	var connections []*models.Connection
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ConnectionFileExt {
			continue
		}

		id := entry.Name()[:len(entry.Name())-len(ConnectionFileExt)]
		od, err := fs.readOnDisk(id)
		if err != nil {
			// Skip connections that fail to load
			continue
		}
		conn, _ := toModel(od, masterPassword)
		connections = append(connections, conn)
	}

	return connections, nil
}

// DeleteConnection removes a connection by ID
func (fs *FileStorage) DeleteConnection(id string) error {
	if err := os.Remove(fs.connectionFilePath(id)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("connection not found: %s", id)
		}
		return fmt.Errorf("failed to delete connection: %w", err)
	}
	return nil
}

// ConnectionExists checks if a connection with the given ID exists
func (fs *FileStorage) ConnectionExists(id string) (bool, error) {
	_, err := os.Stat(fs.connectionFilePath(id))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check connection: %w", err)
	}
	return true, nil
}

// GetConfigPath returns the path to the config file
func (fs *FileStorage) GetConfigPath() string {
	return filepath.Join(fs.storageDir, ConfigFileName)
}
