package storage

import "stsuhi/internal/models"

// Storage defines the interface for connection persistence
type Storage interface {
	// InitStorage creates the storage directory structure
	InitStorage() error

	// IsMasterPasswordSet checks if a master password has been configured
	IsMasterPasswordSet() (bool, error)

	// SetMasterPassword stores the hashed master password
	SetMasterPassword(password string) error

	// VerifyMasterPassword checks if the provided password matches the stored hash
	VerifyMasterPassword(password string) (bool, error)

	// SaveConnection encrypts and saves a connection
	SaveConnection(conn *models.Connection, masterPassword string) error

	// LoadConnection loads and decrypts a connection by ID
	LoadConnection(id string, masterPassword string) (*models.Connection, error)

	// ListConnections loads all connections
	ListConnections(masterPassword string) ([]*models.Connection, error)

	// DeleteConnection removes a connection by ID
	DeleteConnection(id string) error

	// ConnectionExists checks if a connection with the given ID exists
	ConnectionExists(id string) (bool, error)

	// GetConfigPath returns the path to the config file
	GetConfigPath() string
}
