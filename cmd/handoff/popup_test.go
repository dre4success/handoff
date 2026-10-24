package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

// H9b: the operator answers a decision with a click (proposal §17 rows 59 and 65).
// The dialog is replaced by a script of clicks; the real one is in
// evidence/h9b/.

type click struct {
	button, typed string
	gaveUp        bool
	err           error
}

// clicks answers each dialog in turn and records what it was shown.
func clicks(t *testing.T, shown *[]string, answers ...click) dialog {
	return func(title, text string, buttons []string, answer bool) (string, string, bool, error) {
		*shown = append(*shown, title+" | "+text+" | "+strings.Join(buttons, ","))
		if len(answers) == 0 {
			t.Fatalf("a dialog nobody expected: %q", text)
		}
		c := answers[0]
		answers = answers[1:]
		return c.button, c.typed, c.gaveUp, c.err
	}
}

// app opens the repository's record as the helper does.
func (r *repo) app(t *testing.T) *app {
	t.Helper()
	a := &app{name: popupCmd, stdin: os.Stdin, stdout: io.Discard, stderr: io.Discard, cwd: r.dir}
	if err := a.open(); err != nil {
		t.Fatal(err)
	}
	return a
}

// decideAlert is the record's one decide alert, not yet sent. (The command
// that wrote the APPROVE or the note sent it to the test sink and marked it;
// the helper starts from an alert that no one has shown.)
func (r *repo) decideAlert(t *testing.T) status.Alert {
	t.Helper()
	var found []status.Alert
	for _, al := range status.Compute(r.snap(t).Entries).Alerts(time.Now(), time.Hour) {
		if al.Kind == status.Decide {
			found = append(found, al)
		}
	}
	if len(found) != 1 {
		t.Fatalf("decide alerts: %v; want one", found)
	}
	os.Remove(filepath.Join(r.dir, ".handoff", "notified", found[0].Key))
	return found[0]
}

func (r *repo) marked(key string) bool {
	_, err := os.Stat(filepath.Join(r.dir, ".handoff", "notified", key))
	return err == nil
}

func TestH9bApproveClickDecides(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	al := r.decideAlert(t)
	var shown []string
	if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{button: "Approve"})); err != nil {
		t.Fatal(err)
	}
	if d, body := r.decision(t, req); d != "approve" || body != "Approved in the pop-up." {
		t.Fatalf("decision %q with body %q", d, body)
	}
	want := "W1: reviewer approved #" + req + ".\n\nW1 work"
	if len(shown) != 1 || !strings.Contains(shown[0], want) || !strings.HasSuffix(shown[0], "| Later,Deny,Approve") {
		t.Fatalf("shown %q; want one dialog with %q and Later, Deny, Approve", shown, want)
	}
	if !r.marked(al.Key) {
		t.Fatal("the alert was not marked sent")
	}
}

func TestH9bDenyAsksWhy(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	al := r.decideAlert(t)
	var shown []string
	err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{button: "Deny"}, click{button: "Deny", typed: "  tests are missing "}))
	if err != nil {
		t.Fatal(err)
	}
	if d, body := r.decision(t, req); d != "deny" || body != "tests are missing" {
		t.Fatalf("decision %q with body %q", d, body)
	}
	if len(shown) != 2 || !strings.Contains(shown[1], "Deny W1 (#"+req+")?") || !strings.HasSuffix(shown[1], "| Cancel,Deny") {
		t.Fatalf("shown %q", shown)
	}
}

// Cancel on the second dialog takes the deny back, as does leaving it
// unanswered until it gives up.
func TestH9bDenyThenCancelWritesNothing(t *testing.T) {
	for _, c := range []click{{button: "Cancel"}, {gaveUp: true}} {
		r := newRepo(t)
		req := r.approved(t, "W1")
		al := r.decideAlert(t)
		var shown []string
		if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{button: "Deny"}, c)); err != nil {
			t.Fatal(err)
		}
		if d, _ := r.decision(t, req); d != "" {
			t.Fatalf("%+v: decision %q", c, d)
		}
		if !r.marked(al.Key) {
			t.Fatalf("%+v: the operator saw it; the alert should be marked sent", c)
		}
	}
}

// Later, or no click before the dialog gives up: nothing is written, the
// pop-up is not shown again, and handoff in the project still lists it.
func TestH9bLaterAndGivingUpWriteNothing(t *testing.T) {
	for _, c := range []click{{button: "Later"}, {gaveUp: true}} {
		r := newRepo(t)
		req := r.approved(t, "W1")
		al := r.decideAlert(t)
		var shown []string
		if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, c)); err != nil {
			t.Fatal(err)
		}
		if d, _ := r.decision(t, req); d != "" {
			t.Fatalf("%+v: decision %q", c, d)
		}
		if !r.marked(al.Key) {
			t.Fatalf("%+v: not marked sent, so the pop-up would come back", c)
		}
		if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown)); err != nil || len(shown) != 1 {
			t.Fatalf("%+v: asked again: %v, %q", c, err, shown)
		}
		if out := r.ok(t, "", "inbox", "--for", "operator"); !strings.Contains(out, "APPROVE on #"+req) {
			t.Fatalf("%+v: no longer listed:\n%s", c, out)
		}
	}
}

// A dialog that cannot be shown leaves the alert unmarked, for another try.
func TestH9bDialogFailureLeavesTheAlert(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	al := r.decideAlert(t)
	var shown []string
	if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{err: errors.New("no screen")})); err == nil {
		t.Fatal("want the failure returned")
	}
	if d, _ := r.decision(t, req); d != "" || r.marked(al.Key) {
		t.Fatalf("decision %q, marked %v; want neither", d, r.marked(al.Key))
	}
}

// Decided at a terminal before the pop-up came up: it is not shown.
func TestH9bDecidedMeanwhileIsNotAsked(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	al := r.decideAlert(t)
	id, _ := strconv.ParseInt(req, 10, 64)
	if _, _, err := r.app(t).writeDecision(id, "approve", "at the terminal"); err != nil {
		t.Fatal(err)
	}
	var shown []string
	if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown)); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 0 || !r.marked(al.Key) {
		t.Fatalf("shown %q, marked %v", shown, r.marked(al.Key))
	}
}

// Decided at a terminal while the pop-up was up: the click is refused under
// the record lock, the record keeps the one decision, and the operator is told.
func TestH9bClickAfterATerminalDecisionIsRefused(t *testing.T) {
	told := filepath.Join(t.TempDir(), "told.txt")
	t.Setenv("HANDOFF_TEST_NOTIFY", told)
	r := newRepo(t)
	req := r.approved(t, "W1")
	al := r.decideAlert(t)
	id, _ := strconv.ParseInt(req, 10, 64)
	a := r.app(t)
	meanwhile := func(title, text string, buttons []string, answer bool) (string, string, bool, error) {
		if _, _, err := r.app(t).writeDecision(id, "deny", "at the terminal"); err != nil {
			t.Fatal(err)
		}
		return "Approve", "", false, nil
	}
	if err := a.popup(al.Key, al.Target, meanwhile); err == nil {
		t.Fatal("want the late click refused")
	}
	n := 0
	for _, e := range r.snap(t).Entries {
		if e.Kind == "decision" {
			n++
		}
	}
	if d, body := r.decision(t, req); n != 1 || d != "deny" || body != "at the terminal" {
		t.Fatalf("%d decisions; the one on #%s is %q %q", n, req, d, body)
	}
	if b, _ := os.ReadFile(told); !strings.Contains(string(b), "Your click on W1 (#"+req+") was not recorded") {
		t.Fatalf("The operator was told %q", b)
	}
}

func TestH9bNoteToTheOperator(t *testing.T) {
	r := newRepo(t)
	note := idOf(t, r.ok(t, "implementer", "note", "--item", "W2", "--to", "operator", "--title", "May I deploy?", "--body", r.body(t, "to staging\nafter the merge")))
	al := r.decideAlert(t)
	var shown []string
	if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{button: "Approve"})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown[0], "W2: implementer asks you: May I deploy?\n\nto staging\nafter the merge") {
		t.Fatalf("shown %q", shown)
	}
	if d, _ := r.decision(t, note); d != "approve" {
		t.Fatalf("decision on the note: %q", d)
	}
}

// The helper runs only with the alert's lock on descriptor 3, held by it.
func TestH9bHelperNeedsTheLock(t *testing.T) {
	r := newRepo(t)
	r.approved(t, "W1")
	al := r.decideAlert(t)
	dir := filepath.Join(r.dir, ".handoff")
	if err := holdsLock(nil, dir, al.Key); err == nil {
		t.Fatal("no descriptor: want a refusal")
	}
	other, err := os.Create(filepath.Join(t.TempDir(), "x.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := holdsLock(other, dir, al.Key); err == nil || !strings.Contains(err.Error(), "is not") {
		t.Fatalf("another file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "notified"), 0o755); err != nil {
		t.Fatal(err)
	}
	open := func() *os.File {
		f, err := os.OpenFile(filepath.Join(dir, "notified", al.Key+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	mine, theirs := open(), open()
	if err := syscall.Flock(int(theirs.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := holdsLock(mine, dir, al.Key); err == nil || !strings.Contains(err.Error(), "another process") {
		t.Fatalf("held elsewhere: %v", err)
	}
	theirs.Close()
	if err := holdsLock(mine, dir, al.Key); err != nil {
		t.Fatalf("free: %v", err)
	}
	if err := holdsLock(mine, dir, al.Key); err != nil {
		t.Fatalf("already held by this descriptor: %v", err)
	}
	if res := r.h(t, "", popupCmd, al.Key, strconv.FormatInt(al.Target, 10)); res.code != 2 || !strings.Contains(res.err, "descriptor 3") {
		t.Fatalf("run by hand: exit %d, %q", res.code, res.err)
	}
}

// The whole chain, as separate processes: the reviewer's APPROVE starts the
// helper, which holds the alert's lock while its dialog is up, so another
// process asks nothing; when the dialog goes, the alert is marked sent and
// nothing is decided. HANDOFF_TEST_POPUP's dialog can never click.
func TestH9bPopupAcrossProcesses(t *testing.T) {
	r := newRepo(t)
	popups := filepath.Join(t.TempDir(), "popups.txt")
	notes := filepath.Join(t.TempDir(), "notes.txt")
	envs := []string{"HANDOFF_TEST_POPUP=" + popups, "HANDOFF_TEST_NOTIFY=" + notes}
	req := r.request(t, "W1", r.commitA)
	res := r.run(t, "reviewer", "", envs, "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	if res.code != 0 || !strings.Contains(res.err, "asking the operator in a pop-up: W1: reviewer approved #"+req) {
		t.Fatalf("reply: exit %d, stderr %q", res.code, res.err)
	}
	lines := func() []string {
		b, _ := os.ReadFile(popups)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("no %s within 10 s", what)
			}
		}
	}
	until("pop-up", func() bool { b, _ := os.ReadFile(popups); return len(b) > 0 })
	if got := lines(); len(got) != 1 || !strings.Contains(got[0], "W1: reviewer approved #"+req) {
		t.Fatalf("pop-ups: %q", got)
	}
	al := r.decideAlert(t)
	if r.marked(al.Key) {
		t.Fatal("marked sent while the dialog is still up")
	}
	// The helper keeps the lock while its dialog is up, through the garbage
	// collections the test dialog runs (GPT-6, H9b review #31).
	lockPath := filepath.Join(r.dir, ".handoff", "notified", al.Key+".lock")
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		f.Close()
		if err == nil {
			t.Fatal("the alert's lock was free while its dialog was up")
		}
	}
	// Another process passes alerts while the dialog is up: it asks nothing.
	if res := r.run(t, "implementer", "", envs, "wait", "--for", "implementer", "--timeout", "1s"); res.code != 0 || strings.Contains(res.err, "asking the operator") {
		t.Fatalf("a second process: exit %d, stderr %q", res.code, res.err)
	}
	if got := lines(); len(got) != 1 {
		t.Fatalf("a second pop-up while the first was up: %q", got)
	}
	if err := os.WriteFile(popups+".release", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	until("marker", func() bool { return r.marked(al.Key) })
	if d, _ := r.decision(t, req); d != "" {
		t.Fatalf("decision %q; the test dialog must never click", d)
	}
	r.run(t, "implementer", "", envs, "wait", "--for", "implementer", "--timeout", "1s")
	if got := lines(); len(got) != 1 {
		t.Fatalf("asked again after the dialog went: %q", got)
	}
	if b, _ := os.ReadFile(notes); strings.Contains(string(b), "approved") {
		t.Fatalf("the decide alert was also notified: %q", b)
	}
}

// Both dialogs' AppleScript compiles. osacompile checks a script without
// running it, so nothing appears on screen. A real click is in
// evidence/h9b/live-popup.txt.
func TestH9bDialogScriptsCompile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("osacompile is macOS's")
	}
	if os.Getenv("CODEX_SANDBOX") != "" {
		t.Skip("in Codex's sandbox display dialog does not parse (evidence/h9b/codex-sandbox-probe.txt)")
	}
	for _, answer := range []bool{false, true} {
		args := append(dialogScript(3, answer), "-o", filepath.Join(t.TempDir(), "dialog.scpt"))
		if out, err := exec.Command("/usr/bin/osacompile", args...).CombinedOutput(); err != nil {
			t.Fatalf("answer %v: %v\n%s", answer, err, out)
		}
	}
}
