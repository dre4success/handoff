// Package notify tells the operator, once, about each alert: on macOS as a
// notification through osascript, and on every system as a line on the
// session's stderr. It writes nothing but marker files in the record
// directory, and it never messages an agent (proposal §8, §9).
package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

// TestEnv names a file that receives notifications instead of the screen,
// one line each, so tests can count them.
const TestEnv = "HANDOFF_TEST_NOTIFY"

// A key becomes a file name, so it may not leave the markers directory.
var safeKey = regexp.MustCompile(`^[a-z0-9-]+$`)

type Notifier struct {
	dir      string // the record directory
	project  string
	errw     io.Writer
	now      func() time.Time
	sent     map[string]bool      // keys this process knows were sent, or can never be
	failures map[string]int       // failed attempts per key, in this process
	retryAt  map[string]time.Time // when this process may try a key again
	asked    map[string]int       // pop-ups started per key, in this process
	getenv   func(string) string

	// Ask, when set, shows a decide alert as a pop-up the operator can answer, in a
	// process of its own. It is handed the alert's lock, already held, and
	// must keep it until the alert is marked sent or given up; that process
	// marks it sent (proposal §17 row 65).
	Ask func(a status.Alert, lock *os.File) error
}

func New(recordDir, project string, errw io.Writer) *Notifier {
	return &Notifier{dir: recordDir, project: project, errw: errw, now: time.Now,
		sent: map[string]bool{}, failures: map[string]int{}, retryAt: map[string]time.Time{},
		asked: map[string]int{}, getenv: os.Getenv}
}

// sandboxed reports whether this process runs in Codex's sandbox, where
// osascript can neither notify nor ask (evidence/h9b/codex-sandbox-probe.txt).
// A process there leaves every alert to one that can show it. Tests that
// send alerts to a file run anywhere, Codex's sandbox included.
func (n *Notifier) sandboxed() bool {
	return n.getenv("CODEX_SANDBOX") != "" && n.getenv(TestEnv) == ""
}

// Send tells the operator each alert that no process has sent yet. An alert is
// used up only by a send that worked: a failure is reported and retried, at
// once the first time, then less and less often.
func (n *Notifier) Send(alerts []status.Alert) {
	if n.sandboxed() {
		return
	}
	now := n.now()
	for _, a := range alerts {
		if n.sent[a.Key] || now.Before(n.retryAt[a.Key]) {
			continue
		}
		if !safeKey.MatchString(a.Key) {
			n.sent[a.Key] = true // never sendable; report it once
			fmt.Fprintf(n.errw, "handoff: alert key %q is unsafe as a file name; not sent\n", a.Key)
			continue
		}
		if err := n.try(a, now); err != nil {
			n.failures[a.Key]++
			n.retryAt[a.Key] = now.Add(backoff(n.failures[a.Key]))
			fmt.Fprintf(n.errw, "handoff: notify: could not tell the operator, will retry: %v\n", err)
		}
	}
}

// try sends one alert, unless it was sent already or another process is
// sending it. Whoever holds <key>.lock (flock, exclusive) owns the alert
// from checking <key> to creating it, so no other process can send in
// between. The kernel drops the lock when its process exits, killed or not,
// and a process suspended by sleep keeps it, so no file's age is trusted.
// The lock files stay: removing one another process has open would let two
// processes lock different files under one name.
func (n *Notifier) try(a status.Alert, now time.Time) error {
	dir := filepath.Join(n.dir, "notified")
	done := filepath.Join(dir, a.Key)
	if _, err := os.Stat(done); err == nil {
		n.sent[a.Key] = true
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(done+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() // closing the file releases the lock
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil // another process is sending it; look again next pass
	}
	if err != nil {
		return fmt.Errorf("lock %s: %w", done+".lock", err)
	}
	if _, err := os.Stat(done); err == nil {
		n.sent[a.Key] = true // sent by the process that held the lock before
		return nil
	}
	if a.Kind == status.Decide && n.Ask != nil {
		// The pop-up's process holds the lock from here and marks the alert
		// sent. If it fails, the alert is free again; this process looks
		// after 1, 2, 4 and 8 minutes, then every 15.
		if err := n.Ask(a, f); err != nil {
			return err
		}
		n.asked[a.Key]++
		n.retryAt[a.Key] = now.Add(backoff(n.asked[a.Key] + 1))
		fmt.Fprintf(n.errw, "handoff: asking the operator in a pop-up: %s\n", a.Text)
		return nil
	}
	if err := show(n.project, a.Text); err != nil {
		return err
	}
	n.sent[a.Key] = true
	fmt.Fprintf(n.errw, "handoff: told the operator: %s\n", a.Text)
	if err := os.WriteFile(done, nil, 0o644); err != nil {
		fmt.Fprintf(n.errw, "handoff: notify: sent, but could not mark %s as sent, so it may be sent again: %v\n", a.Key, err)
	}
	return nil
}

// backoff is the wait before retry number failures. Most failures pass, so
// the first retry is at once; then 1, 2, 4 and 8 minutes, then every 15.
func backoff(failures int) time.Duration {
	if failures <= 1 {
		return 0
	}
	if failures > 5 {
		return 15 * time.Minute
	}
	return time.Minute << (failures - 2)
}

// Path is where key's marker lives; key.lock beside it is its lock.
func Path(recordDir, key string) (string, error) {
	if !safeKey.MatchString(key) {
		return "", fmt.Errorf("alert key %q is unsafe as a file name", key)
	}
	return filepath.Join(recordDir, "notified", key), nil
}

// MarkSent records that key's alert has been dealt with, so no process
// sends it again.
func MarkSent(recordDir, key string) error {
	done, err := Path(recordDir, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(done), 0o755); err != nil {
		return err
	}
	return os.WriteFile(done, nil, 0o644)
}

// Tell shows the operator one notification, outside the once-only bookkeeping: for
// news that is not an alert, such as a click that could not be recorded.
func Tell(project, text string) error { return show(project, text) }

func show(project, text string) error {
	if path := os.Getenv(TestEnv); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f, "%s: %s\n", project, text)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}
	if runtime.GOOS != "darwin" {
		return nil // stderr has it
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// The title and text go in as arguments, never into the script, so no
	// quote in them can change what runs.
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		"handoff: "+project, text).CombinedOutput()
	if err != nil && len(bytes.TrimSpace(out)) > 0 {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(out)) // osascript's own reason
	}
	return err
}
