package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"stsuhi/internal/models"
)

// Connect launches an SSH session using the provided connection details
func Connect(conn *models.Connection) error {
	// If we have a password, use PTY-based connection for auto-fill
	if conn.Password != "" {
		return connectWithPTY(conn)
	}

	// No password - use standard SSH (key-based auth)
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

// connectWithPTY handles SSH connection with automatic password input
func connectWithPTY(conn *models.Connection) error {
	args := buildSSHArgs(conn)
	cmd := exec.Command("ssh", args...)

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}
	defer ptmx.Close()

	// Handle terminal size changes
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				return
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Set stdin in raw mode
	oldState, err := setRawMode(os.Stdin.Fd())
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer restoreMode(os.Stdin.Fd(), oldState)

	// Track whether we've seen any password prompts
	sshPasswordSent := false

	// Create a buffer to capture output for password detection
	var outputBuffer strings.Builder

	// Copy input from user to PTY
	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	// Read from PTY and write to stdout, detecting password prompts
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}

			// Write to stdout
			os.Stdout.Write(buf[:n])

			// Add to buffer for password prompt detection
			outputBuffer.Write(buf[:n])

			// Keep only last 500 characters to detect prompts
			if outputBuffer.Len() > 500 {
				s := outputBuffer.String()
				outputBuffer.Reset()
				outputBuffer.WriteString(s[len(s)-500:])
			}

			output := strings.ToLower(outputBuffer.String())

			// Detect SSH password prompt (only send once)
			if !sshPasswordSent && conn.Password != "" {
				if strings.Contains(output, "password:") || strings.Contains(output, "'s password:") {
					ptmx.Write([]byte(conn.Password + "\n"))
					sshPasswordSent = true
					outputBuffer.Reset()
					continue
				}
			}

			// Detect sudo password prompt - use same password as SSH login
			if conn.Password != "" && sshPasswordSent {
				// Check for sudo password prompts in various formats
				if strings.Contains(output, "[sudo]") && strings.Contains(output, "password") {
					ptmx.Write([]byte(conn.Password + "\n"))
					outputBuffer.Reset()
					continue
				}
				// Also match "Password:" or "password:" at end of line (generic sudo prompt)
				// Only do this after SSH password was sent to avoid confusion
				if strings.HasSuffix(strings.TrimSpace(output), "password:") ||
				   strings.HasSuffix(strings.TrimSpace(output), "password: ") {
					ptmx.Write([]byte(conn.Password + "\n"))
					outputBuffer.Reset()
					continue
				}
			}
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		// Exit status 255 usually means connection closed normally
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 255 {
				return nil
			}
		}
		return fmt.Errorf("ssh session ended: %w", err)
	}

	return nil
}

// setRawMode sets the terminal to raw mode
func setRawMode(fd uintptr) (*syscall.Termios, error) {
	oldState, err := getTermios(fd)
	if err != nil {
		return nil, err
	}

	newState := *oldState
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	newState.Iflag &^= syscall.IXON | syscall.ICRNL
	newState.Oflag &^= syscall.OPOST

	if err := setTermios(fd, &newState); err != nil {
		return nil, err
	}

	return oldState, nil
}

// restoreMode restores the terminal to its previous state
func restoreMode(fd uintptr, state *syscall.Termios) error {
	return setTermios(fd, state)
}

// getTermios gets the terminal attributes
func getTermios(fd uintptr) (*syscall.Termios, error) {
	var termios syscall.Termios
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&termios))); err != 0 {
		return nil, err
	}
	return &termios, nil
}

// setTermios sets the terminal attributes
func setTermios(fd uintptr, termios *syscall.Termios) error {
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(termios))); err != 0 {
		return err
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
