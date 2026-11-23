package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dre4success/handoff/internal/wait"
)

// H34 part 3 (row 111), through a session's hook: a round to two reviewers,
// one APPROVE, and the implementer's Stop hook doesn't wake it; the second
// APPROVE wakes it once, with both.
func TestH34p3TheHookWakesOnceForARound(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	var reqs []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		reqs = append(reqs, idOf(t, l))
	}
	first := idOf(t, r.ok(t, "reviewer", "reply", reqs[0], "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")))
	if quiet := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "2s"); strings.Contains(quiet.err, "#"+first+" reply") {
		t.Fatalf("woken for one APPROVE of two:\n%s", quiet.err)
	}
	second := idOf(t, r.ok(t, "reviewer-2", "reply", reqs[1], "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")))
	woke := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "5s")
	if woke.code != exitWake || !strings.Contains(woke.err, "#"+first+" reply") || !strings.Contains(woke.err, "#"+second+" reply") {
		t.Fatalf("the round's wake: exit %d\n%s", woke.code, woke.err)
	}
}

// reviewer-2's #549: a hook whose wait ends at a break in the chain wakes
// its session with what it gave before the break, a held APPROVE among it,
// rather than move the cursor past it unseen.
func TestH34p3AHookAtABreakGivesWhatWasHeld(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	first := idOf(t, strings.Split(strings.TrimSpace(out), "\n")[0])
	approve := idOf(t, r.ok(t, "reviewer", "reply", first, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")))
	if held := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "1s"); strings.Contains(held.err, "#"+approve+" reply") {
		t.Fatalf("not held:\n%s", held.err)
	}
	f, err := os.OpenFile(filepath.Join(r.dir, ".handoff", "channel.jsonl"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, `{"v":1,"id":99,"ts":"2026-11-23T03:00:00Z","kind":"note","item":"W1","from":"reviewer","to":"implementer","body":"forged","prev":"broken"}`+"\n")
	f.Close()
	woke := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "2s")
	if woke.code != exitWake || !strings.Contains(woke.err, "#"+approve+" reply") || !strings.Contains(woke.err, "The record is broken after these") || strings.Contains(woke.err, "#99") {
		t.Fatalf("at the break: exit %d\n%s", woke.code, woke.err)
	}
	if c, err := wait.ReadCursor(filepath.Join(r.dir, ".handoff"), "implementer"); err != nil || fmt.Sprint(c) != approve {
		t.Fatalf("cursor %d, want #%s", c, approve)
	}
}

// reviewer-2's #563 and the reviewer's #564: a wait that fails for another
// reason, as when the cursor can't be written, wakes no one, however often
// its hook runs; the entry is delivered once the cursor can be written.
func TestH34p3AHookWhoseWaitFailsWakesNoOne(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	note := idOf(t, r.ok(t, "reviewer", "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "n")))
	cursors := filepath.Join(r.dir, ".handoff", "cursors")
	if err := os.MkdirAll(cursors, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cursors, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cursors, 0o755) })
	for range 2 {
		// The operator is told, each time; the session isn't woken.
		if res := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "1s"); res.code != 0 || strings.Contains(res.err, "#"+note) || !strings.Contains(res.err, "told the operator") {
			t.Fatalf("a hook whose wait can't move its cursor: exit %d\n%q", res.code, res.err)
		}
	}
	os.Chmod(cursors, 0o755)
	if res := r.run(t, "", hookIn("s-imp", "Stop"), imp, "hook", "listen", "--timeout", "2s"); res.code != exitWake || !strings.Contains(res.err, "#"+note+" note") {
		t.Fatalf("once it can: exit %d\n%s", res.code, res.err)
	}
}

// Rows 110 and 111: a blocked requester gets no block alert for an APPROVE
// held back from it; the APPROVE that completes the round raises one.
func TestH34p3NoBlockAlertForAHeldApprove(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-imp")
	r.as(t, agy, "role", "implementer")
	sent, notify := sink(t)
	runAt(t, t.TempDir(), agyIn("ag-imp", "Stop", r.dir, agyContinuations), append(agy, notify...), "hook", "listen", "--timeout", "2s")
	if b := r.blockOf(t, "ag-imp"); b == nil || b.Reason != "cap" {
		t.Fatalf("the block: %+v", b)
	}
	out := r.run(t, "implementer", "", notify, "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b")).out
	lines := strings.Split(strings.TrimSpace(out), "\n")
	first, second := idOf(t, lines[0]), idOf(t, lines[1])
	alerts := func() []string {
		var got []string
		for _, l := range sentTo(t, sent) {
			if strings.Contains(l, "work waits for it") {
				got = append(got, l)
			}
		}
		return got
	}
	r.run(t, "reviewer", "", notify, "reply", first, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	if got := alerts(); len(got) != 0 {
		t.Fatalf("an alert for a held APPROVE: %q", got)
	}
	last := idOf(t, r.run(t, "reviewer-2", "", notify, "reply", second, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")).out)
	if got := alerts(); len(got) != 1 || !strings.Contains(got[0], "(#"+last+")") {
		t.Fatalf("the round's last APPROVE: %q", got)
	}
}
