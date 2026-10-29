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

// promptWatcher decides when to auto-type the stored password. It keeps a
// virtual copy of the screen and only treats a prompt as live when the
// cursor is sitting right after it. Matching raw output instead misfires
// on full-screen apps (e.g. a multiplexer redrawing a tab that still shows
// an old "[sudo] password for" line), but those apps put the cursor back
// where the pane's real cursor is, so a live prompt inside them still works.
type promptWatcher struct {
	vt         vt10x.Terminal
	pending    []byte // trailing bytes of a UTF-8 rune split across reads
	sudoPrompt *regexp.Regexp
	send       func()
	timer      *time.Timer

	mu        sync.Mutex
	loginSent bool // SSH login password goes out at most once
	answered  bool // current prompt already answered; cleared once it's gone
}

func newPromptWatcher(user string, cols, rows int, send func()) *promptWatcher {
	w := &promptWatcher{
		vt: vt10x.New(vt10x.WithSize(cols, rows)),
		// Only auto-fill sudo for the SSH user (not docker containers, etc.)
		// Format: "[sudo] password for username:"
		sudoPrompt: regexp.MustCompile(`\[sudo\] password for ` + regexp.QuoteMeta(strings.ToLower(user)) + `:$`),
		send:       send,
	}
	w.timer = time.AfterFunc(time.Hour, w.check)
	w.timer.Stop()
	return w
}

// Write feeds PTY output to the virtual screen and re-arms the quiet timer
func (w *promptWatcher) Write(p []byte) {
	data := append(w.pending, p...)
	n, _ := w.vt.Write(data)
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

	var match bool
	if !w.loginSent {
		match = strings.HasSuffix(line, "password:")
	} else {
		match = w.sudoPrompt.MatchString(line)
	}
	if !match {
		w.answered = false
		return
	}
	if w.answered {
		return
	}
	w.send()
	w.loginSent = true
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
