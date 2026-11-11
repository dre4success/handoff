package status

import (
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// H28 (row 96): the operator's go-ahead while away approves an item once
// every live request on it is approved, all at one commit.

func (b *builder) to(item, reviewer, commit string, supersedes int64) int64 {
	return b.add(record.Entry{Kind: "request", Item: item, From: "implementer", To: reviewer, Commit: commit, Supersedes: supersedes, Title: item})
}

func (b *builder) verdict(item, reviewer string, re int64, verdict, commit string) int64 {
	return b.add(record.Entry{Kind: "reply", Item: item, From: reviewer, To: "implementer", Re: re, Verdict: verdict, Commit: commit})
}

func (b *builder) away(from string) int64 {
	return b.add(record.Entry{Kind: "away", From: from, Body: "Merge when the reviewers approve."})
}

func (b *builder) back() int64 {
	return b.add(record.Entry{Kind: "back", From: Operator, Body: "Back."})
}

func decides(st *Status) []string {
	var out []string
	for _, a := range st.Alerts(time.Now(), time.Hour) {
		if a.Kind == Decide {
			out = append(out, a.Text)
		}
	}
	return out
}

func notes(st *Status) []string {
	var out []string
	for _, a := range st.Alerts(time.Now(), time.Hour) {
		if a.Kind == GoAhead {
			out = append(out, a.Text)
		}
	}
	return out
}

const recorded = "Your go-ahead is recorded (#1): Merge when the reviewers approve. Until you run handoff back, items are approved without asking you."

// Every alert key must be a safe file name, as the notifier requires.
func keysSafe(t *testing.T, st *Status) {
	t.Helper()
	for _, a := range st.Alerts(time.Now(), time.Hour) {
		for _, c := range a.Key {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				t.Fatalf("alert key %q is not a safe file name", a.Key)
			}
		}
	}
}

func TestH28GoAheadNeedsEveryReviewerAtOneCommit(t *testing.T) {
	var b builder
	away := b.away(Operator)
	one, two := b.to("W1", "reviewer", commitA, 0), b.to("W1", "reviewer-2", commitA, 0)
	b.verdict("W1", "reviewer", one, "APPROVE", commitA)

	st := Compute(b.entries)
	if r, _ := st.Request(one); r.State != AwaitingOperator || r.GoAhead != nil {
		t.Fatalf("one of two approved: %s, go-ahead %v", r.State, r.GoAhead)
	}
	if d := decides(st); len(d) != 0 {
		t.Fatalf("away, a first approval alerts: %q", d)
	}
	if n := notes(st); len(n) != 1 || n[0] != recorded {
		t.Fatalf("away, after a first approval, the operator hears %q", n)
	}

	last := b.verdict("W1", "reviewer-2", two, "APPROVE", commitA)
	st = Compute(b.entries)
	for _, id := range []int64{one, two} {
		if r, _ := st.Request(id); r.State != Decided || r.GoAhead == nil || r.GoAhead.ID != away || r.Decision != nil {
			t.Fatalf("#%d: %s, go-ahead %v, decision %v", id, r.State, r.GoAhead, r.Decision)
		}
	}
	if d := decides(st); len(d) != 0 {
		t.Fatalf("an item approved under the go-ahead alerts: %q", d)
	}
	if n := notes(st); len(n) != 2 || n[1] != "W1: approved under your go-ahead #1, by reviewer and reviewer-2 (#2, #3)." {
		t.Fatalf("the operator hears %q", n)
	}
	keysSafe(t, st)
	if note := st.GoAheadNote(b.entries[last-1]); note != "W1 is approved under the operator's go-ahead #1: that is the operator's approve" {
		t.Fatalf("note %q", note)
	}
	if _, err := st.Decidable(one); err == nil {
		t.Fatal("an item approved under the go-ahead can still be decided")
	}
	if got := st.ApprovedSince(away); len(got) != 2 {
		t.Fatalf("approved since #%d: %d requests", away, len(got))
	}
}

// After CHANGES from one reviewer, a fix sent to that reviewer alone leaves
// the approvals on two commits: the go-ahead covers neither, and the
// operator hears of it once no request on the item is still in review.
func TestH28GoAheadRefusesApprovalsOnTwoCommits(t *testing.T) {
	var b builder
	b.away(Operator)
	one, two := b.to("W1", "reviewer", commitA, 0), b.to("W1", "reviewer-2", commitA, 0)
	b.verdict("W1", "reviewer", one, "APPROVE", commitA)
	b.verdict("W1", "reviewer-2", two, "CHANGES", commitA)
	if d := decides(Compute(b.entries)); len(d) != 0 {
		t.Fatalf("with CHANGES owed an answer, an alert: %q", d)
	}
	three := b.to("W1", "reviewer-2", commitB, two)
	b.verdict("W1", "reviewer-2", three, "APPROVE", commitB)
	st := Compute(b.entries)
	for _, id := range []int64{one, three} {
		if r, _ := st.Request(id); r.State != AwaitingOperator {
			t.Fatalf("#%d: %s, want awaiting the operator", id, r.State)
		}
	}
	if n := notes(st); len(n) != 1 {
		t.Fatalf("approvals at two commits: the operator hears %q", n)
	}
	d := decides(st)
	if len(d) != 2 || !strings.Contains(d[0], "the approvals on W1 name different commits, so your go-ahead doesn't cover it. Your decision.") {
		t.Fatalf("alerts %q", d)
	}
}

// An away entry approves what is approved already, and nothing a reviewer
// hasn't approved. Back ends it: what it approved stays approved, and a
// later approval waits for the operator. An away entry from anyone but the
// operator changes nothing.
func TestH28GoAheadStartsAndEnds(t *testing.T) {
	var b builder
	done := b.to("W1", "reviewer", commitA, 0)
	b.verdict("W1", "reviewer", done, "APPROVE", commitA)
	open := b.to("W2", "reviewer", commitA, 0)
	b.away("implementer")
	if st := Compute(b.entries); st.Away != nil {
		t.Fatal("an away entry from the implementer took effect")
	}
	away := b.away(Operator)
	st := Compute(b.entries)
	if r, _ := st.Request(done); r.State != Decided || r.GoAhead.ID != away {
		t.Fatalf("approved before the go-ahead: %s", r.State)
	}
	if r, _ := st.Request(open); r.State != Open {
		t.Fatalf("never approved: %s", r.State)
	}
	if n := notes(st); len(n) != 2 || n[1] != "W1: approved under your go-ahead #5, by reviewer (#1)." {
		t.Fatalf("an away entry that approves: the operator hears %q", n)
	}
	again := b.away(Operator)
	if st := Compute(b.entries); st.Away.ID != again || st.AwaySince.ID != away {
		t.Fatalf("a second away: away #%d since #%d", st.Away.ID, st.AwaySince.ID)
	}
	b.back()
	b.verdict("W2", "reviewer", open, "APPROVE", commitA)
	st = Compute(b.entries)
	if st.Away != nil || st.AwaySince != nil {
		t.Fatal("still away after back")
	}
	if n := notes(st); len(n) != 0 {
		t.Fatalf("after back, the operator hears %q", n)
	}
	if r, _ := st.Request(done); r.State != Decided {
		t.Fatalf("back undid an approval under the go-ahead: %s", r.State)
	}
	if r, _ := st.Request(open); r.State != AwaitingOperator {
		t.Fatalf("approved after back: %s", r.State)
	}
	if d := decides(st); len(d) != 1 || !strings.HasSuffix(d[0], "approved #3. Your decision.") {
		t.Fatalf("alerts after back %q", d)
	}
}
