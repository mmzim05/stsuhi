package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/mmzim05/stsuhi/internal/models"
	"github.com/mmzim05/stsuhi/internal/onepassword"
)

// Connect launches an SSH session using the provided connection details
func Connect(conn *models.Connection) error {
	if conn.UseOnePassword {
		resolved, err := resolveOnePasswordCredential(conn)
		if err != nil {
			return err
		}
		conn = resolved
	}

	if err := validateTarget(conn); err != nil {
		return err
	}

	// A stored password needs the PTY path so it can be auto-typed
	if conn.Password != "" || conn.SudoPassword != "" {
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

// resolveOnePasswordCredential fetches the username/password (and,
// optionally, address/port) for a 1Password-backed connection and returns
// a copy of conn with them filled in, leaving the original (and its stored
// connection file) untouched. 1Password's values win over whatever's
// stored locally when both are present, same as username has always
// worked - so a connection can leave address/port blank locally and have
// them resolved here instead, keeping nothing but the item/vault
// reference on disk.
func resolveOnePasswordCredential(conn *models.Connection) (*models.Connection, error) {
	if !onepassword.IsAvailable() {
		return nil, fmt.Errorf("1password CLI (op) not found in PATH")
	}

	cred, err := onepassword.GetCredential(conn.OnePasswordItem, conn.OnePasswordVault)
	if err != nil {
		return nil, fmt.Errorf("1password lookup failed: %w", err)
	}

	resolved := *conn
	if cred.Username != "" {
		resolved.User = cred.Username
	}
	if cred.Address != "" {
		resolved.Address = cred.Address
	}
	if cred.Port != "" {
		if port, err := strconv.Atoi(cred.Port); err == nil && port > 0 {
			resolved.Port = port
		}
	}
	resolved.Password = cred.Password

	if resolved.Address == "" {
		return nil, fmt.Errorf("1password item %q has no address and none is stored locally", conn.OnePasswordItem)
	}

	return &resolved, nil
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
	// Close only fails if already closed, which leaves nothing to clean up
	defer ptmx.Close()

	cols, rows := 80, 24
	if size, err := pty.GetsizeFull(os.Stdin); err == nil {
		cols, rows = int(size.Cols), int(size.Rows)
	}
	sudoPassword := conn.SudoPassword
	if sudoPassword == "" {
		sudoPassword = conn.Password
	}
	term := &terminalIO{out: os.Stdout, pty: ptmx}
	watcher := newPromptWatcher(conn.User, conn.Password, sudoPassword,
		cols, rows, promptActions{
			sendLogin: func(secret string) {
				reportError("auto-fill", term.sendSecret(secret))
			},
			offerSudo: func(secret string) {
				reportError("sudo prompt", term.offerSudo(secret))
			},
			cancelSudo: func() {
				reportError("sudo prompt", term.cancelOffer())
			},
		})
	defer watcher.Stop()

	// Handle terminal size changes
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				return
			}
			if size, err := pty.GetsizeFull(os.Stdin); err == nil {
				watcher.Resize(int(size.Cols), int(size.Rows))
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Set stdin in raw mode
	oldState, err := setRawMode(os.Stdin.Fd())
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer func() {
		if err := restoreMode(os.Stdin.Fd(), oldState); err != nil {
			fmt.Fprintf(os.Stderr, "stsuhi: failed to restore terminal: %v\n", err)
		}
	}()

	// Copy input from user to PTY. It ends when the PTY closes; that error
	// is the normal end of a session, not a failure.
	go func() {
		if err := term.forwardInput(os.Stdin); err != nil {
			return
		}
	}()

	// Read from PTY and write to stdout, watching for password prompts.
	// A read error is the PTY closing at session end.
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			if err := term.writeOut(buf[:n]); err != nil {
				return
			}
			watcher.Write(buf[:n])
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

// reportError prints a non-fatal session error. The terminal is in raw
// mode, so the line needs an explicit carriage return.
func reportError(action string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\nstsuhi: %s failed: %v\r\n", action, err)
	}
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

	// Point ssh at a specific agent socket (e.g. 1Password's SSH agent)
	// instead of relying on SSH_AUTH_SOCK / ~/.ssh/config
	if conn.UseSSHAgent && conn.SSHAgentSocket != "" {
		args = append(args, "-o", fmt.Sprintf("IdentityAgent=%s", conn.SSHAgentSocket))
	}

	// "--" ends option parsing, so the target can never be read as a flag
	target := fmt.Sprintf("%s@%s", conn.User, conn.Address)
	args = append(args, "--", target)

	return args
}

// validateTarget rejects a user or address that ssh could read as an
// option (e.g. "-oProxyCommand=...") or that holds spaces or control
// characters. Values can come from a 1Password item, which may be shared,
// so they are checked here right before ssh runs, not only in the form.
func validateTarget(conn *models.Connection) error {
	fields := map[string]string{"user": conn.User, "address": conn.Address}
	for name, value := range fields {
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("invalid %s %q: must not start with '-'", name, value)
		}
		if strings.ContainsFunc(value, isUnsafeRune) {
			return fmt.Errorf("invalid %s %q: contains spaces or control characters", name, value)
		}
	}
	if strings.Contains(conn.Address, "@") {
		return fmt.Errorf("invalid address %q: must not contain '@'", conn.Address)
	}
	return nil
}

func isUnsafeRune(r rune) bool {
	return r <= ' ' || r == 0x7f
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
