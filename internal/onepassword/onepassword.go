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

// Credential is everything a connection can source from a 1Password item:
// not just username/password, but optionally the host/port too, so a
// fully 1Password-backed connection can leave nothing but the item/vault
// reference stored locally. Address and Port come from whichever of the
// "url"/"endpoint"/"website" fields is present (see addressPortLabels),
// formatted as "address:port" (port optional) - most Login items won't
// have one, and that's fine, callers fall back to whatever's stored
// locally in that case.
type Credential struct {
	Username string
	Password string
	Address  string
	Port     string // raw string from 1Password; empty if the matched field had no port
}

// addressPortLabels are the field labels GetCredential checks for the
// host/port, in priority order (first one present on the item wins).
// "website" matches 1Password's own built-in URL field on Login items, so
// an item that already has one filled in works without adding anything.
var addressPortLabels = map[string]bool{"url": true, "endpoint": true, "website": true}

// GetCredential looks up the named 1Password item's "username", "password",
// and optional host/port ("address:port", under a "url"/"endpoint"/
// "website" field label) fields, so a connection's SSH login (and,
// optionally, its host/port) can be sourced directly from a 1Password
// vault item instead of stored locally. If vault is empty, `op` searches
// all vaults the signed-in account can see. Same non-blocking timeout/
// no-stdin guard as GetPassword.
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
	args = append(args, "--fields", "label=username,label=password,label=url,label=endpoint,label=website", "--reveal", "--format", "json")

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
		switch {
		case f.Label == "username":
			cred.Username = f.Value
		case f.Label == "password":
			cred.Password = f.Value
		case addressPortLabels[f.Label] && cred.Address == "":
			cred.Address, cred.Port = parseAddressPort(f.Value)
		}
	}

	if cred.Password == "" {
		return Credential{}, fmt.Errorf("1password item %q has no password field", itemName)
	}

	return cred, nil
}

// parseAddressPort splits a "url" field value formatted as "address:port"
// (port optional) into its two parts.
func parseAddressPort(value string) (address, port string) {
	value = strings.TrimSpace(value)
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		return value[:idx], value[idx+1:]
	}
	return value, ""
}
