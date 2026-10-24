package status

import (
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// timed builds entries stamped minutes after t0.
type timed struct{ builder }

func (b *timed) at(min int, e record.Entry) int64 {
	e.TS = t0.Add(time.Duration(min) * time.Minute).Format(time.RFC3339)
	return b.add(e)
}

func after(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func texts(as []Alert) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Text)
	}
	return out
}

// one checks there is exactly one alert, of the kind given, mentioning want.
func one(t *testing.T, as []Alert, kind AlertKind, want string) Alert {
	t.Helper()
	if len(as) != 1 || as[0].Kind != kind || !strings.Contains(as[0].Text, want) {
		t.Fatalf("alerts = %q; want one %s alert mentioning %q", texts(as), kind, want)
	}
	return as[0]
}

func none(t *testing.T, as []Alert) {
	t.Helper()
	if len(as) != 0 {
		t.Fatalf("alerts = %q; want none", texts(as))
	}
}

const quiet = 30 * time.Minute

// The trial's stall: a claim, then nothing. Each step owes a move to someone;
// the operator hears when the move is overdue, once per step.
func TestQuietAlertFollowsWhoOwesTheNextMove(t *testing.T) {
	var b timed
	req := b.at(0, record.Entry{Kind: "request", Item: "H8", From: "claude", To: "codex", Commit: commitA, Title: "H8"})
	st := Compute(b.entries)
	none(t, st.Alerts(after(29), quiet))
	open := one(t, st.Alerts(after(31), quiet), Quiet, "H8: codex hasn't picked up #1 (31 min)")

	b.at(10, record.Entry{Kind: "claim", Item: "H8", From: "codex", Re: req})
	st = Compute(b.entries)
	none(t, st.Alerts(after(39), quiet))
	claimed := one(t, st.Alerts(after(41), quiet), Quiet, "H8: codex claimed #1 and has been quiet for 31 min")
	if claimed.Key == open.Key {
		t.Fatalf("the claim did not change the key %q, so the operator would not hear about the new silence", open.Key)
	}

	q := b.at(50, record.Entry{Kind: "reply", Item: "H8", From: "codex", To: "claude", Re: req, Verdict: "QUESTION", Commit: commitA})
	st = Compute(b.entries)
	none(t, st.Alerts(after(79), quiet))
	one(t, st.Alerts(after(81), quiet), Quiet, "H8: claude hasn't answered the QUESTION on #1 (31 min)")

	b.at(85, record.Entry{Kind: "note", Item: "H8", From: "claude", To: "codex", Re: q, Body: "answer"})
	st = Compute(b.entries)
	none(t, st.Alerts(after(114), quiet))
	one(t, st.Alerts(after(116), quiet), Quiet, "H8: codex claimed #1 and has been quiet for 31 min")
}

func TestLongSilenceIsGivenInHours(t *testing.T) {
	var b timed
	req := b.at(0, record.Entry{Kind: "request", Item: "H7", From: "implementer", To: "reviewer", Commit: commitA})
	b.at(1, record.Entry{Kind: "claim", Item: "H7", From: "reviewer", Re: req})
	one(t, Compute(b.entries).Alerts(after(1+46*60), quiet), Quiet, "quiet for 46 h")
}

// A claim from a role the request is not addressed to moves nothing.
func TestAnotherRolesClaimDoesNotRestartTheClock(t *testing.T) {
	var b timed
	req := b.at(0, record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA})
	b.at(1, record.Entry{Kind: "claim", Item: "W1", From: "reviewer", Re: req})
	b.at(20, record.Entry{Kind: "claim", Item: "W1", From: "reviewer-2", Re: req})
	one(t, Compute(b.entries).Alerts(after(35), quiet), Quiet, "quiet for 34 min")
}

// An APPROVE needs the operator at once; it is not a silence to wait out.
func TestApproveAlertsTheOperatorAtOnceUntilDecided(t *testing.T) {
	var b timed
	req := b.at(0, record.Entry{Kind: "request", Item: "H8", From: "claude", To: "codex", Commit: commitA})
	b.at(5, record.Entry{Kind: "reply", Item: "H8", From: "codex", To: "claude", Re: req, Verdict: "APPROVE", Commit: commitA})
	st := Compute(b.entries)
	a := one(t, st.Alerts(after(5), quiet), Decide, "H8: codex approved #1. Your decision")
	if a.Target != req {
		t.Fatalf("target #%d; want the request, #%d, which is what the operator decides", a.Target, req)
	}
	if again := st.Alerts(after(500), quiet); len(again) != 1 || again[0].Key != a.Key {
		t.Fatalf("hours later: %q; want the same single decide alert, so it is sent once", texts(again))
	}
	b.at(10, record.Entry{Kind: "decision", Item: "H8", From: "operator", To: "claude", Re: req, Decision: "approve"})
	none(t, Compute(b.entries).Alerts(after(500), quiet))
}

func TestNoteToTheOperatorSendsAnAlert(t *testing.T) {
	var b timed
	n := b.at(0, record.Entry{Kind: "note", Item: "H8", From: "claude", To: Operator, Title: "May I deploy?", Body: "b"})
	if a := one(t, Compute(b.entries).Alerts(after(0), quiet), Decide, "H8: claude asks you: May I deploy?"); a.Target != n {
		t.Fatalf("target #%d; want the note, #%d", a.Target, n)
	}
	b.at(1, record.Entry{Kind: "decision", Item: "H8", From: Operator, To: "claude", Re: n, Decision: "approve"})
	none(t, Compute(b.entries).Alerts(after(60), quiet))
}

// A superseded request owes nothing; its replacement does.
func TestSupersededRequestIsNotQuiet(t *testing.T) {
	var b timed
	first := b.at(0, record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA})
	b.at(5, record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: commitB, Supersedes: first})
	one(t, Compute(b.entries).Alerts(after(40), quiet), Quiet, "hasn't picked up #2 (35 min)")
}

// Entries written before timestamps were checked must not crash the rule.
func TestUnreadableTimestampIsSkipped(t *testing.T) {
	var b builder
	b.request("W1", commitA, 0) // no TS
	none(t, Compute(b.entries).Alerts(after(600), quiet))
}

// While the requester owes a response, a claim from the reviewer is not the
// move that is owed, so it does not restart the requester's clock.
func TestClaimDoesNotRestartTheRequestersClock(t *testing.T) {
	var b timed
	req := b.at(0, record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA})
	b.at(1, record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: req, Verdict: "CHANGES", Commit: commitA})
	b.at(20, record.Entry{Kind: "claim", Item: "W1", From: "reviewer", Re: req})
	st := Compute(b.entries)
	if got := stateOf(t, st, req); got != Answered {
		t.Fatalf("state = %s, want answered", got)
	}
	one(t, st.Alerts(after(35), quiet), Quiet, "implementer hasn't answered the CHANGES on #1 (34 min)")
}

// An agent owes a move while a request waits for a verdict or a response,
// and only for so long after its last entry; nobody owes one while it waits
// for the operator, or once it is decided or superseded (row 76).
func TestOwedFollowsTheMoveAndItsAge(t *testing.T) {
	const within = 2 * time.Hour
	var b timed
	owed := func(min int, want bool) {
		t.Helper()
		if got := Compute(b.entries).Owed(after(min), within); got != want {
			t.Fatalf("at %d min, owed = %v, want %v", min, got, want)
		}
	}
	owed(0, false)
	req := b.at(0, record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA})
	owed(1, true)
	owed(119, true)
	owed(120, false)
	b.at(110, record.Entry{Kind: "claim", Item: "W1", From: "reviewer", Re: req})
	owed(120, true)
	q := b.at(130, record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: req, Verdict: "QUESTION", Commit: commitA})
	owed(131, true)
	b.at(140, record.Entry{Kind: "note", Item: "W1", From: "implementer", To: "reviewer", Re: q, Body: "answer"})
	owed(141, true)
	b.at(150, record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: req, Verdict: "APPROVE", Commit: commitA})
	owed(151, false)
	b.at(160, record.Entry{Kind: "decision", Item: "W1", From: "operator", To: "implementer", Re: req, Decision: "approve"})
	owed(161, false)

	old := b.at(170, record.Entry{Kind: "request", Item: "W2", From: "implementer", To: "reviewer", Commit: commitA})
	owed(171, true)
	b.at(180, record.Entry{Kind: "request", Item: "W2", From: "implementer", To: "reviewer", Commit: commitB, Supersedes: old})
	if got := stateOf(t, Compute(b.entries), old); got != Superseded {
		t.Fatalf("state = %s, want superseded", got)
	}
	owed(181, true) // the new request
	b.at(190, record.Entry{Kind: "note", Item: "W2", From: "implementer", To: "operator", Body: "a question for the operator"})
	owed(301, false)
}
