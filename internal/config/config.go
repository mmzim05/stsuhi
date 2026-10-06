package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SecurityRules defines when auto-login should be cleared
type SecurityRules struct {
	ClearOnReboot    bool `json:"clear_on_reboot"`
	ClearOnLock      bool `json:"clear_on_lock"`
	ClearOnIdle      bool `json:"clear_on_idle"`
	IdleTimeoutHours int  `json:"idle_timeout_hours"`
	NeverClear       bool `json:"never_clear"`
}

// AutoLoginConfig stores auto-login settings
type AutoLoginConfig struct {
	Enabled             bool          `json:"enabled"`
	SecurityRules       SecurityRules `json:"security_rules"`
	LastLoginTime       time.Time     `json:"last_login_timestamp"`
	BootID              string        `json:"boot_id"`
	SessionID           string        `json:"session_id"`
	DontAskAgain        bool          `json:"dont_ask_again"`
	Backend             string        `json:"backend"` // "keyring" (default) or "1password"
	OnePasswordItemName string        `json:"onepassword_item_name,omitempty"`
}

// BackendKeyring and BackendOnePassword identify the supported auto-login backends.
const (
	BackendKeyring     = "keyring"
	BackendOnePassword = "1password"
)

// Config represents the application configuration
type Config struct {
	AutoLogin AutoLoginConfig `json:"auto_login"`

	// Theme selects the UI color palette: "" (adaptive built-in default),
	// "omarchy" (follow the desktop's current Omarchy theme), or
	// "omarchy:<slug>" (one specific Omarchy theme). See internal/theme.
	Theme string `json:"theme,omitempty"`
}

// EffectiveBackend returns the configured auto-login backend, treating an
// empty value (configs written before the backend field existed) as keyring.
func (a *AutoLoginConfig) EffectiveBackend() string {
	if a.Backend == "" {
		return BackendKeyring
	}
	return a.Backend
}

// GetDefault returns the default configuration
func GetDefault() *Config {
	return &Config{
		AutoLogin: AutoLoginConfig{
			Enabled: false,
			SecurityRules: SecurityRules{
				ClearOnReboot:    true,
				ClearOnLock:      false,
				ClearOnIdle:      false,
				IdleTimeoutHours: 24,
				NeverClear:       false,
			},
			DontAskAgain: true, // Don't prompt for auto-login by default
			Backend:      BackendKeyring,
		},
	}
}

// Load reads the configuration from a file
func Load(configPath string) (*Config, error) {
	// If file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return GetDefault(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		// If JSON is invalid, return default config
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

	// Write with restricted permissions (user read/write only)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
