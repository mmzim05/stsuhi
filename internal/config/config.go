package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// UnlockConfig says whether the master password is read from a 1Password
// item. It only matters on machines without a keyring, where the password
// would otherwise be asked for. The JSON names are those older versions
// used for auto-login, so an existing 1Password setup keeps working.
type UnlockConfig struct {
	Enabled             bool   `json:"enabled"`
	Backend             string `json:"backend,omitempty"`
	OnePasswordItemName string `json:"onepassword_item_name,omitempty"`
}

// BackendOnePassword marks the 1Password unlock in UnlockConfig.Backend.
const BackendOnePassword = "1password"

// UsesOnePassword reports whether the master password comes from 1Password
func (u UnlockConfig) UsesOnePassword() bool {
	return u.Enabled && u.Backend == BackendOnePassword
}

// Config represents the application configuration
type Config struct {
	Unlock UnlockConfig `json:"auto_login"`

	// Theme selects the UI color palette: "" (adaptive built-in default),
	// "omarchy" (follow the desktop's current Omarchy theme), or
	// "omarchy:<slug>" (one specific Omarchy theme). See internal/theme.
	Theme string `json:"theme,omitempty"`
}

// GetDefault returns the default configuration
func GetDefault() *Config {
	return &Config{}
}

// Load reads the configuration from a file
func Load(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return GetDefault(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		// A corrupt config shouldn't lock the user out; start from defaults.
		return GetDefault(), nil
	}

	return &config, nil
}

// Save writes the configuration to a file
func Save(configPath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 0600 like every file under the storage directory
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
