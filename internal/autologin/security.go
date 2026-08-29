package autologin

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"stsuhi/internal/config"
	"stsuhi/internal/keyring"
)

// GetBootID returns a platform-specific boot identifier
func GetBootID() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return getLinuxBootID()
	case "darwin":
		return getMacOSBootID()
	case "windows":
		return getWindowsBootID()
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// getLinuxBootID reads the boot ID from /proc/sys/kernel/random/boot_id
func getLinuxBootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("failed to read boot_id: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// getMacOSBootID gets the boot time from sysctl
func getMacOSBootID() (string, error) {
	cmd := exec.Command("sysctl", "-n", "kern.boottime")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get boot time: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getWindowsBootID gets the last boot up time from systeminfo
func getWindowsBootID() (string, error) {
	cmd := exec.Command("systeminfo", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to uptime command if available
		return getWindowsUptime()
	}

	// Parse CSV output to extract boot time
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// System Boot Time is typically in the output
		return strings.TrimSpace(lines[0]), nil
	}

	return getWindowsUptime()
}

// getWindowsUptime fallback for Windows
func getWindowsUptime() (string, error) {
	cmd := exec.Command("wmic", "os", "get", "LastBootUpTime")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get boot time: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		return strings.TrimSpace(lines[1]), nil
	}

	return "", fmt.Errorf("failed to parse boot time")
}

// GetSessionID returns a platform-specific session identifier
func GetSessionID() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return os.Getenv("XDG_SESSION_ID"), nil
	case "darwin":
		// On macOS, use combination of USER and console owner
		return os.Getenv("USER"), nil
	case "windows":
		return os.Getenv("SESSIONNAME"), nil
	default:
		return "", nil
	}
}

// ShouldClearAutoLogin checks if auto-login should be cleared based on security rules
func ShouldClearAutoLogin(cfg *config.Config) (bool, string) {
	rules := cfg.AutoLogin.SecurityRules

	// If "never clear" is set, don't clear
	if rules.NeverClear {
		return false, ""
	}

	// Check reboot
	if rules.ClearOnReboot {
		currentBootID, err := GetBootID()
		if err == nil && currentBootID != "" && cfg.AutoLogin.BootID != "" {
			if currentBootID != cfg.AutoLogin.BootID {
				return true, "system was rebooted"
			}
		}
	}

	// Check session/lock
	if rules.ClearOnLock {
		currentSessionID, err := GetSessionID()
		if err == nil && currentSessionID != "" && cfg.AutoLogin.SessionID != "" {
			if currentSessionID != cfg.AutoLogin.SessionID {
				return true, "session changed (logout/lock detected)"
			}
		}
	}

	// Check idle timeout
	if rules.ClearOnIdle && rules.IdleTimeoutHours > 0 {
		if !cfg.AutoLogin.LastLoginTime.IsZero() {
			elapsed := time.Since(cfg.AutoLogin.LastLoginTime)
			timeout := time.Duration(rules.IdleTimeoutHours) * time.Hour
			if elapsed > timeout {
				return true, fmt.Sprintf("idle timeout exceeded (%d hours)", rules.IdleTimeoutHours)
			}
		}
	}

	return false, ""
}

// UpdateLoginMetadata updates config with current boot ID, session ID, and timestamp
func UpdateLoginMetadata(cfg *config.Config) error {
	bootID, err := GetBootID()
	if err == nil {
		cfg.AutoLogin.BootID = bootID
	}

	sessionID, err := GetSessionID()
	if err == nil {
		cfg.AutoLogin.SessionID = sessionID
	}

	cfg.AutoLogin.LastLoginTime = time.Now()

	return nil
}

// ClearAutoLogin disables auto-login and clears the keyring
func ClearAutoLogin(cfg *config.Config) error {
	// Only the keyring backend ever has a secret to remove - stsuhi never
	// writes anything to 1Password itself.
	if cfg.AutoLogin.EffectiveBackend() == config.BackendKeyring {
		if err := keyring.DeletePassword(); err != nil {
			// Don't fail if already deleted
			fmt.Printf("Warning: failed to delete from keyring: %v\n", err)
		}
	}

	// Disable in config
	cfg.AutoLogin.Enabled = false
	cfg.AutoLogin.BootID = ""
	cfg.AutoLogin.SessionID = ""
	cfg.AutoLogin.LastLoginTime = time.Time{}
	cfg.AutoLogin.Backend = config.BackendKeyring
	cfg.AutoLogin.OnePasswordItemName = ""

	return nil
}
