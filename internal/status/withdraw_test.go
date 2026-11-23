package status

import (
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// H38 (row 118): a request its requester withdraws is owed by no one, and
// keeps the go-ahead off its item until its reviewer is asked again.

func (b *builder) withdraw(from string, re int64) int64 {
	return b.add(record.Entry{Kind: "withdraw", Item: b.entries[re-1].Item, From: from, To: b.entries[re-1].To, Re: re})
}

// round adds a complete round on W1 at commit to reviewer and reviewer-2,
// and returns its two requests.
func (b *builder) round(commit string) (int64, int64) {
	panel := []string{"reviewer", "reviewer-2"}
	first := int64(len(b.entries) + 1)
	one := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commit, Round: first, Panel: panel, Title: "W1"})
	two := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commit, Round: first, Panel: panel, Title: "W1"})
	return one, two
}

// Open, claimed or answered with CHANGES, a withdrawn request is in no
// inbox, raises no quiet alert, and is no work for the stuck check. Only its
// requester withdraws it, and only before an approval.
func TestH38AWithdrawnRequestIsOwedByNoOne(t *testing.T) {
	b := &builder{}
	open := b.request("W1", commitA, 0)
	claimed := b.request("W2", commitA, 0)
	b.claim(claimed)
	answered := b.request("W3", commitA, 0)
	b.reply(answered, "CHANGES", commitA)
	approved := b.request("W4", commitA, 0)
	b.reply(approved, "APPROVE", commitA)
	b.withdraw("reviewer", open) // not its requester: changes nothing
	if st := Compute(b.entries); stateOf(t, st, open) != Open {
		t.Fatalf("withdrawn by its reviewer: %s", stateOf(t, st, open))
	}
	w := b.withdraw("implementer", open)
	b.withdraw("implementer", claimed)
	b.withdraw("implementer", answered)
	b.withdraw("implementer", approved) // approved: the operator's to decide
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	for i := range b.entries {
		b.entries[i].TS = old
	}
	st := Compute(b.entries)
	for _, id := range []int64{open, claimed, answered} {
		if stateOf(t, st, id) != Withdrawn {
			t.Fatalf("#%d: %s", id, stateOf(t, st, id))
		}
	}
	if r, _ := st.Request(open); r.WithdrawnBy == nil || r.WithdrawnBy.ID != w {
		t.Fatalf("withdrawn by: %+v", r.WithdrawnBy)
	}
	if stateOf(t, st, approved) != AwaitingOperator {
		t.Fatalf("an approved request withdrawn: %s", stateOf(t, st, approved))
	}
	rev, imp := st.Inbox("reviewer"), st.Inbox("implementer")
	if len(rev.NeedsReview)+len(rev.BeingReviewed)+len(imp.NeedsResponse) != 0 {
		t.Fatalf("inboxes: %v %v %v", ids(rev.NeedsReview), ids(rev.BeingReviewed), ids(imp.NeedsResponse))
	}
	for _, a := range st.Alerts(time.Now(), time.Hour) {
		if a.Kind == Quiet {
			t.Fatalf("a quiet alert: %q", a.Text)
		}
	}
	if owed := st.OwedBy("reviewer", 0); len(owed) != 0 {
		t.Fatalf("owed by the reviewer: %+v", owed)
	}
	if got := Summary(b.entries[w-1]); got != "#"+itoa(w)+" withdraw W1 implementer → reviewer: withdrew #"+itoa(open) {
		t.Fatalf("summary: %q", got)
	}
}

// Under the go-ahead, the request withdrawn from a round that asked for
// changes doesn't leave the rest to approve the item: the operator decides,
// and is told why. Once that reviewer is asked again and approves, the
// go-ahead approves as before.
func TestH38AWithdrawalKeepsTheGoAheadOff(t *testing.T) {
	b := &builder{}
	b.away(Operator)
	one, two := b.round(commitA)
	b.verdict("W1", "reviewer-2", two, "CHANGES", commitA)
	b.withdraw("implementer", two)
	b.verdict("W1", "reviewer", one, "APPROVE", commitA)
	st := Compute(b.entries)
	if r, _ := st.Request(one); r.State != AwaitingOperator || r.GoAhead != nil {
		t.Fatalf("approved under the go-ahead with a request withdrawn: %s, %v", r.State, r.GoAhead)
	}
	want := "W1: reviewer approved #" + itoa(one) + ", but implementer withdrew its request #" + itoa(two) + " to reviewer-2, so your go-ahead doesn't cover it. Your decision."
	if d := decides(st); len(d) != 1 || d[0] != want {
		t.Fatalf("decide alerts: %q\nwant %q", d, want)
	}

	again := b.to("W1", "reviewer-2", commitA, 0)
	b.verdict("W1", "reviewer-2", again, "APPROVE", commitA)
	st = Compute(b.entries)
	for _, id := range []int64{one, again} {
		if r, _ := st.Request(id); r.State != Decided || r.GoAhead == nil {
			t.Fatalf("#%d, reviewer-2 asked again and approving: %s, %v", id, r.State, r.GoAhead)
		}
	}
}

// With the operator here, the decision says a reviewer's request was
// withdrawn, and the operator's approve is work to merge, the withdrawn
// request left out of it.
func TestH38AnApprovalLeavesTheWithdrawnOut(t *testing.T) {
	b := &builder{}
	one, two := b.round(commitA)
	b.verdict("W1", "reviewer-2", two, "CHANGES", commitA)
	b.withdraw("implementer", two)
	b.verdict("W1", "reviewer", one, "APPROVE", commitA)
	st := Compute(b.entries)
	if d := decides(st); len(d) != 1 || !strings.Contains(d[0], "; implementer withdrew its request #"+itoa(two)+" to reviewer-2. Your decision.") {
		t.Fatalf("decide alerts: %q", d)
	}
	b.decide(one, "approve")
	aps := Compute(b.entries).Approvals(0)
	if len(aps) != 1 || len(aps[0].Requests) != 1 || aps[0].Requests[0].Entry.ID != one {
		t.Fatalf("approvals: %+v", aps)
	}
}
