package ssh

import (
	"io"
	"sync"
)

const (
	// Save cursor, show the hint after the prompt, so it can be erased
	// later without touching the remote screen.
	sudoHint      = "\x1b7\x1b[2m  [stsuhi] insert saved sudo password? [y/N]\x1b[0m"
	sudoHintClear = "\x1b8\x1b[K"
)

// terminalIO sits between the local terminal and the PTY. It serializes
// writes to the local terminal and asks the user before a saved sudo
// password is typed: at a sudo prompt the next keypress decides, "y"
// inserts the password and any other key is passed through as typed.
type terminalIO struct {
	mu      sync.Mutex
	out     io.Writer // local terminal
	pty     io.Writer // remote session
	pending string    // sudo password on offer; empty when no offer shown
}

// writeOut copies remote output to the local terminal
func (t *terminalIO) writeOut(p []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, err := t.out.Write(p)
	return err
}

// sendSecret types secret into the remote session
func (t *terminalIO) sendSecret(secret string) error {
	_, err := io.WriteString(t.pty, secret+"\n")
	return err
}

// offerSudo shows the confirmation hint for secret
func (t *terminalIO) offerSudo(secret string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending != "" {
		return nil
	}
	t.pending = secret
	_, err := io.WriteString(t.out, sudoHint)
	return err
}

// cancelOffer withdraws the hint, e.g. once the sudo prompt is gone, so a
// later "y" typed into a shell can never send the password
func (t *terminalIO) cancelOffer() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.clearLocked()
}

func (t *terminalIO) clearLocked() error {
	if t.pending == "" {
		return nil
	}
	t.pending = ""
	_, err := io.WriteString(t.out, sudoHintClear)
	return err
}

// forwardInput copies the user's keystrokes to the remote session until
// in fails, answering a pending sudo offer with the first key
func (t *terminalIO) forwardInput(in io.Reader) error {
	buf := make([]byte, 1024)
	for {
		n, err := in.Read(buf)
		if err != nil {
			return err
		}
		if err := t.handleInput(buf[:n]); err != nil {
			return err
		}
	}
}

func (t *terminalIO) handleInput(p []byte) error {
	t.mu.Lock()
	secret := t.pending
	if secret == "" {
		t.mu.Unlock()
		_, err := t.pty.Write(p)
		return err
	}
	clearErr := t.clearLocked()
	t.mu.Unlock()
	if clearErr != nil {
		return clearErr
	}

	switch p[0] {
	case 'y', 'Y':
		return t.sendSecret(secret)
	case 'n', 'N':
		p = p[1:]
	}
	if len(p) == 0 {
		return nil
	}
	_, err := t.pty.Write(p)
	return err
}
