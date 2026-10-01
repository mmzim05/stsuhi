package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"stsuhi/internal/crypto"
	"stsuhi/internal/models"
)

const (
	ConnectionsDirName = "connections"
	MasterKeyFileName  = "master.key"
	ConfigFileName     = "config.json"
	ConnectionFileExt  = ".enc"

	// PlainConnectionFileExt is used instead of ConnectionFileExt when
	// plainText mode is on (1Password mode), so plaintext and encrypted
	// connection files never collide if a directory has both from a past
	// mode switch - each mode only ever sees its own files.
	PlainConnectionFileExt = ".json"
)

// FileStorage implements Storage interface using encrypted files
type FileStorage struct {
	storageDir     string
	connectionsDir string
	masterKeyPath  string
	plainText      bool
}

// SetPlainTextMode toggles plaintext (1Password mode) storage. See the
// Storage interface doc comment.
func (fs *FileStorage) SetPlainTextMode(plainText bool) {
	fs.plainText = plainText
}

// connectionFileExt returns the extension for the currently active mode.
func (fs *FileStorage) connectionFileExt() string {
	if fs.plainText {
		return PlainConnectionFileExt
	}
	return ConnectionFileExt
}

func (fs *FileStorage) connectionFilePath(id string) string {
	return filepath.Join(fs.connectionsDir, id+fs.connectionFileExt())
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
	// Create storage directory with restricted permissions
	if err := os.MkdirAll(fs.storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create connections directory
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

	// Write hash to file with restricted permissions
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

// SaveConnection saves a connection, encrypted unless plainText mode is on
func (fs *FileStorage) SaveConnection(conn *models.Connection, masterPassword string) error {
	if err := conn.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Update timestamp
	conn.Update()

	// Serialize to JSON
	jsonData, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	out := jsonData
	if !fs.plainText {
		encrypted, err := crypto.Encrypt(jsonData, masterPassword)
		if err != nil {
			return fmt.Errorf("failed to encrypt connection: %w", err)
		}
		out = []byte(encrypted)
	}

	if err := os.WriteFile(fs.connectionFilePath(conn.ID), out, 0600); err != nil {
		return fmt.Errorf("failed to write connection file: %w", err)
	}

	return nil
}

// LoadConnection loads a connection by ID, decrypting unless plainText mode is on
func (fs *FileStorage) LoadConnection(id string, masterPassword string) (*models.Connection, error) {
	data, err := os.ReadFile(fs.connectionFilePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("connection not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read connection file: %w", err)
	}

	jsonData := data
	if !fs.plainText {
		jsonData, err = crypto.Decrypt(string(data), masterPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt connection: %w", err)
		}
	}

	var conn models.Connection
	if err := json.Unmarshal(jsonData, &conn); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection: %w", err)
	}

	return &conn, nil
}

// ListConnections loads all connections
func (fs *FileStorage) ListConnections(masterPassword string) ([]*models.Connection, error) {
	entries, err := os.ReadDir(fs.connectionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*models.Connection{}, nil
		}
		return nil, fmt.Errorf("failed to read connections directory: %w", err)
	}

	ext := fs.connectionFileExt()
	var connections []*models.Connection
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ext {
			continue
		}

		id := entry.Name()[:len(entry.Name())-len(ext)]
		conn, err := fs.LoadConnection(id, masterPassword)
		if err != nil {
			// Skip connections that fail to load
			continue
		}
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
