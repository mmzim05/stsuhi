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
	StorageDirName     = ".stsuhi"
	ConnectionsDirName = "connections"
	MasterKeyFileName  = "master.key"
	ConfigFileName     = "config.json"
	ConnectionFileExt  = ".enc"
)

// FileStorage implements Storage interface using encrypted files
type FileStorage struct {
	storageDir     string
	connectionsDir string
	masterKeyPath  string
}

// NewFileStorage creates a new FileStorage instance
func NewFileStorage() (*FileStorage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, StorageDirName)
	connectionsDir := filepath.Join(storageDir, ConnectionsDirName)
	masterKeyPath := filepath.Join(storageDir, MasterKeyFileName)

	return &FileStorage{
		storageDir:     storageDir,
		connectionsDir: connectionsDir,
		masterKeyPath:  masterKeyPath,
	}, nil
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

// SaveConnection encrypts and saves a connection
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

	// Encrypt data
	encrypted, err := crypto.Encrypt(jsonData, masterPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt connection: %w", err)
	}

	// Write to file
	filePath := filepath.Join(fs.connectionsDir, conn.ID+ConnectionFileExt)
	if err := os.WriteFile(filePath, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("failed to write connection file: %w", err)
	}

	return nil
}

// LoadConnection loads and decrypts a connection by ID
func (fs *FileStorage) LoadConnection(id string, masterPassword string) (*models.Connection, error) {
	filePath := filepath.Join(fs.connectionsDir, id+ConnectionFileExt)

	// Read encrypted file
	encryptedData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("connection not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read connection file: %w", err)
	}

	// Decrypt data
	jsonData, err := crypto.Decrypt(string(encryptedData), masterPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt connection: %w", err)
	}

	// Deserialize from JSON
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

	var connections []*models.Connection
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ConnectionFileExt {
			continue
		}

		id := entry.Name()[:len(entry.Name())-len(ConnectionFileExt)]
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
	filePath := filepath.Join(fs.connectionsDir, id+ConnectionFileExt)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("connection not found: %s", id)
		}
		return fmt.Errorf("failed to delete connection: %w", err)
	}
	return nil
}

// ConnectionExists checks if a connection with the given ID exists
func (fs *FileStorage) ConnectionExists(id string) (bool, error) {
	filePath := filepath.Join(fs.connectionsDir, id+ConnectionFileExt)
	_, err := os.Stat(filePath)
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
