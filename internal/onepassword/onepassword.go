package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	binaryName = "op"

	// DefaultItemName is the 1Password item stsuhi looks for when the user
	// hasn't overridden it: an item named "stsuhi" with a field labeled
	// "password" (the default field name for Login/Password category items).
	DefaultItemName = "stsuhi"
	passwordField   = "password"

	readTimeout = 15 * time.Second
)

var ErrItemNameInvalid = errors.New("1password item name cannot be empty")

// IsAvailable checks whether the 1Password CLI is installed and on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath(binaryName)
	return err == nil
}

// ValidateItemName performs a basic sanity check on a 1Password item name.
func ValidateItemName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrItemNameInvalid
	}
	return nil
}

// GetPassword looks up the named 1Password item (any vault) and reads its
// "password" field via `op item get <name> --fields password --reveal`.
// The call is bounded by a timeout and never attaches stdin, so a locked or
// unconfigured `op` (which would otherwise fall back to an interactive
// `op signin` prompt) cannot hang the caller - it just fails.
func GetPassword(itemName string) (string, error) {
	if err := ValidateItemName(itemName); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryName, "item", "get", itemName, "--fields", passwordField, "--reveal")
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("1password lookup timed out - is the desktop app unlocked?")
		}
		return "", fmt.Errorf("failed to read from 1password: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Credential is a username/password pair read from a 1Password item.
type Credential struct {
	Username string
	Password string
}

// GetCredential looks up the named 1Password item's "username" and
// "password" fields, so a connection's SSH login can be sourced directly
// from a 1Password vault item instead of stored locally. If vault is empty,
// `op` searches all vaults the signed-in account can see. Same non-blocking
// timeout/no-stdin guard as GetPassword.
func GetCredential(itemName, vault string) (Credential, error) {
	if err := ValidateItemName(itemName); err != nil {
		return Credential{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()

	args := []string{"item", "get", itemName}
	if strings.TrimSpace(vault) != "" {
		args = append(args, "--vault", vault)
	}
	args = append(args, "--fields", "label=username,label=password", "--reveal", "--format", "json")

	cmd := exec.CommandContext(ctx, binaryName, args...)
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Credential{}, fmt.Errorf("1password lookup timed out - is the desktop app unlocked?")
		}
		return Credential{}, fmt.Errorf("failed to read from 1password: %w", err)
	}

	var fields []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(output, &fields); err != nil {
		return Credential{}, fmt.Errorf("unexpected response from 1password: %w", err)
	}

	var cred Credential
	for _, f := range fields {
		switch f.Label {
		case "username":
			cred.Username = f.Value
		case "password":
			cred.Password = f.Value
		}
	}

	if cred.Password == "" {
		return Credential{}, fmt.Errorf("1password item %q has no password field", itemName)
	}

	return cred, nil
}
