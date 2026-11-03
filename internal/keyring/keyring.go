package keyring

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	ServiceName = "stsuhi"
	Username    = "master-password"
)

// SavePassword stores the master password in the OS keyring
func SavePassword(password string) error {
	err := keyring.Set(ServiceName, Username, password)
	if err != nil {
		return formatKeyringError("save", err)
	}
	return nil
}

// GetPassword retrieves the master password from the OS keyring
func GetPassword() (string, error) {
	password, err := keyring.Get(ServiceName, Username)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("password not found in keyring")
		}
		return "", formatKeyringError("retrieve", err)
	}
	return password, nil
}

// formatKeyringError provides helpful error messages for common keyring issues
func formatKeyringError(operation string, err error) error {
	errStr := err.Error()

	// Check for common Linux Secret Service errors
	if contains(errStr, "not activatable") || contains(errStr, "Cannot autolaunch") {
		return fmt.Errorf("keyring not available - install gnome-keyring or kwallet5 and ensure it's running")
	}

	if contains(errStr, "Failed to unlock") || contains(errStr, "denied") {
		return fmt.Errorf("keyring access denied - unlock your keyring first")
	}

	return fmt.Errorf("failed to %s password in keyring: %w", operation, err)
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// DeletePassword removes the master password from the OS keyring
func DeletePassword() error {
	err := keyring.Delete(ServiceName, Username)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete password from keyring: %w", err)
	}
	return nil
}

// IsAvailable checks if the keyring is available on this system
func IsAvailable() bool {
	// Try to set and delete a test value
	testKey := "test-availability-check"
	err := keyring.Set(ServiceName, testKey, "test")
	if err != nil {
		// If we get the "not activatable" error, keyring is not available
		if contains(err.Error(), "not activatable") || contains(err.Error(), "Cannot autolaunch") {
			return false
		}
		return false
	}
	if err := keyring.Delete(ServiceName, testKey); err != nil {
		return false
	}
	return true
}
