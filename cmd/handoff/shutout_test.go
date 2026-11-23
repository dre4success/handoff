package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/wait"
)

// H34 part 6 (row 107): a session whose role the operator gave to another
// can't still act as it, or as any role, until the operator gives it one
// again. It keeps a mark through its process ending, a resume, /clear and
// /background, and a write or a delivery under way when the role moves can't
// land after it.

const lostWant = "you no longer hold reviewer: the operator gave it to session s-new at "

// pausePoint is a pause point in a handoff process (testPause, H36).
type pausePoint struct{ dir, name string }

// pauseAt names a pause point, name or name@n, and returns it and the
// environment that sets it.
func pauseAt(t *testing.T, point string) (pausePoint, string) {
	dir := t.TempDir()
	name, _, _ := strings.Cut(point, "@")
	return pausePoint{dir, name}, "HANDOFF_TEST_PAUSE=" + point + "=" + dir
}

// reached waits until a process is held at the point.
func (p pausePoint) reached(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(p.dir, p.name+".paused")); err == nil {
			return
		}
	}
	t.Fatalf("no process reached the pause point %s in 30 s", p.name)
}

// release lets the process held there go on.
func (p pausePoint) release(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.dir, p.name+".go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// listenedFor waits until something listens for role.
func (r *repo) listenedFor(t *testing.T, role string) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if wait.Listening(filepath.Join(r.dir, ".handoff"), role) {
			return
		}
	}
	t.Fatalf("nothing listens for %s after 30 s", role)
}

func TestH34p6ASessionThatLostItsRoleCantActAsAny(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	old, nu := claude("s-old", cli(t)), claude("s-new", cli(t))
	r.as(t, old, "role", "reviewer")
	if out := r.as(t, nu, "role", "reviewer"); !strings.Contains(out, "Another session had reviewer; it has no role now.") {
		t.Fatalf("role reviewer, taken:\n%s", out)
	}
	body := r.body(t, "b")
	for _, args := range [][]string{
		{"claim", req, "--as", "reviewer"},
		{"reply", req, "--as", "reviewer", "--verdict", "APPROVE", "--commit", r.commitA, "--body", body},
		{"note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", body},
		{"post", "--as", "implementer", "--item", "W2", "--to", "reviewer", "--commit", r.commitA, "--title", "W2", "--body", body},
		{"claim", req}, // no --as
		{"role"},
		{"brief"},
		{},
	} {
		r.refusedIn(t, old, lostWant, args...)
	}
	// Nothing was written by it.
	for _, e := range r.snap(t).Entries {
		if e.Kind != "request" {
			t.Fatalf("the old session wrote %+v", e)
		}
	}
	// The new holder acts.
	r.as(t, nu, "claim", req)
	// handoff role, the one command it may run, ends the mark.
	if out := r.as(t, old, "role", "reviewer-2"); !strings.Contains(out, "You are now reviewer-2") {
		t.Fatalf("role reviewer-2:\n%s", out)
	}
	r.as(t, old, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "back as reviewer-2"))
}

func TestH34p6TheMarkOutlivesTheProcessAndMovesWithTheSession(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	old := claude("s-old", pid)
	r.as(t, old, "role", "reviewer")
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	end()
	// Resumed under its id, in a new process: still refused.
	resumed := claude("s-old", cli(t))
	r.refusedIn(t, resumed, lostWant, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))

	// /clear: a new id in the same process carries the mark, and no role.
	pid2 := cli(t)
	r.as(t, claude("s-a", pid2), "role", "reviewer-2")
	r.as(t, claude("s-b", cli(t)), "role", "reviewer-2")
	cleared := claude("s-a2", pid2)
	if out := r.run(t, "", hookIn("s-a2", "SessionStart", "clear"), cleared, "hook", "brief").out; !strings.Contains(out, "you no longer hold reviewer-2: the operator gave it to session s-b at ") {
		t.Fatalf("the brief after /clear:\n%s", out)
	}
	r.refusedIn(t, cleared, "you no longer hold reviewer-2", "note", "--as", "reviewer-2", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))

	// A marked session is never a holder: once the new holder's process
	// ends, a post to reviewer is told no session has it.
	nuPid, nuEnd := cliEnds(t)
	r.as(t, claude("s-c", nuPid), "role", "reviewer")
	nuEnd()
	res := r.run(t, "implementer", "", nil, "post", "--item", "W3", "--to", "reviewer", "--commit", r.commitA, "--title", "W3", "--body", r.body(t, "b"))
	if res.code != 0 || !strings.Contains(res.err, "no session has reviewer now") {
		t.Fatalf("post to a role only marked sessions had: exit %d\n%s", res.code, res.err)
	}
}

func TestH34p6TheBackgroundedSessionCarriesItsMark(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-old", pid), "role", "reviewer")
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	f.write(t, "s-old", wentOnLine("s-old", "s-bg", time.Now()))
	f.write(t, "s-bg")
	end()
	bg := claude("s-bg", cli(t))
	if out := r.run(t, "", hookInT("s-bg", "SessionStart", f.path("s-bg"), "fork"), bg, "hook", "brief").out; !strings.Contains(out, lostWant) {
		t.Fatalf("the brief after /background:\n%s", out)
	}
	r.refusedIn(t, bg, lostWant, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
}

// A session whose role is fixed by --as or HANDOFF_ROLE, with no binding and
// no mark, passes as before.
func TestH34p6ARoleWithNoBindingPassesAsBefore(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	r.as(t, claude("s-x", cli(t)), "claim", req, "--as", "reviewer")
	r.ok(t, "reviewer", "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
}

// A write that resolved its role before the move, and writes after it, is
// refused.
func TestH34p6AWritePausedAcrossTheMoveIsRefused(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "reviewer")
	p, pause := pauseAt(t, "write")
	cmd := exec.Command(bin, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "late"))
	cmd.Dir, cmd.Env = r.dir, env(append(old, pause)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t)
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	p.release(t)
	if err := cmd.Wait(); err == nil || !strings.Contains(stderr.String(), lostWant) {
		t.Fatalf("the paused note: %v\n%s", err, stderr.String())
	}
	if n := len(r.snap(t).Entries); n != 0 {
		t.Fatalf("%d entries written", n)
	}
}

// The old listener, paused at a delivery while the role moves, delivers
// nothing and moves no cursor; the new holder gets the entry.
func TestH34p6TheOldListenerTakesNothingFromItsSuccessor(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "reviewer")
	p, pause := pauseAt(t, "deliver")
	listen := exec.Command(bin, "hook", "listen", "--timeout", "20s")
	listen.Dir, listen.Env = r.dir, env(append(old, pause)...)
	listen.Stdin = strings.NewReader(hookIn("s-old", "Stop"))
	var stderr strings.Builder
	listen.Stderr = &stderr
	if err := listen.Start(); err != nil {
		t.Fatal(err)
	}
	req := r.request(t, "W1", r.commitA)
	p.reached(t) // the listener has seen it, and is held at the fence
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	p.release(t)
	listen.Wait()
	if strings.Contains(stderr.String(), "#"+req+" request") {
		t.Fatalf("the old listener delivered #%s:\n%s", req, stderr.String())
	}
	// The old session's next Stop tells it, once.
	first := r.run(t, "", hookIn("s-old", "Stop"), old, "hook", "listen", "--timeout", "2s")
	if first.code != exitWake || !strings.Contains(first.err, lostWant) {
		t.Fatalf("the old session's next Stop: exit %d\n%s", first.code, first.err)
	}
	if again := r.run(t, "", hookIn("s-old", "Stop"), old, "hook", "listen", "--timeout", "2s"); again.code != 0 || again.err != "" {
		t.Fatalf("its Stop after: exit %d\n%s", again.code, again.err)
	}
	heard := r.run(t, "", hookIn("s-new", "Stop"), claude("s-new", cli(t)), "hook", "listen", "--timeout", "3s")
	if heard.code != exitWake || !strings.Contains(heard.err, "#"+req+" request") {
		t.Fatalf("the new holder's hook: exit %d\n%s", heard.code, heard.err)
	}
}

// Moving roles and writing at once never deadlock: the sessions' lock comes
// before the record's, and a write takes only the record's.
func TestH34p6MovesAndWritesNeverDeadlock(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	sessions := [][]string{claude("s-1", cli(t)), claude("s-2", cli(t)), claude("s-3", cli(t))}
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := range 6 {
			wg.Add(2)
			go func(i int) {
				defer wg.Done()
				r.run(t, "", "", sessions[i%3], "role", "reviewer")
			}(i)
			go func(i int) {
				defer wg.Done()
				r.run(t, "", "", sessions[(i+1)%3], "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "n"))
			}(i)
		}
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("role moves and writes still running after 60 s")
	}
}

// reviewer-3's #371, point 1: wait is not a read, so a marked session can't
// take its old role's lock and deliveries with --for.
func TestH34p6AMarkedSessionCantWaitForAnyRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "reviewer")
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	r.request(t, "W1", r.commitA)
	for _, args := range [][]string{{"wait", "--timeout", "1s"}, {"wait", "--for", "reviewer", "--timeout", "1s"}} {
		res := r.run(t, "", "", old, args...)
		if res.code != 2 || !strings.Contains(res.err, lostWant) || strings.Contains(res.err, "--for") {
			t.Fatalf("%v from the marked session: exit %d\n%s%s", args, res.code, res.out, res.err)
		}
	}
	if c, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer"); err != nil || c != 0 {
		t.Fatalf("reviewer's cursor: %d, %v", c, err)
	}
	// Reading stays open.
	r.as(t, old, "inbox", "--for", "reviewer")
}

// reviewer-3's #371, point 2: the fence refusing on a skipped fyi keeps
// what was delivered before it.
func TestH34p6TheFenceKeepsWhatWasDelivered(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "reviewer")
	req := r.request(t, "W1", r.commitA)
	if _, _, err := r.store().Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) {
		return &record.Entry{Kind: "fyi", From: "operator", To: "reviewer", Session: "s-prev", Body: "for another session"}, nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	p, pause := pauseAt(t, "deliver@2")
	listen := exec.Command(bin, "hook", "listen", "--timeout", "20s")
	listen.Dir, listen.Env = r.dir, env(append(old, pause)...)
	listen.Stdin = strings.NewReader(hookIn("s-old", "Stop"))
	var stderr strings.Builder
	listen.Stderr = &stderr
	if err := listen.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // #1 delivered; the fyi is held at the fence
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	p.release(t)
	err := listen.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != exitWake || !strings.Contains(stderr.String(), "#"+req+" request") {
		t.Fatalf("the old listener: %v\n%s", err, stderr.String())
	}
}

// reviewer-3's #371, point 4: at Antigravity's 10th continue, which it
// ignores, the telling waits for a turn that can reach the model.
func TestH34p6AntigravityIsToldOnATurnItCanHear(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-old")
	r.as(t, agy, "role", "reviewer")
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	at10 := runAt(t, r.dir, agyIn("ag-old", "Stop", r.dir, agyContinuations), nil, "hook", "listen", "--timeout", "2s")
	if strings.TrimSpace(at10.out) != "{}" {
		t.Fatalf("at the 10th: %q\n%s", at10.out, at10.err)
	}
	at0 := runAt(t, r.dir, agyIn("ag-old", "Stop", r.dir, 0), nil, "hook", "listen", "--timeout", "2s")
	if !strings.Contains(at0.out, `"decision":"continue"`) || !strings.Contains(at0.out, "you no longer hold reviewer") {
		t.Fatalf("on a turn after the operator typed: %q\n%s", at0.out, at0.err)
	}
}

// reviewer-3's #371, point 6: repair is a write, and fenced.
func TestH34p6RepairIsFenced(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "reviewer")
	p, pause := pauseAt(t, "write")
	cmd := exec.Command(bin, "repair")
	cmd.Dir, cmd.Env = r.dir, env(append(old, pause)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t)
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	p.release(t)
	if err := cmd.Wait(); err == nil || !strings.Contains(stderr.String(), lostWant) {
		t.Fatalf("the paused repair: %v\n%s", err, stderr.String())
	}
}

// reviewer-3's #371, point 7: a hook whose role is fixed by --as checks the
// session on its stdin for a mark.
func TestH34p6AHookWithAFixedRoleSeesItsSessionsMark(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.as(t, codex("cx-old"), "role", "reviewer")
	r.as(t, codex("cx-new"), "role", "reviewer")
	req := r.request(t, "W1", r.commitA)
	res := r.run(t, "", hookIn("cx-old", "Stop"), nil, "hook", "listen", "--as", "reviewer", "--timeout", "2s")
	if strings.Contains(res.err, "#"+req+" request") {
		t.Fatalf("the old thread's hook delivered #%s:\n%s", req, res.err)
	}
	if c, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer"); err != nil || c != 0 {
		t.Fatalf("reviewer's cursor: %d, %v", c, err)
	}
}

// reviewer-3's #371, point 8: an older mark in the same process doesn't
// make /clear drop the mark of the conversation now in it.
func TestH34p6ClearMovesTheMarkOfTheConversationInTheProcess(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-1", pid), "role", "reviewer")
	r.as(t, claude("s-x", cli(t)), "role", "reviewer")
	r.as(t, claude("s-2", pid), "role", "reviewer-2")
	r.as(t, claude("s-y", cli(t)), "role", "reviewer-2")
	cleared := claude("s-3", pid)
	if out := r.run(t, "", hookIn("s-3", "SessionStart", "clear"), cleared, "hook", "brief").out; !strings.Contains(out, "you no longer hold reviewer-2: the operator gave it to session s-y at ") {
		t.Fatalf("the brief after /clear:\n%s", out)
	}
	r.refusedIn(t, cleared, "you no longer hold reviewer-2", "note", "--as", "reviewer-2", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
	// The older mark keeps its mark, for a resume by its id.
	r.refusedIn(t, claude("s-1", cli(t)), "you no longer hold reviewer", "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
}

// The reviewer's case C in #372: a session whose process ended, then any
// other role given in between, then its role given on. Its binding is kept,
// marked gone, so it is marked lost and, resumed, refused.
func TestH34p6AnEndedSessionIsMarkedWhenItsRoleIsGivenOn(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	end()
	r.as(t, claude("s-b", cli(t)), "role", "implementer") // another role, in between
	if out := r.as(t, claude("s-c", cli(t)), "role", "reviewer"); strings.Contains(out, "Another session had reviewer") {
		t.Fatalf("an ended session counted as having the role:\n%s", out)
	}
	r.refusedIn(t, claude("s-a", cli(t)), "you no longer hold reviewer: the operator gave it to session s-c at ", "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
}

// reviewer-3's R2-A in #378: a session that went on by /background, its old
// binding marked gone by a bind in between, is live, not gone.
func TestH34p6ABackgroundedSessionIsNotGone(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	f.write(t, "s-a", wentOnLine("s-a", "s-bg", time.Now()))
	f.write(t, "s-bg")
	end()
	r.as(t, claude("s-imp", cli(t)), "role", "implementer") // marks s-a gone
	bg := claude("s-bg", cli(t))
	if out := r.run(t, "", hookInT("s-bg", "SessionStart", f.path("s-bg"), "fork"), bg, "hook", "brief").out; !strings.Contains(out, "you are reviewer") {
		t.Fatalf("the brief after /background:\n%s", out)
	}
	res := r.run(t, "implementer", "", nil, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	if res.code != 0 || strings.Contains(res.err, "no session has reviewer") {
		t.Fatalf("a post to the backgrounded session's role: exit %d\n%s", res.code, res.err)
	}
}

// reviewer-3's R2-B in #378: a gone session's role given on, then its
// /background: the mark moves to the session it went on as.
func TestH34p6AGoneSessionsMarkMovesWithBackground(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	f.write(t, "s-a", wentOnLine("s-a", "s-bg", time.Now()))
	f.write(t, "s-bg")
	end()
	r.as(t, claude("s-imp", cli(t)), "role", "implementer") // marks s-a gone
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")    // marks s-a lost
	bg := claude("s-bg", cli(t))
	if out := r.run(t, "", hookInT("s-bg", "SessionStart", f.path("s-bg"), "fork"), bg, "hook", "brief").out; !strings.Contains(out, lostWant) {
		t.Fatalf("the brief after /background:\n%s", out)
	}
	r.refusedIn(t, bg, lostWant, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
}

// reviewer-3's third point in #378: a session's wait for a role it doesn't
// hold stops delivering once the session is marked.
func TestH34p6AWaitForAnotherRoleStopsWhenItsSessionIsMarked(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := claude("s-imp", cli(t))
	r.as(t, s, "role", "implementer")
	p, pause := pauseAt(t, "deliver")
	w := exec.Command(bin, "wait", "--for", "reviewer-2", "--timeout", "20s")
	w.Dir, w.Env = r.dir, env(append(s, pause)...)
	var out strings.Builder
	w.Stdout, w.Stderr = &out, &out
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	note := idOf(t, r.ok(t, "reviewer", "note", "--to", "reviewer-2", "--item", "W1", "--body", r.body(t, "for reviewer-2")))
	p.reached(t) // the wait has seen it, and is held at the fence
	r.as(t, claude("s-imp2", cli(t)), "role", "implementer")
	p.release(t)
	w.Wait()
	if strings.Contains(out.String(), "#"+note+" note") {
		t.Fatalf("the marked session's wait delivered #%s:\n%s", note, out.String())
	}
	if c, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer-2"); err != nil || c != 0 {
		t.Fatalf("reviewer-2's cursor: %d, %v", c, err)
	}
}

// The reviewer's point 4 in #379: a session resumed by its id while it still
// holds its role is live again from its first command, before any hook.
func TestH34p6AResumedSessionIsLiveFromItsFirstCommand(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	end()
	r.as(t, claude("s-imp", cli(t)), "role", "implementer") // marks s-a gone
	resumed := claude("s-a", cli(t))
	r.as(t, resumed, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "back"))
	res := r.run(t, "implementer", "", nil, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	if res.code != 0 || strings.Contains(res.err, "no session has reviewer") {
		t.Fatalf("a post to the resumed session's role: exit %d\n%s", res.code, res.err)
	}
}

// reviewer-2's #388, point 1: a mark carried by /background or /clear stays
// on the id it was carried from, which can be resumed, detached from its
// process; and a role carried leaves a mark on its old id. The operator's
// handoff role still ends a mark.
func TestH34p6TheOldIdKeepsItsMarkWhenAMoveCarriesIt(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	note := func(role string) []string {
		return []string{"note", "--as", role, "--to", "implementer", "--item", "W1", "--body", r.body(t, "b")}
	}
	// /background of a marked session: both ids are refused.
	pid, end := cliEnds(t)
	r.as(t, claude("s-old", pid), "role", "reviewer")
	r.as(t, claude("s-new", cli(t)), "role", "reviewer")
	f.write(t, "s-old", wentOnLine("s-old", "s-bg", time.Now()))
	f.write(t, "s-bg")
	end()
	bg := claude("s-bg", cli(t))
	r.run(t, "", hookInT("s-bg", "SessionStart", f.path("s-bg"), "fork"), bg, "hook", "brief")
	r.refusedIn(t, bg, lostWant, note("reviewer")...)
	r.refusedIn(t, claude("s-old", cli(t)), lostWant, note("reviewer")...) // resumed by its old id

	// /clear of a marked session: both ids are refused, and the old id's
	// mark no longer matches the process, so a second /clear there moves
	// the mark of the conversation now in it.
	pid2 := cli(t)
	r.as(t, claude("s-a", pid2), "role", "reviewer-2")
	r.as(t, claude("s-b", cli(t)), "role", "reviewer-2")
	lost2 := "you no longer hold reviewer-2: the operator gave it to session s-b at "
	r.run(t, "", hookIn("s-a2", "SessionStart", "clear"), claude("s-a2", pid2), "hook", "brief")
	r.refusedIn(t, claude("s-a2", pid2), lost2, note("reviewer-2")...)
	r.refusedIn(t, claude("s-a", cli(t)), lost2, note("reviewer-2")...)
	if out := r.run(t, "", hookIn("s-a3", "SessionStart", "clear"), claude("s-a3", pid2), "hook", "brief").out; !strings.Contains(out, lost2) {
		t.Fatalf("the brief after a second /clear:\n%s", out)
	}
	r.refusedIn(t, claude("s-a3", pid2), lost2, note("reviewer-2")...)

	// A role moved by /background twice, a hook between: the id left behind
	// the first time doesn't make the second ambiguous, and both old ids are
	// marked.
	pid3, end3 := cliEnds(t)
	r.as(t, claude("s-1", pid3), "role", "implementer")
	f.wentOn(t, map[string]string{"s-1": "s-2"})
	f.write(t, "s-2")
	end3()
	pid4, end4 := cliEnds(t)
	if out := r.run(t, "", hookInT("s-2", "SessionStart", f.path("s-2"), "fork"), claude("s-2", pid4), "hook", "brief").out; !strings.Contains(out, "you are implementer") {
		t.Fatalf("after the first /background:\n%s", out)
	}
	f.wentOn(t, map[string]string{"s-1": "s-2", "s-2": "s-3"})
	f.write(t, "s-3")
	end4()
	if out := r.run(t, "", hookInT("s-3", "SessionStart", f.path("s-3"), "fork"), claude("s-3", cli(t)), "hook", "brief").out; !strings.Contains(out, "you are implementer") {
		t.Fatalf("after the second /background:\n%s", out)
	}
	if b, ok := boundTo(t, r, "s-3"); !ok || !slices.Equal(b.Was, []string{"s-1", "s-2"}) {
		t.Fatalf("s-3's binding: %+v", b)
	}
	for id, to := range map[string]string{"s-1": "s-2", "s-2": "s-3"} {
		r.refusedIn(t, claude(id, cli(t)), "you no longer hold implementer: this conversation went on in session "+to+" at ", "inbox")
	}

	// The operator's way back is unchanged: handoff role in the old id ends
	// its mark, and the id it was carried to stays marked.
	r.as(t, claude("s-old", cli(t)), "role", "reviewer-2")
	r.as(t, claude("s-old", cli(t)), "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "back"))
	r.refusedIn(t, bg, lostWant, note("reviewer")...)
}

// reviewer-2's #388, point 2: a move by /clear or /background takes the
// record's lock for writing, after the sessions', as bind does, so it waits
// for a write or a delivery that holds the record's lock.
func TestH34p6ClearAndBackgroundMoveUnderTheRecordsLock(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"clear", "background"} {
		t.Run(how, func(t *testing.T) {
			r := newRepo(t)
			f := newFolder(t)
			pid, end := cliEnds(t)
			r.as(t, claude("s-a", pid), "role", "reviewer")
			in, moved := hookIn("s-b", "SessionStart", "clear"), claude("s-b", pid)
			if how == "background" {
				f.wentOn(t, map[string]string{"s-a": "s-b"})
				f.write(t, "s-b")
				end()
				in, moved = hookInT("s-b", "SessionStart", f.path("s-b"), "fork"), claude("s-b", cli(t))
			}
			unlock := holdRecordLock(t, r) // as a delivery under its fence holds it
			done := make(chan result, 1)
			go func() { done <- r.run(t, "", in, moved, "hook", "brief") }()
			select {
			case res := <-done:
				unlock()
				t.Fatalf("the move went on while the record's lock was held:\n%s%s", res.out, res.err)
			case <-time.After(1500 * time.Millisecond):
			}
			if b, ok := boundTo(t, r, "s-b"); ok {
				unlock()
				t.Fatalf("s-b bound while the record's lock was held: %+v", b)
			}
			unlock()
			if res := <-done; !strings.Contains(res.out, "you are reviewer") {
				t.Fatalf("the brief once the lock was free:\n%s%s", res.out, res.err)
			}
		})
	}
}

// holdRecordLock takes the record's lock, shared, as a delivery under its
// fence holds it, and returns its release.
func holdRecordLock(t *testing.T, r *repo) (unlock func()) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(r.dir, ".handoff", record.LockName), os.O_RDWR|os.O_CREATE, 0o644)
	if err == nil {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
	}
	if err != nil {
		t.Fatal(err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }
}

// reviewer-2's #388, point 2, and #408, point 1: a move killed between its
// two writes leaves the old id marked, never free. Nothing finishes it: the
// new session is marked too, until the operator gives it a role, and a role
// the operator gives meanwhile stays theirs, even once it is free again.
func TestH34p6AMoveCutShortLeavesTheOldIdMarked(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	moved := claude("s-b", pid)
	cut := r.run(t, "", hookIn("s-b", "SessionStart", "clear"), append(moved, "HANDOFF_TEST_FAULT=move-between"), "hook", "brief")
	if cut.code != -1 {
		t.Fatalf("the cut move: exit %d\n%s%s", cut.code, cut.out, cut.err)
	}
	if _, ok := boundTo(t, r, "s-b"); ok {
		t.Fatal("s-b was written before the old id was marked")
	}
	r.refusedIn(t, claude("s-a", cli(t)), "you no longer hold reviewer: this conversation went on in session s-b at ", "inbox")
	const cutWant = "you no longer hold reviewer: its move to this session, by /clear at "
	note := []string{"note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b")}
	// Before any hook of it writes its mark, a command from it is refused
	// as its hook would mark it (the reviewer, #427).
	r.refusedIn(t, moved, cutWant, note...)
	if _, ok := boundTo(t, r, "s-b"); ok {
		t.Fatal("a command wrote s-b's binding")
	}
	if out := r.run(t, "", hookIn("s-b", "Stop"), moved, "hook", "brief").out; !strings.Contains(out, cutWant) {
		t.Fatalf("s-b's next hook:\n%s", out)
	}
	r.refusedIn(t, moved, cutWant, note...)
	// The operator gives the role to another, which later takes another
	// role, so the role is free: s-b still doesn't get it.
	x := claude("s-x", cli(t))
	r.as(t, x, "role", "reviewer")
	r.as(t, x, "role", "reviewer-2")
	if out := r.run(t, "", hookIn("s-b", "Stop"), moved, "hook", "brief").out; !strings.Contains(out, cutWant) {
		t.Fatalf("s-b's hook once the role was free:\n%s", out)
	}
	r.refusedIn(t, moved, cutWant, note...)
	// The operator's way back.
	r.as(t, moved, "role", "reviewer")
	r.as(t, moved, note...)
}

// reviewer-2's #408, point 2: a hook whose role is fixed by --as resolves
// what its session's conversation carried to a new id first. A marked
// conversation moved by /clear or /background is refused at the new id; a
// role moves with it, as on the path with no fixed role.
func TestH34p6AFixedRoleHookTakesWhatTheConversationCarried(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		how    string
		marked bool
	}{{"clear", true}, {"background", true}, {"clear", false}, {"background", false}} {
		t.Run(fmt.Sprintf("%s, marked %v", c.how, c.marked), func(t *testing.T) {
			r := newRepo(t)
			f := newFolder(t)
			pid, end := cliEnds(t)
			r.as(t, claude("s-a", pid), "role", "reviewer")
			if c.marked {
				r.as(t, claude("s-x", cli(t)), "role", "reviewer")
			}
			in, moved := hookIn("s-b", "SessionStart", "clear"), claude("s-b", pid)
			if c.how == "background" {
				f.wentOn(t, map[string]string{"s-a": "s-b"})
				f.write(t, "s-b")
				end()
				in, moved = hookInT("s-b", "SessionStart", f.path("s-b"), "fork"), claude("s-b", cli(t))
			}
			brief := r.run(t, "", in, moved, "hook", "brief", "--as", "reviewer").out
			if !c.marked {
				if !strings.Contains(brief, "you are reviewer") {
					t.Fatalf("the brief at the new id:\n%s", brief)
				}
				if b, ok := boundTo(t, r, "s-b"); !ok || b.Role != "reviewer" {
					t.Fatalf("s-b's binding: %+v", b)
				}
				r.refusedIn(t, claude("s-a", cli(t)), "you no longer hold reviewer: this conversation went on in session s-b at ", "inbox")
				return
			}
			lost := "you no longer hold reviewer: the operator gave it to session s-x at "
			if !strings.Contains(brief, lost) {
				t.Fatalf("the brief at the new id:\n%s", brief)
			}
			req := r.request(t, "W1", r.commitA)
			heard := r.run(t, "", hookIn("s-b", "Stop"), moved, "hook", "listen", "--as", "reviewer", "--timeout", "2s")
			if strings.Contains(heard.err, "#"+req+" request") {
				t.Fatalf("the new id's hook delivered #%s:\n%s", req, heard.err)
			}
			if cur, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer"); err != nil || cur != 0 {
				t.Fatalf("reviewer's cursor: %d, %v", cur, err)
			}
			r.refusedIn(t, moved, lost, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
		})
	}
}

// reviewer-2's #388, point 3: a hook whose role is fixed by --as, started
// while its session had no binding, is fenced: once the session takes a
// role and loses it, the hook delivers nothing, and stops, so the new
// holder can listen.
func TestH34p6AFixedRoleHookStartedUnboundIsFenced(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := codex("cx-a")
	p, pause := pauseAt(t, "deliver")
	listen := exec.Command(bin, "hook", "listen", "--as", "reviewer", "--timeout", "20s")
	listen.Dir, listen.Env = r.dir, env(append(s, pause)...)
	listen.Stdin = strings.NewReader(hookIn("cx-a", "Stop"))
	var stderr strings.Builder
	listen.Stderr = &stderr
	if err := listen.Start(); err != nil {
		t.Fatal(err)
	}
	r.listenedFor(t, "reviewer") // listening, with no binding
	r.as(t, s, "role", "reviewer")
	req := r.request(t, "W1", r.commitA)
	p.reached(t) // the hook has seen it, and is held at the fence
	r.as(t, codex("cx-b"), "role", "reviewer")
	p.release(t)
	began := time.Now()
	listen.Wait()
	if strings.Contains(stderr.String(), "#"+req+" request") {
		t.Fatalf("the old hook delivered #%s:\n%s", req, stderr.String())
	}
	if d := time.Since(began); d > 10*time.Second {
		t.Fatalf("the old hook listened on for %v", d)
	}
	if c, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer"); err != nil || c != 0 {
		t.Fatalf("reviewer's cursor: %d, %v", c, err)
	}
	heard := r.run(t, "", hookIn("cx-b", "Stop"), codex("cx-b"), "hook", "listen", "--timeout", "3s")
	if heard.code != exitWake || !strings.Contains(heard.err, "#"+req+" request") {
		t.Fatalf("the new holder's hook: exit %d\n%s", heard.code, heard.err)
	}
}

// reviewer-2's #388, point 3: a bound hook keeps the generation it resolved
// its role at. The role moved away and back before it listens: it delivers
// nothing, and the next hook of the session, at the new generation, does.
func TestH34p6AHookKeepsTheGenerationItResolved(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := codex("cx-a")
	r.as(t, s, "role", "reviewer")
	req := r.request(t, "W1", r.commitA)
	listen := exec.Command(bin, "hook", "listen", "--as", "reviewer", "--timeout", "20s")
	p, pause := pauseAt(t, "write")
	listen.Dir, listen.Env = r.dir, env(append(s, pause)...)
	listen.Stdin = strings.NewReader(hookIn("cx-a", "Stop"))
	var stderr strings.Builder
	listen.Stderr = &stderr
	if err := listen.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // resolved, and held
	r.as(t, codex("cx-b"), "role", "reviewer")
	r.as(t, s, "role", "reviewer") // back, at a new generation
	p.release(t)
	listen.Wait()
	if strings.Contains(stderr.String(), "#"+req+" request") {
		t.Fatalf("the hook delivered #%s at a generation it didn't resolve:\n%s", req, stderr.String())
	}
	heard := r.run(t, "", hookIn("cx-a", "Stop"), s, "hook", "listen", "--as", "reviewer", "--timeout", "3s")
	if heard.code != exitWake || !strings.Contains(heard.err, "#"+req+" request") {
		t.Fatalf("the session's next hook: exit %d\n%s", heard.code, heard.err)
	}
}

// The reviewer's Q1 and Q2 in #410: after /background, the operator gives
// the new id a role again, and it goes on by /background once more. The
// role goes with it: the latest session is the one the transcripts lead
// through, whatever its binding notes.
func TestH34p6ARoleGivenAgainAfterBackgroundGoesOnWithIt(t *testing.T) {
	t.Parallel()
	for _, marked := range []bool{false, true} {
		t.Run(fmt.Sprintf("marked %v", marked), func(t *testing.T) {
			r := newRepo(t)
			f := newFolder(t)
			pid, end := cliEnds(t)
			r.as(t, claude("s-x", pid), "role", "reviewer")
			if marked {
				r.as(t, claude("s-z", cli(t)), "role", "reviewer")
			}
			f.wentOn(t, map[string]string{"s-x": "s-y"})
			f.write(t, "s-y")
			end()
			pidY, endY := cliEnds(t)
			y := claude("s-y", pidY)
			r.run(t, "", hookInT("s-y", "SessionStart", f.path("s-y"), "fork"), y, "hook", "brief")
			role := "reviewer"
			if marked {
				role = "implementer" // the operator gives the marked one another role
			}
			time.Sleep(10 * time.Millisecond) // the role is older than the line below
			r.as(t, y, "role", role)
			f.wentOn(t, map[string]string{"s-x": "s-y", "s-y": "s-y2"})
			f.write(t, "s-y2")
			endY()
			if out := r.run(t, "", hookInT("s-y2", "SessionStart", f.path("s-y2"), "fork"), claude("s-y2", cli(t)), "hook", "brief").out; !strings.Contains(out, "you are "+role) {
				t.Fatalf("s-y2's brief, want %s:\n%s", role, out)
			}
			r.refusedIn(t, claude("s-y", cli(t)), "you no longer hold "+role+": this conversation went on in session s-y2 at ", "inbox")
			r.refusedIn(t, claude("s-x", cli(t)), "you no longer hold reviewer", "inbox")
		})
	}
}

// The reviewer's N in #410: a move that finished, its new id's binding
// marked later because another conversation in its process was given a
// role, and the new id resumed. It is refused as a conversation that went
// on, not taken for a move cut short, and never gets the role back.
func TestH34p6AFinishedMoveIsNeverTakenForACutOne(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-x", pid), "role", "reviewer")
	f.wentOn(t, map[string]string{"s-x": "s-y"})
	f.write(t, "s-y")
	end()
	pidY := cli(t)
	if out := r.run(t, "", hookInT("s-y", "SessionStart", f.path("s-y"), "fork"), claude("s-y", pidY), "hook", "brief").out; !strings.Contains(out, "you are reviewer") {
		t.Fatalf("s-y's brief:\n%s", out)
	}
	// /clear in s-y's process with no hook to move the role, and the
	// operator gives the conversation now there another role.
	r.as(t, claude("s-z", pidY), "role", "reviewer-2")
	resumed := claude("s-y", cli(t))
	want := "you no longer hold reviewer: this conversation went on in session s-z at "
	r.refusedIn(t, resumed, want, "inbox")
	if out := r.run(t, "", hookInT("s-y", "Stop", f.path("s-y")), resumed, "hook", "brief").out; !strings.Contains(out, want) || strings.Contains(out, "cut short") {
		t.Fatalf("s-y's hook after its resume:\n%s", out)
	}
	r.refusedIn(t, resumed, want, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
	res := r.run(t, "implementer", "", nil, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	if res.code != 0 || !strings.Contains(res.err, "no session has reviewer now") {
		t.Fatalf("a post to reviewer: exit %d\n%s", res.code, res.err)
	}
}

// reviewer-2's #422: a hook whose role is fixed by --as doesn't take that
// role while a move to its session is known and not settled. The marked
// session it went on from still runs past the hook's wait; then its mark is
// carried, once it has ended. And a move cut short whose mark on the new id
// can't be written yet blocks the fixed role too.
func TestH34p6AFixedRoleWaitsForACarriedMoveToSettle(t *testing.T) {
	t.Parallel()
	t.Run("the marked session still runs", func(t *testing.T) {
		r := newRepo(t)
		f := newFolder(t)
		pid, end := cliEnds(t)
		r.as(t, claude("s-a", pid), "role", "reviewer")
		r.as(t, claude("s-x", cli(t)), "role", "reviewer") // s-a marked
		f.wentOn(t, map[string]string{"s-a": "s-b"})
		f.write(t, "s-b")
		moved := append(claude("s-b", cli(t)), "HANDOFF_TEST_CONTINUED_WAIT=300ms")
		req := r.request(t, "W1", r.commitA)
		if out := r.run(t, "", hookInT("s-b", "SessionStart", f.path("s-b"), "fork"), moved, "hook", "brief", "--as", "reviewer").out; strings.Contains(out, "you are reviewer") {
			t.Fatalf("the brief while the move is pending:\n%s", out)
		}
		heard := r.run(t, "", hookInT("s-b", "Stop", f.path("s-b")), moved, "hook", "listen", "--as", "reviewer", "--timeout", "2s")
		if strings.Contains(heard.err, "#"+req+" request") {
			t.Fatalf("the hook took the fixed role while the move was pending:\n%s", heard.err)
		}
		if cur, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "reviewer"); err != nil || cur != 0 {
			t.Fatalf("reviewer's cursor: %d, %v", cur, err)
		}
		end()
		lost := "you no longer hold reviewer: the operator gave it to session s-x at "
		if out := r.run(t, "", hookInT("s-b", "Stop", f.path("s-b")), moved, "hook", "brief", "--as", "reviewer").out; !strings.Contains(out, lost) {
			t.Fatalf("the brief once s-a has ended:\n%s", out)
		}
		r.refusedIn(t, moved, lost, "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
	})
	t.Run("a cut move's mark can't be written yet", func(t *testing.T) {
		r := newRepo(t)
		pid := cli(t)
		r.as(t, claude("s-a", pid), "role", "reviewer")
		moved := claude("s-b", pid)
		if cut := r.run(t, "", hookIn("s-b", "SessionStart", "clear"), append(moved, "HANDOFF_TEST_FAULT=move-between"), "hook", "brief"); cut.code != -1 {
			t.Fatalf("the cut move: exit %d\n%s%s", cut.code, cut.out, cut.err)
		}
		req := r.request(t, "W1", r.commitA)
		sessions := filepath.Join(r.dir, ".handoff", "sessions")
		if err := os.Chmod(sessions, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(sessions, 0o755) })
		heard := r.run(t, "", hookIn("s-b", "Stop"), moved, "hook", "listen", "--as", "reviewer", "--timeout", "2s")
		if strings.Contains(heard.err, "#"+req+" request") {
			t.Fatalf("the hook took the fixed role with the move cut short:\n%s", heard.err)
		}
		if _, ok := boundTo(t, r, "s-b"); ok {
			t.Fatal("s-b was written into a folder it can't write")
		}
		r.refusedIn(t, moved, "was cut short", "note", "--as", "reviewer", "--to", "implementer", "--item", "W1", "--body", r.body(t, "b"))
		os.Chmod(sessions, 0o755)
		if out := r.run(t, "", hookIn("s-b", "Stop"), moved, "hook", "brief", "--as", "reviewer").out; !strings.Contains(out, "was cut short") {
			t.Fatalf("the brief once the mark can be written:\n%s", out)
		}
	})
}

// reviewer-3's #441 and the reviewer's #442: Claude Code runs a /clear's
// SessionStart hooks, brief and listen, at once. The one that finds the role
// being moved, or moved already, by the other takes it too, and none says
// the role is lost or cut short. First with the mover paused between its two
// writes, beside a hook with no fixed role and one with --as; then brief and
// listen together, /clear after /clear, with no pause.
func TestH34p6BothClearHooksHaveTheRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-0", pid), "role", "reviewer")
	paused := func(id string, other ...string) {
		t.Helper()
		in := hookIn(id, "SessionStart", "clear")
		done := make(chan result, 2)
		p, pause := pauseAt(t, "move")
		go func() { done <- r.run(t, "", in, append(claude(id, pid), pause), "hook", "brief") }()
		p.reached(t) // the first is held between its writes
		go func() { done <- r.run(t, "", in, claude(id, pid), append([]string{"hook", "brief"}, other...)...) }()
		// The second may look before the first goes on, or after: either
		// way it must have the role, so this pause is no race to win.
		time.Sleep(300 * time.Millisecond)
		p.release(t)
		for range 2 {
			if res := <-done; !strings.Contains(res.out, "you are reviewer") {
				t.Fatalf("a /clear hook of %s:\n%s%s", id, res.out, res.err)
			}
		}
	}
	paused("s-1")
	paused("s-2", "--as", "reviewer")
	for i := 3; i <= 12; i++ {
		id := fmt.Sprintf("s-%d", i)
		in := hookIn(id, "SessionStart", "clear")
		brief, listen := make(chan result, 1), make(chan result, 1)
		go func() { brief <- r.run(t, "", in, claude(id, pid), "hook", "brief") }()
		go func() { listen <- r.run(t, "", in, claude(id, pid), "hook", "listen", "--timeout", "1s") }()
		if res := <-brief; !strings.Contains(res.out, "you are reviewer") {
			t.Fatalf("%s's brief:\n%s%s", id, res.out, res.err)
		}
		if res := <-listen; res.code != exitWake || !strings.Contains(res.err, "nothing new for reviewer") {
			t.Fatalf("%s's listener: exit %d\n%s", id, res.code, res.err)
		}
	}
	if b, ok := boundTo(t, r, "s-12"); !ok || b.Role != "reviewer" {
		t.Fatalf("s-12's binding: %+v", b)
	}
}

// reviewer-3's #450: a /background's SessionStart hooks run at once too. A
// hook that reads the session files while the other moves the role, as the
// old id's file is rewritten from a binding to a mark, sees it once, and
// takes the role. First with the mover paused between its two writes; then
// /background after /background, three hooks at once, with marked files
// about, as a project gathers them.
func TestH34p6BothBackgroundHooksHaveTheRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	for i := range 20 {
		r.as(t, claude(fmt.Sprintf("h-%d", i), cli(t)), "role", "reviewer-2")
	}
	pid, end := cliEnds(t)
	r.as(t, claude("s-0", pid), "role", "reviewer")
	hooks := func(id string, pid int, held bool) {
		t.Helper()
		brief := hookInT(id, "SessionStart", f.path(id), "fork")
		done := make(chan result, 3)
		p, pause := pauseAt(t, "move")
		first := claude(id, pid)
		if held {
			first = append(first, pause)
		}
		go func() { done <- r.run(t, "", brief, first, "hook", "brief") }()
		if held {
			p.reached(t) // the first is held between its writes
		}
		go func() { done <- r.run(t, "", brief, claude(id, pid), "hook", "brief") }()
		go func() {
			done <- r.run(t, "", brief, claude(id, pid), "hook", "listen", "--timeout", "1s")
		}()
		if held {
			// The others may look before the first goes on, or after:
			// either way they must have the role, so this is no race to win.
			time.Sleep(300 * time.Millisecond)
			p.release(t)
		}
		for range 3 {
			res := <-done
			if !strings.Contains(res.out, "you are reviewer") && !(res.code == exitWake && strings.Contains(res.err, "nothing new for reviewer")) {
				t.Fatalf("a /background hook of %s: exit %d\n%s%s", id, res.code, res.out, res.err)
			}
		}
	}
	for i := 0; i < 8; i++ {
		from, to := fmt.Sprintf("s-%d", i), fmt.Sprintf("s-%d", i+1)
		f.wentOn(t, map[string]string{from: to})
		f.write(t, to)
		end()
		var next int
		next, end = cliEnds(t)
		hooks(to, next, i == 0)
	}
	end()
	if b, ok := boundTo(t, r, "s-8"); !ok || b.Role != "reviewer" {
		t.Fatalf("s-8's binding: %+v", b)
	}
}

// The reviewer's note in #451: an fyi reads who has each role under the
// record's lock, which a move holds for writing. One sent while a /clear's
// move is between its writes waits for it, and goes to the new session.
func TestH34p6AnFyiDuringAMoveGoesToTheNewSession(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-a", pid), "role", "reviewer")
	done := make(chan result, 1)
	p, pause := pauseAt(t, "move")
	go func() {
		done <- r.run(t, "", hookIn("s-b", "SessionStart", "clear"), append(claude("s-b", pid), pause), "hook", "brief")
	}()
	p.reached(t) // the move is held between its writes
	sent := make(chan error, 1)
	go func() {
		out, err := r.atTerminal(t, "", nil, "fyi", "Carry on.")
		if err != nil {
			err = fmt.Errorf("%w\n%s", err, out)
		}
		sent <- err
	}()
	// The fyi waits on the record's lock, which the move holds; if it hasn't
	// got there yet, it reads after the move, which is as good.
	time.Sleep(300 * time.Millisecond)
	p.release(t)
	if err := <-sent; err != nil {
		t.Fatalf("fyi: %v", err)
	}
	if res := <-done; !strings.Contains(res.out, "you are reviewer") {
		t.Fatalf("the /clear hook:\n%s%s", res.out, res.err)
	}
	var to []string
	for _, e := range r.snap(t).Entries {
		if e.Kind == "fyi" && e.To == "reviewer" {
			to = append(to, e.Session)
		}
	}
	if len(to) != 1 || to[0] != "s-b" {
		t.Fatalf("the fyi to reviewer named %q, want s-b", to)
	}
}
