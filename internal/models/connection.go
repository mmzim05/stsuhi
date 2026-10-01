package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmptyServerName      = errors.New("server name cannot be empty")
	ErrEmptyUser            = errors.New("user cannot be empty")
	ErrEmptyAddress         = errors.New("address cannot be empty")
	ErrInvalidPort          = errors.New("port must be between 1 and 65535")
	ErrNoAuthMethod         = errors.New("password, passkey path, SSH agent, or 1Password item must be provided")
	ErrEmptyOnePasswordItem = errors.New("1Password item name cannot be empty")
)

// Connection represents an SSH connection configuration
type Connection struct {
	ID               string    `json:"id"`
	ServerName       string    `json:"server_name"`
	User             string    `json:"user"`
	Address          string    `json:"address"`
	Port             int       `json:"port"`
	Password         string    `json:"password,omitempty"`
	PasskeyPath      string    `json:"passkey_path,omitempty"`
	SudoPassword     string    `json:"sudo_password,omitempty"`
	UseSSHAgent      bool      `json:"use_ssh_agent,omitempty"`
	SSHAgentSocket   string    `json:"ssh_agent_socket,omitempty"`
	UseOnePassword   bool      `json:"use_one_password,omitempty"`
	OnePasswordVault string    `json:"one_password_vault,omitempty"`
	OnePasswordItem  string    `json:"one_password_item,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// NewConnection creates a new Connection with generated ID and timestamps
func NewConnection() *Connection {
	now := time.Now()
	return &Connection{
		ID:        uuid.New().String(),
		Port:      22, // Default SSH port
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the connection has all required fields
func (c *Connection) Validate() error {
	if c.ServerName == "" {
		return ErrEmptyServerName
	}
	if c.User == "" && !c.UseOnePassword {
		return ErrEmptyUser
	}
	if c.Address == "" {
		return ErrEmptyAddress
	}
	if c.Port < 1 || c.Port > 65535 {
		return ErrInvalidPort
	}
	if c.Password == "" && c.PasskeyPath == "" && !c.UseSSHAgent && !c.UseOnePassword {
		return ErrNoAuthMethod
	}
	if c.UseOnePassword && strings.TrimSpace(c.OnePasswordItem) == "" {
		return ErrEmptyOnePasswordItem
	}
	return nil
}

// Update updates the connection's UpdatedAt timestamp
func (c *Connection) Update() {
	c.UpdatedAt = time.Now()
}

// DisplayString returns a formatted string for display in the UI
func (c *Connection) DisplayString() string {
	authMethod := "password"
	if c.PasskeyPath != "" {
		authMethod = "key"
	}
	if c.UseSSHAgent {
		authMethod = "agent"
	}
	if c.UseOnePassword {
		authMethod = "1password"
	}
	user := c.User
	if user == "" && c.UseOnePassword {
		user = "?"
	}
	return fmt.Sprintf("%s (%s@%s:%d) [%s]", c.ServerName, user, c.Address, c.Port, authMethod)
}
