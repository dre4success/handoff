package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// H34 part 4 (row 112): smaller fixes.

// From its 7th wake, an Antigravity session is told how many handoff has
// left before the operator types in it.
func TestH34p4AntigravityWakesCountDown(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-1")
	r.as(t, agy, "role", "reviewer")
	reason := func(n int) string {
		t.Helper()
		res := runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, n), agy, "hook", "listen", "--timeout", "1s")
		var o struct{ Reason string }
		json.Unmarshal([]byte(res.out), &o)
		return o.Reason
	}
	if got := reason(5); strings.Contains(got, "more time") || strings.Contains(got, "last time") {
		t.Fatalf("at 5, told a count: %q", got)
	}
	if got := reason(6); !strings.Contains(got, "handoff can wake this session 3 more times before the operator types in it") {
		t.Fatalf("at 6: %q", got)
	}
	if got := reason(8); !strings.Contains(got, "1 more time before") {
		t.Fatalf("at 8: %q", got)
	}
	if got := reason(9); !strings.Contains(got, "This is the last time handoff can wake this session") {
		t.Fatalf("at 9: %q", got)
	}
}

// away names each role an Antigravity session holds, and its 10 wakes.
func TestH34p4AwayWarnsOfAntigravitySessions(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.as(t, antigravity("ag-1"), "role", "implementer")
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
	out, err := r.atTerminal(t, "", nil, "away", "Merge when approved.")
	if err != nil || !strings.Contains(out, "implementer is an Antigravity session: it listens with a wait it runs in the background. If it stops running one, handoff can wake it only 10 times before you type in it.") || strings.Contains(out, "reviewer is an Antigravity session") {
		t.Fatalf("away: %v\n%s", err, out)
	}
}

// handoff role with a role the project lacks says how to add it; a bad
// name gets the name rules.
func TestH34p4AnUnknownRoleSaysHowToAddIt(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := claude("s-1", cli(t))
	r.refusedIn(t, s, "reviewer-3 isn't a role here (the roles are implementer, reviewer, reviewer-2). If the operator gave it to you, run `", "role", "reviewer-3")
	r.refusedIn(t, s, " roles add reviewer-3`, then `", "role", "reviewer-3")
	r.refusedIn(t, s, "use lower-case letters, digits and '-'", "role", "Reviewer_3")
}

// Usage lists settings for all three CLIs.
func TestH34p4UsageListsAntigravity(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if res := r.run(t, "", "", nil, "help"); !strings.Contains(res.err, "settings claude|codex|antigravity") || !strings.Contains(res.err, "(Antigravity\n          takes no --as)") {
		t.Fatalf("usage:\n%s", res.err)
	}
}

// After an fyi, a session told that can't take it now is named, with when
// it will: blocked, or not listening now. One that listens isn't.
func TestH34p4AnFyiSaysWhenABlockedOrDeafSessionGetsIt(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-imp")
	r.as(t, agy, "role", "implementer")
	runAt(t, t.TempDir(), agyIn("ag-imp", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "2s") // the cap: blocked
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer")                                                              // not listening
	rev2 := claude("s-rev2", cli(t))
	r.as(t, rev2, "role", "reviewer-2")
	r.start(t, "", "", rev2, "wait", "--timeout", "60s")
	r.listenedFor(t, "reviewer-2")
	out, err := r.atTerminal(t, "", nil, "fyi", "Carry on.")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"implementer's session can't be woken; it gets this when you type in it. To reach the role now, give it to another session and send again.",
		"reviewer's session isn't listening now; it gets this the next time it listens.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("fyi lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "reviewer-2's session") {
		t.Fatalf("a listening session named:\n%s", out)
	}
}

// fyiHeld runs fyi at the operator's terminal, held at the told pause point
// after its write, and returns what releases it and gives what it printed.
func (r *repo) fyiHeld(t *testing.T, msg string) func() string {
	p, pause := pauseAt(t, "told")
	done := make(chan string, 1)
	go func() {
		out, _ := r.atTerminalWith(t, []string{pause}, nil, "fyi", msg)
		done <- out
	}()
	p.reached(t)
	return func() string { p.release(t); return <-done }
}

// reviewer-2's #624: the receipt's line is for the exact fyi. One its
// session's wait delivered meanwhile gets none; nor does one whose role
// moved meanwhile to a session it isn't for, at a prompt.
func TestH34p4TheFyiLineIsForTheExactFyi(t *testing.T) {
	t.Parallel()
	t.Run("delivered meanwhile", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		rev := claude("s-rev", cli(t))
		r.as(t, rev, "role", "reviewer")
		r.as(t, claude("s-imp", cli(t)), "role", "implementer")
		release := r.fyiHeld(t, "Carry on.")
		if res := r.run(t, "", "", rev, "wait", "--timeout", "5s"); !strings.Contains(res.out, "fyi") {
			t.Fatalf("the reviewer's wait: exit %d\n%s%s", res.code, res.out, res.err)
		}
		out := release()
		if strings.Contains(out, "reviewer's session isn't listening now") || !strings.Contains(out, "implementer's session isn't listening now") {
			t.Fatalf("the receipt:\n%s", out)
		}
	})
	t.Run("the role moved to a session at a prompt", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-old", cli(t)), "role", "implementer")
		release := r.fyiHeld(t, "Carry on.")
		nu := claude("s-new", cli(t))
		r.as(t, nu, "role", "implementer")
		r.run(t, "", `{"session_id":"s-new","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow A"}`, nu, "hook", "notify")
		if b := r.blockOf(t, "s-new"); b == nil {
			t.Fatal("no prompt on the new session")
		}
		if out := release(); strings.Contains(out, "implementer's session") {
			t.Fatalf("a line for a session the fyi isn't for:\n%s", out)
		}
	})
}
