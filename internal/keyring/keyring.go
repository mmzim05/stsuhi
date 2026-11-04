// Package keyring keeps stsuhi's master key in the OS keyring (Secret
// Service, macOS Keychain or Windows Credential Manager).
package keyring

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// ServiceName groups every stsuhi entry in the keyring.
const ServiceName = "stsuhi"

// DefaultAccount is the entry for the default data directory. It is the
// name older versions used for the auto-login copy of the master password,
// so an existing cached password keeps working.
const DefaultAccount = "master-password"

// ErrNotFound is returned by GetPassword when the entry doesn't exist.
var ErrNotFound = errors.New("no master key in keyring")

// SavePassword stores password in the keyring entry account
func SavePassword(account, password string) error {
	if err := keyring.Set(ServiceName, account, password); err != nil {
		return formatKeyringError("save", err)
	}
	return nil
}

// GetPassword reads the keyring entry account
func GetPassword(account string) (string, error) {
	password, err := keyring.Get(ServiceName, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", formatKeyringError("read", err)
	}
	return password, nil
}

// DeletePassword removes the keyring entry account. A missing entry is
// not an error.
func DeletePassword(account string) error {
	err := keyring.Delete(ServiceName, account)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return formatKeyringError("delete", err)
	}
	return nil
}

// IsAvailable reports whether a keyring is running and writable, by
// writing and removing a test entry
func IsAvailable() bool {
	const testAccount = "test-availability-check"
	if err := keyring.Set(ServiceName, testAccount, "test"); err != nil {
		return false
	}
	return keyring.Delete(ServiceName, testAccount) == nil
}

// formatKeyringError turns common Secret Service failures into advice
func formatKeyringError(operation string, err error) error {
	message := err.Error()
	if strings.Contains(message, "not activatable") ||
		strings.Contains(message, "Cannot autolaunch") {
		return fmt.Errorf("keyring not available: install gnome-keyring "+
			"or kwallet and make sure it is running: %w", err)
	}
	if strings.Contains(message, "Failed to unlock") ||
		strings.Contains(message, "denied") {
		return fmt.Errorf("keyring access denied, unlock your keyring "+
			"first: %w", err)
	}
	return fmt.Errorf("%s master key in keyring: %w", operation, err)
}
