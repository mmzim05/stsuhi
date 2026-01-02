package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"stsuhi/internal/models"
)

// Connect launches an SSH session using the provided connection details
func Connect(conn *models.Connection) error {
	args := buildSSHArgs(conn)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh connection failed: %w", err)
	}

	return nil
}

// buildSSHArgs constructs the SSH command arguments from a connection
func buildSSHArgs(conn *models.Connection) []string {
	args := make([]string, 0)

	// Add port if not default
	if conn.Port != 22 {
		args = append(args, "-p", strconv.Itoa(conn.Port))
	}

	// Add identity file if using key authentication
	if conn.PasskeyPath != "" {
		args = append(args, "-i", conn.PasskeyPath)
	}

	// Add user@host
	target := fmt.Sprintf("%s@%s", conn.User, conn.Address)
	args = append(args, target)

	return args
}

// GetSSHCommand returns the SSH command string for display purposes
func GetSSHCommand(conn *models.Connection) string {
	args := buildSSHArgs(conn)
	cmd := "ssh"
	for _, arg := range args {
		cmd += " " + arg
	}
	return cmd
}
