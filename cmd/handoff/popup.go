package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dre4success/handoff/internal/notify"
	"github.com/dre4success/handoff/internal/status"
)

// The pop-up (proposal §17 rows 59 and 65). When a decision starts waiting
// for the operator, the process that sends the alert starts `handoff _popup` in a
// session of its own, so that it outlives the command, and hands it the
// alert's lock as descriptor 3. The helper shows a dialog with Approve, Deny
// and Later, writes the operator's click as the operator's decision, and marks the
// alert sent. Only a click writes: nothing in the helper's arguments or
// environment answers for the operator.

const popupCmd = "_popup"

// testPopupEnv names a file that receives pop-ups instead of the screen, for
// tests. A pop-up sent there is never clicked: it waits for <file>.release,
// then counts as unanswered. So no setting can answer for the operator.
const testPopupEnv = "HANDOFF_TEST_POPUP"

// popupGiveUp is how long the dialog waits for a click, in seconds. After
// that it closes, and handoff in the project still lists the decision.
const popupGiveUp = 8 * 60 * 60

// dialog shows one dialog and returns the button clicked, and the text typed
// when answer is true. gaveUp is true if nobody clicked in time.
type dialog func(title, text string, buttons []string, answer bool) (button, typed string, gaveUp bool, err error)

// startPopup is the notifier's Ask, on macOS.
func (a *app) startPopup(al status.Alert, lock *os.File) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.Command(self, popupCmd, al.Key, strconv.FormatInt(al.Target, 10))
	cmd.Dir = filepath.Dir(a.dir) // the project, which outlives any worktree
	cmd.Env = os.Environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.ExtraFiles = []*os.File{lock} // descriptor 3
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reaped here if this process lives on, by launchd if not
	return nil
}

// cmdPopup is the helper. It refuses to run unless descriptor 3 is the
// alert's lock and it holds it, so two dialogs never ask the same thing.
func (a *app) cmdPopup(args []string) (int, error) {
	if len(args) != 2 {
		return 0, errors.New("_popup is started by handoff itself")
	}
	key := args[0]
	id, err := parseID(args[1])
	if err != nil {
		return 0, err
	}
	if err := a.open(); err != nil {
		return 0, err
	}
	lock := os.NewFile(3, "lock")
	if err := holdsLock(lock, a.dir, key); err != nil {
		return 0, err
	}
	// The lock lasts as long as this file is open. A file nothing refers to
	// is closed by the garbage collector, which would free the lock while
	// the dialog is still up (GPT-6, H9b review #31).
	defer lock.Close()
	show := osaDialog
	if path := os.Getenv(testPopupEnv); path != "" {
		show = testDialog(path)
	}
	return exitOK, a.popup(key, id, show)
}

func testDialog(path string) dialog {
	return func(title, text string, buttons []string, answer bool) (string, string, bool, error) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", "", false, err
		}
		fmt.Fprintf(f, "%s | %s | %s\n", title, strings.ReplaceAll(text, "\n", " / "), strings.Join(buttons, ","))
		f.Close()
		for range 100 {
			// A real dialog stays up long enough for garbage collection to
			// run, so this one runs it as it waits.
			runtime.GC()
			if _, err := os.Stat(path + ".release"); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		return "", "", true, nil
	}
}

// holdsLock checks that f is key's lock file and that this process holds
// its lock. flock on a descriptor that already holds the lock succeeds; one
// held through another open of the file does not.
func holdsLock(f *os.File, recordDir, key string) error {
	done, err := notify.Path(recordDir, key)
	if err != nil {
		return err
	}
	if f == nil {
		return errors.New("no lock on descriptor 3")
	}
	got, err := f.Stat()
	if err != nil {
		return fmt.Errorf("no lock on descriptor 3: %w", err)
	}
	want, err := os.Stat(done + ".lock")
	if err != nil || !os.SameFile(got, want) {
		return fmt.Errorf("descriptor 3 is not %s.lock", done)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another process holds %s.lock", done)
	}
	return nil
}

// popup asks the operator about entry id and writes the answer. Later, a dialog left
// unanswered, or one closed some other way writes nothing; handoff in the
// project still lists the decision. If the dialog cannot be shown at all,
// the alert is not marked sent, so another process tries again.
func (a *app) popup(key string, id int64, show dialog) error {
	done, err := notify.Path(a.dir, key)
	if err != nil {
		return err
	}
	if _, err := os.Stat(done); err == nil {
		return nil
	}
	snap, err := a.store.Load()
	if err != nil {
		return err
	}
	t, err := status.Compute(snap.Entries).Decidable(id)
	if err != nil {
		return notify.MarkSent(a.dir, key) // decided at a terminal meanwhile
	}
	project := filepath.Base(filepath.Dir(a.dir))
	title := "handoff: " + project
	button, _, gaveUp, err := show(title, popupText(t), []string{"Later", "Deny", "Approve"}, false)
	if err != nil {
		return err
	}
	var decision, body string
	switch {
	case gaveUp:
	case button == "Approve":
		decision, body = "approve", "Approved in the pop-up."
	case button == "Deny":
		b, why, gaveUp, err := show(title, fmt.Sprintf("Deny %s? Say why, if you like.", label(t)), []string{"Cancel", "Deny"}, true)
		if err == nil && !gaveUp && b == "Deny" {
			decision, body = "deny", strings.TrimSpace(why)
			if body == "" {
				body = "Denied in the pop-up."
			}
		}
	}
	if err := notify.MarkSent(a.dir, key); err != nil {
		return err
	}
	if decision == "" {
		return nil
	}
	if _, _, err := a.writeDecision(t.Entry.ID, decision, body); err != nil {
		notify.Tell(project, fmt.Sprintf("Your click on %s was not recorded: %v. Run handoff in the project.", label(t), err))
		return err
	}
	return nil
}

// popupText says what the operator is deciding, as the terminal does: the request
// and who approved it, or the note and what it asks.
func popupText(t *status.Target) string {
	item := t.Entry.Item
	if item == "" {
		item = "-"
	}
	if t.Request != nil {
		who := t.Entry.To
		if t.Request.Verdict != nil {
			who = t.Request.Verdict.From
		}
		return fmt.Sprintf("%s: %s approved #%d.\n\n%s\n\nLater keeps it waiting: run handoff in the project to decide there.",
			item, who, t.Entry.ID, firstLine(t.Entry.Title))
	}
	body := strings.Split(strings.TrimRight(t.Entry.Body, "\n"), "\n")
	if len(body) > 10 {
		body = append(body[:10], fmt.Sprintf("… (handoff show %d for the rest)", t.Entry.ID))
	}
	ask := firstLine(t.Entry.Title)
	if ask == "" {
		ask = "a decision"
	}
	return fmt.Sprintf("%s: %s asks you: %s\n\n%s", item, t.Entry.From, ask, strings.Join(body, "\n"))
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// osaDialog shows the dialog through osascript. The title, text and button
// names go in as arguments, never into the script, so nothing in them can
// change what runs. No button is the default, so a stray Return does not
// answer; the dialog comes to the front.
func osaDialog(title, text string, buttons []string, answer bool) (string, string, bool, error) {
	if runtime.GOOS != "darwin" {
		return "", "", false, errors.New("pop-ups need macOS")
	}
	args := append(dialogScript(len(buttons), answer), title, text)
	out, err := exec.Command("/usr/bin/osascript", append(args, buttons...)...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(string(exit.Stderr), "(-128)") {
			return "Cancel", "", false, nil // the Cancel button, or Escape
		}
		return "", "", false, fmt.Errorf("osascript: %v: %s", err, strings.TrimSpace(string(exitStderr(err))))
	}
	parts := strings.SplitN(strings.TrimSuffix(string(out), "\n"), "\n", 3)
	if len(parts) < 2 {
		return "", "", false, fmt.Errorf("osascript: unexpected answer %q", out)
	}
	typed := ""
	if len(parts) == 3 {
		typed = parts[2]
	}
	return parts[1], typed, parts[0] == "true", nil
}

// dialogScript is the AppleScript osaDialog runs, as osascript's -e
// options. It takes the title, the text and n button names as arguments.
func dialogScript(n int, answer bool) []string {
	var names []string
	for i := range n {
		names = append(names, fmt.Sprintf("item %d of argv", i+3))
	}
	show := fmt.Sprintf("set r to display dialog (item 2 of argv) with title (item 1 of argv) buttons {%s} giving up after %d",
		strings.Join(names, ", "), popupGiveUp)
	ret := "return (gave up of r as text) & linefeed & (button returned of r)"
	if answer {
		show = fmt.Sprintf("set r to display dialog (item 2 of argv) with title (item 1 of argv) default answer \"\" buttons {%s} default button (item %d of argv) giving up after %d",
			strings.Join(names, ", "), n+2, popupGiveUp)
		ret += " & linefeed & (text returned of r)"
	}
	return []string{"-e", "on run argv", "-e", "activate", "-e", show, "-e", ret, "-e", "end run"}
}

func exitStderr(err error) []byte {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.Stderr
	}
	return nil
}
