package wait

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// H34 part 3 (row 111): a listener holds an APPROVE back from its requester
// while the rest of its item at the commit is still reviewed, for at most
// 30 minutes; whatever the requester must act on carries the held ones.

// held is a record whose entries take their time from the fake clock.
type held struct {
	t     *testing.T
	s     *record.Store
	clock *fakeClock
}

func newHeld(t *testing.T) *held {
	h := &held{t: t, s: newStore(t), clock: newClock()}
	h.s.Now = func() time.Time { return h.clock.now }
	return h
}

func (h *held) add(e record.Entry) int64 {
	h.t.Helper()
	got, _, err := h.s.Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) { return &e, nil, nil })
	if err != nil {
		h.t.Fatal(err)
	}
	return got.ID
}

func (h *held) ask(to, commit string) int64 {
	return h.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: to, Commit: commit, Title: "W1"})
}

func (h *held) says(req int64, from, verdict, commit string) int64 {
	return h.add(record.Entry{Kind: "reply", Item: "W1", From: from, To: "implementer", Re: req, Verdict: verdict, Commit: commit})
}

// listen runs the implementer's wait, with each tick a minute, until ticks
// pass or something is delivered, and returns what it printed.
func (h *held) listen(minutes int, during func(tick int)) (int, string) {
	h.t.Helper()
	h.clock.hook = func(c *fakeClock) {
		if during != nil {
			during(c.ticks)
		}
	}
	h.clock.ticks = 0
	code, out, _ := run(h.t, h.s, Options{Role: "implementer", Timeout: time.Duration(minutes) * time.Minute, Interval: time.Minute, Clock: h.clock})
	return code, out
}

// ids lists the entry ids a wait's output delivered, in order.
func ids(out string) []string {
	var got []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "#") {
			got = append(got, strings.Fields(l)[0])
		}
	}
	return got
}

func TestH34p3OneWakeForThreeReviewers(t *testing.T) {
	h := newHeld(t)
	r1, r2, r3 := h.ask("reviewer", "c"), h.ask("reviewer-2", "c"), h.ask("reviewer-3", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	code, out := h.listen(3, func(tick int) {
		if tick == 1 {
			h.says(r2, "reviewer-2", "APPROVE", "c")
		}
		if tick == 2 {
			h.says(r3, "reviewer-3", "APPROVE", "c")
		}
	})
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#4 #5 #6" {
		t.Fatalf("exit %d, delivered %q", code, got)
	}
	if c := cursor(t, h.s, "implementer"); c != 6 {
		t.Fatalf("cursor %d", c)
	}
}

// A CHANGES is delivered at once, and carries the APPROVE held before it.
func TestH34p3AHeldApproveIsCarriedByAChanges(t *testing.T) {
	h := newHeld(t)
	r1, r2 := h.ask("reviewer", "c"), h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	if code, out := h.listen(2, nil); code != ExitTimeout || out != "" {
		t.Fatalf("held: exit %d:\n%s", code, out)
	}
	h.says(r2, "reviewer-2", "CHANGES", "c")
	if code, out := h.listen(2, nil); code != ExitDelivered || strings.Join(ids(out), " ") != "#3 #4" {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

// With nothing new written, the hold ends 30 minutes after the APPROVE; a
// restart meanwhile keeps it, and moves no cursor.
func TestH34p3TheHoldEndsAtThirtyMinutesAndARestartKeepsIt(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	if code, out := h.listen(10, nil); code != ExitTimeout || out != "" || cursor(t, h.s, "implementer") != 0 {
		t.Fatalf("within the hold: exit %d, %q", code, out)
	}
	code, out := h.listen(25, nil) // a new listener, as after a restart
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#3" || h.clock.ticks > 21 {
		t.Fatalf("after 30 minutes: exit %d, %q, at tick %d", code, got, h.clock.ticks)
	}
}

// An fyi for another session behind a held one waits with it, the cursor
// unmoved; an urgent decision then carries both, the fyi passed over.
func TestH34p3AnFyiBehindAHeldOneThenADecision(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	h.add(record.Entry{Kind: "fyi", From: "operator", To: "implementer", Session: "another", Body: "not yours"})
	if code, _ := h.listen(2, nil); code != ExitTimeout || cursor(t, h.s, "implementer") != 0 {
		t.Fatalf("exit %d, cursor %d", code, cursor(t, h.s, "implementer"))
	}
	h.add(record.Entry{Kind: "note", Item: "W1", From: "operator", To: "implementer", Body: "stop"})
	code, out := h.listen(2, nil)
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#3 #5" || cursor(t, h.s, "implementer") != 5 {
		t.Fatalf("exit %d, delivered %q, cursor %d", code, got, cursor(t, h.s, "implementer"))
	}
}

// The request still owed replaced at another commit, by an entry addressed
// to no one the listener serves: the APPROVE no longer qualifies, and goes.
func TestH34p3ASupersessionReleasesIt(t *testing.T) {
	h := newHeld(t)
	r1, r2 := h.ask("reviewer", "c"), h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	code, out := h.listen(3, func(tick int) {
		if tick == 1 {
			h.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer-2", Commit: "d", Supersedes: r2, Title: "W1"})
		}
	})
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#3" || h.clock.ticks != 1 {
		t.Fatalf("exit %d, delivered %q at tick %d, want it held until tick 1", code, got, h.clock.ticks)
	}
}

// Requests at different commits, and an APPROVE on a request already
// superseded, hold nothing.
func TestH34p3MixedCommitsAndASupersededRequestHoldNothing(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "d")
	h.says(r1, "reviewer", "APPROVE", "c")
	if code, out := h.listen(1, nil); code != ExitDelivered || strings.Join(ids(out), " ") != "#3" {
		t.Fatalf("mixed commits: exit %d:\n%s", code, out)
	}
	h2 := newHeld(t)
	a := h2.ask("reviewer", "c")
	h2.ask("reviewer-2", "c")
	h2.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: "c", Supersedes: a, Title: "W1"})
	h2.says(a, "reviewer", "APPROVE", "c")
	if code, out := h2.listen(1, nil); code != ExitDelivered || strings.Join(ids(out), " ") != "#4" {
		t.Fatalf("on a superseded request: exit %d:\n%s", code, out)
	}
}

// Another role's request on the item holds it too; that role's review
// done, by an entry addressed to it, the APPROVE goes.
func TestH34p3ReleasedByAnEntryToAnotherRole(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	other := h.add(record.Entry{Kind: "request", Item: "W1", From: "reviewer-2", To: "reviewer-3", Commit: "c", Title: "W1"})
	h.says(r1, "reviewer", "APPROVE", "c")
	code, out := h.listen(3, func(tick int) {
		if tick == 1 {
			h.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer-3", To: "reviewer-2", Re: other, Verdict: "APPROVE", Commit: "c"})
		}
	})
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#3" || h.clock.ticks != 1 {
		t.Fatalf("exit %d, delivered %q at tick %d, want it held until tick 1", code, got, h.clock.ticks)
	}
}

// Two held, the first's 30 minutes end: both go in that one wake.
func TestH34p3TwoHeldGoInOneWake(t *testing.T) {
	h := newHeld(t)
	r1, r2 := h.ask("reviewer", "c"), h.ask("reviewer-2", "c")
	h.ask("reviewer-3", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	h.clock.now = h.clock.now.Add(10 * time.Minute)
	h.says(r2, "reviewer-2", "APPROVE", "c")
	code, out := h.listen(25, nil)
	if got := strings.Join(ids(out), " "); code != ExitDelivered || got != "#4 #5" || h.clock.ticks < 19 || h.clock.ticks > 21 {
		t.Fatalf("exit %d, delivered %q at tick %d, want both at the first's 30 minutes, tick 20", code, got, h.clock.ticks)
	}
}

// --emit streams every entry as it lands, held or not.
func TestH34p3EmitHoldsNothing(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	h.clock.hook = nil
	_, out, _ := run(t, h.s, Options{Role: "implementer", Timeout: 2 * time.Minute, Interval: time.Minute, Clock: h.clock, Emit: true})
	if got := strings.Join(ids(out), " "); got != "#3" {
		t.Fatalf("emit delivered %q", got)
	}
}

// reviewer-2's #535, point 1: requests at A, A and B on one item. The go-
// ahead's test wants one commit for the item, so with mixed commits nothing
// is held, the A-and-A pair included.
func TestH34p3MixedCommitsWithAPairHoldNothing(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "c")
	h.ask("reviewer-3", "d")
	h.says(r1, "reviewer", "APPROVE", "c")
	if code, out := h.listen(1, nil); code != ExitDelivered || strings.Join(ids(out), " ") != "#4" || h.clock.ticks != 0 {
		t.Fatalf("exit %d at tick %d:\n%s", code, h.clock.ticks, out)
	}
}

// reviewer-2's #535, point 2: a wait that ends at a break in the chain still
// gives the verified entries before it, a held APPROVE included.
func TestH34p3ABreakEndsTheHold(t *testing.T) {
	h := newHeld(t)
	r1 := h.ask("reviewer", "c")
	h.ask("reviewer-2", "c")
	h.says(r1, "reviewer", "APPROVE", "c")
	appendForged(t, h.s, 99)
	var out, errOut bytes.Buffer
	_, err := Run(h.s, Options{Role: "implementer", Timeout: time.Minute, Interval: time.Second, Clock: h.clock, Out: &out, Err: &errOut})
	if !errors.Is(err, record.ErrBroken) || strings.Join(ids(out.String()), " ") != "#3" || cursor(t, h.s, "implementer") != 3 {
		t.Fatalf("err %v, delivered %q, cursor %d", err, out.String(), cursor(t, h.s, "implementer"))
	}
}
