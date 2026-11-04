package ssh

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hinshun/vt10x"
)

// promptQuiet is how long output has to stop before the screen is checked
// for a prompt. A real prompt sits idle waiting for input; checking mid-burst
// could catch a redraw at the moment its cursor passes an old prompt.
const promptQuiet = 50 * time.Millisecond

// promptWatcher decides when to auto-type the SSH login password and when
// to offer the sudo password (the user confirms each sudo insert). It keeps a
// virtual copy of the screen and only treats a prompt as live when the
// cursor is sitting right after it. Matching raw output instead misfires
// on full-screen apps (e.g. a multiplexer redrawing a tab that still shows
// an old "[sudo] password for" line), but those apps put the cursor back
// where the pane's real cursor is, so a live prompt inside them still works.
type promptWatcher struct {
	vt         vt10x.Terminal
	pending    []byte // trailing bytes of a UTF-8 rune split across reads
	sudoPrompt *regexp.Regexp
	login      string // SSH login password; empty skips the login prompt
	sudo       string // sudo password; empty never answers sudo
	sendLogin  func(secret string)
	offerSudo  func(secret string)
	cancelSudo func()
	timer      *time.Timer

	mu        sync.Mutex
	loginSent bool // SSH login password goes out at most once
	answered  bool // current sudo prompt already offered; cleared once it's gone
}

// promptActions are what a promptWatcher does at each kind of prompt
type promptActions struct {
	sendLogin  func(secret string)
	offerSudo  func(secret string)
	cancelSudo func()
}

func newPromptWatcher(user, login, sudo string, cols, rows int, actions promptActions) *promptWatcher {
	w := &promptWatcher{
		vt: vt10x.New(vt10x.WithSize(cols, rows)),
		// Only auto-fill sudo for the SSH user (not docker containers, etc.)
		// Format: "[sudo] password for username:"
		sudoPrompt: regexp.MustCompile(`\[sudo\] password for ` + regexp.QuoteMeta(strings.ToLower(user)) + `:$`),
		login:      login,
		sudo:       sudo,
		sendLogin:  actions.sendLogin,
		offerSudo:  actions.offerSudo,
		cancelSudo: actions.cancelSudo,
		loginSent:  login == "",
	}
	w.timer = time.AfterFunc(time.Hour, w.check)
	w.timer.Stop()
	return w
}

// Write feeds PTY output to the virtual screen and re-arms the quiet timer
func (w *promptWatcher) Write(p []byte) {
	data := append(w.pending, p...)
	n, err := w.vt.Write(data)
	if err != nil {
		// The virtual screen is only a guide for prompt detection, so a
		// bad write drops the chunk rather than the session.
		w.pending = nil
		w.timer.Reset(promptQuiet)
		return
	}
	// Keep the tail of a rune split across reads for the next write
	w.pending = append([]byte(nil), data[n:]...)
	w.timer.Reset(promptQuiet)
}

func (w *promptWatcher) Resize(cols, rows int) {
	w.vt.Resize(cols, rows)
}

func (w *promptWatcher) Stop() {
	w.timer.Stop()
}

func (w *promptWatcher) check() {
	line := w.lineBeforeCursor()

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.loginSent {
		if strings.HasSuffix(line, "password:") {
			w.sendLogin(w.login)
			w.loginSent = true
		}
		return
	}

	if w.sudo == "" || !w.sudoPrompt.MatchString(line) {
		if w.answered {
			w.cancelSudo()
		}
		w.answered = false
		return
	}
	if w.answered {
		return
	}
	w.offerSudo(w.sudo)
	w.answered = true
}

// lineBeforeCursor returns the cursor row's text up to the cursor,
// lowercased with trailing spaces trimmed
func (w *promptWatcher) lineBeforeCursor() string {
	w.vt.Lock()
	defer w.vt.Unlock()
	c := w.vt.Cursor()
	cols, rows := w.vt.Size()
	if c.Y < 0 || c.Y >= rows {
		return ""
	}
	var b strings.Builder
	for x := 0; x < c.X && x < cols; x++ {
		r := w.vt.Cell(x, c.Y).Char
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(strings.ToLower(b.String()), " ")
}
