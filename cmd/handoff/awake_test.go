package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// H13: a listening session keeps the Mac from idle sleep while an agent owes
// a move, so the other agent's work goes on while the operator is away, and lets it
// sleep otherwise (§17 row 76). Holds go to a file here, not to caffeinate.
func TestH13ListenersHoldOffSleepWhileAMoveIsOwed(t *testing.T) {
	r := newRepo(t)
	path := filepath.Join(t.TempDir(), "awake.txt")
	envs := []string{"HANDOFF_TEST_AWAKE=" + path}

	if res := r.run(t, "", "", envs, "wait", "--for", "reviewer-2", "--timeout", "2s"); res.code != 3 {
		t.Fatalf("wait with nothing owed: exit %d, stderr %q", res.code, res.err)
	}
	if got := sentTo(t, path); got != nil {
		t.Fatalf("with nothing owed: %q; want no hold", got)
	}

	// A plain wait for another role holds while the reviewer owes a verdict,
	// and lets go when the APPROVE leaves the move to the operator.
	w, _, _ := r.start(t, "", "", envs, "wait", "--for", "reviewer-2", "--timeout", "60s")
	req := r.request(t, "W1", r.commitA)
	hold, release := fmt.Sprintf("hold %d", w.Process.Pid), fmt.Sprintf("release %d", w.Process.Pid)
	eventually(t, "a hold", func() bool { return slices.Equal(sentTo(t, path), []string{hold}) })
	l, _, _ := r.start(t, "implementer", `{"hook_event_name":"Stop"}`, envs, "hook", "listen", "--timeout", "60s")
	eventually(t, "the hook's hold", r.listening(t, "implementer", l.Process.Pid))
	eventually(t, "the hook's hold", func() bool { return len(sentTo(t, path)) == 2 })
	r.ok(t, "reviewer", "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	if code := exitCode(l.Wait()); code != 2 {
		t.Fatalf("the hook: exit %d; want 2, woken by the reply", code)
	}
	hookHold, hookRelease := fmt.Sprintf("hold %d", l.Process.Pid), fmt.Sprintf("release %d", l.Process.Pid)
	eventually(t, "both released", func() bool { return len(sentTo(t, path)) == 4 })
	got := sentTo(t, path)
	if !slices.Equal(got[:2], []string{hold, hookHold}) || !slices.Contains(got[2:], release) || !slices.Contains(got[2:], hookRelease) {
		t.Fatalf("holds %q; want %q and %q held, then both released", got, hold, hookHold)
	}
	if res := r.run(t, "", "", nil, "wait", "--for", "reviewer-2", "--timeout", "1s"); res.code != 4 {
		t.Fatalf("another wait for reviewer-2: exit %d, stderr %q; want 4, as the first still listens, without a hold", res.code, res.err)
	}
}
