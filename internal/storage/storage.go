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

	// SaveConnection writes a connection's metadata in the clear and
	// encrypts any local secret (password/passkey/sudo password) with
	// masterPassword. masterPassword may be "" if the connection has no
	// local secret to encrypt, or if an existing encrypted secret is being
	// carried forward unchanged - see FileStorage.SaveConnection.
	SaveConnection(conn *models.Connection, masterPassword string) error

	// LoadConnection loads a connection by ID, decrypting its secret (if
	// any) with masterPassword. Returns ErrMasterPasswordRequired alongside
	// the metadata-only connection if it has a secret but masterPassword
	// is "".
	LoadConnection(id string, masterPassword string) (*models.Connection, error)

	// ListConnections loads every connection's metadata. Never requires
	// a master password - secrets are never decrypted for listing.
	ListConnections(masterPassword string) ([]*models.Connection, error)

	// DeleteConnection removes a connection by ID
	DeleteConnection(id string) error

	// ConnectionExists checks if a connection with the given ID exists
	ConnectionExists(id string) (bool, error)

	// GetConfigPath returns the path to the config file
	GetConfigPath() string
}
