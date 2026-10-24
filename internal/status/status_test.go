package status

import (
	"testing"

	"github.com/dre4success/handoff/internal/record"
)

const (
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type builder struct {
	entries []record.Entry
}

func (b *builder) add(e record.Entry) int64 {
	e.ID = int64(len(b.entries) + 1)
	b.entries = append(b.entries, e)
	return e.ID
}

func (b *builder) request(item, commit string, supersedes int64) int64 {
	return b.add(record.Entry{Kind: "request", Item: item, From: "implementer", To: "reviewer", Commit: commit, Supersedes: supersedes, Title: item})
}

func (b *builder) claim(re int64) int64 {
	return b.add(record.Entry{Kind: "claim", Item: "W1", From: "reviewer", Re: re})
}

func (b *builder) reply(re int64, verdict, commit string) int64 {
	return b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: re, Verdict: verdict, Commit: commit})
}

func (b *builder) note(from, to string, re int64) int64 {
	return b.add(record.Entry{Kind: "note", Item: "W1", From: from, To: to, Re: re, Body: "n"})
}

func (b *builder) decide(re int64, decision string) int64 {
	return b.add(record.Entry{Kind: "decision", Item: "W1", From: "operator", To: "implementer", Re: re, Decision: decision})
}

func stateOf(t *testing.T, s *Status, id int64) State {
	t.Helper()
	r, ok := s.Request(id)
	if !ok {
		t.Fatalf("no request #%d", id)
	}
	return r.State
}

func ids(rs []*Request) []int64 {
	var out []int64
	for _, r := range rs {
		out = append(out, r.Entry.ID)
	}
	return out
}

func TestRequestLifecycle(t *testing.T) {
	var b builder
	req := b.request("W1", commitA, 0)
	steps := []struct {
		do   func() int64
		want State
	}{
		{func() int64 { return 0 }, Open},
		{func() int64 { return b.claim(req) }, Claimed},
		{func() int64 { return b.reply(req, "QUESTION", commitA) }, Answered},
		{func() int64 { return b.note("implementer", "reviewer", 3) }, Claimed}, // answers the QUESTION (#3)
		{func() int64 { return b.reply(req, "APPROVE", commitA) }, AwaitingOperator},
		{func() int64 { return b.decide(req, "approve") }, Decided},
	}
	for i, st := range steps {
		st.do()
		if got := stateOf(t, Compute(b.entries), req); got != st.want {
			t.Fatalf("step %d: state = %s, want %s", i, got, st.want)
		}
	}
}

// H5 CHANGES 2: a note linked to the QUESTION but sent elsewhere is not the
// answer; only the one addressed to the reviewer is.
func TestOnlyANoteToTheReviewerAnswers(t *testing.T) {
	for _, to := range []string{Operator, "reviewer-2"} {
		var b builder
		req := b.request("W1", commitA, 0)
		q := b.reply(req, "QUESTION", commitA)
		b.note("implementer", to, q)
		s := Compute(b.entries)
		if got := stateOf(t, s, req); got != Answered {
			t.Fatalf("after a linked note to %s: state = %s, want answered", to, got)
		}
		if got := ids(s.Inbox("implementer").NeedsResponse); len(got) != 1 || got[0] != req {
			t.Fatalf("after a linked note to %s: implementer needs response = %v, want [%d]", to, got, req)
		}
		b.note("implementer", "reviewer", q)
		if got := stateOf(t, Compute(b.entries), req); got != Claimed {
			t.Fatalf("after the note to the reviewer: state = %s, want claimed", got)
		}
	}
}

func TestNoteAnsweringAnOlderReplyChangesNothing(t *testing.T) {
	var b builder
	req := b.request("W1", commitA, 0)
	q1 := b.reply(req, "QUESTION", commitA)
	b.reply(req, "CHANGES", commitA)
	b.note("implementer", "reviewer", q1)
	if got := stateOf(t, Compute(b.entries), req); got != Answered {
		t.Fatalf("state = %s, want answered: the note answers a reply that no longer stands", got)
	}
}

// Check 6, in the reducer: only the exact-commit verdict on the live revision changes state.
func TestSupersededRequestIgnoresLaterReplies(t *testing.T) {
	var b builder
	r1 := b.request("W1", commitA, 0)
	b.claim(r1)
	b.reply(r1, "CHANGES", commitA)
	r2 := b.request("W1", commitB, r1)
	b.reply(r1, "APPROVE", commitA) // recorded, changes nothing
	s := Compute(b.entries)
	if got := stateOf(t, s, r1); got != Superseded {
		t.Fatalf("r1 = %s, want superseded", got)
	}
	if r, _ := s.Request(r1); r.SupersededBy != r2 {
		t.Fatalf("r1 superseded by #%d, want #%d", r.SupersededBy, r2)
	}
	if got := stateOf(t, s, r2); got != Open {
		t.Fatalf("r2 = %s, want open", got)
	}
	b.reply(r2, "APPROVE", commitB)
	if got := stateOf(t, Compute(b.entries), r2); got != AwaitingOperator {
		t.Fatalf("r2 after APPROVE = %s, want awaiting operator", got)
	}
}

func TestLiveRequest(t *testing.T) {
	var b builder
	r1 := b.request("W1", commitA, 0)
	s := Compute(b.entries)
	if r, ok := s.Live("W1", "implementer", "reviewer"); !ok || r.Entry.ID != r1 {
		t.Fatalf("live = %v, %v; want #%d", r, ok, r1)
	}
	if _, ok := s.Live("W1", "implementer", "reviewer-2"); ok {
		t.Fatal("a request to reviewer is not live for reviewer-2")
	}
	b.request("W1", commitB, r1)
	if r, _ := Compute(b.entries).Live("W1", "implementer", "reviewer"); r.Entry.ID == r1 {
		t.Fatal("a superseded request is not live")
	}
}

func TestInboxLists(t *testing.T) {
	var b builder
	open := b.request("W1", commitA, 0)
	claimed := b.request("W2", commitA, 0)
	b.claim(claimed)
	answered := b.request("W3", commitA, 0)
	b.reply(answered, "CHANGES", commitA)
	approved := b.request("W4", commitA, 0)
	b.reply(approved, "APPROVE", commitA)
	toOperator := b.note("reviewer", "operator", 0)
	s := Compute(b.entries)

	rev := s.Inbox("reviewer")
	if got := ids(rev.NeedsReview); len(got) != 1 || got[0] != open {
		t.Fatalf("reviewer needs review = %v, want [%d]", got, open)
	}
	if got := ids(rev.BeingReviewed); len(got) != 1 || got[0] != claimed {
		t.Fatalf("reviewer being reviewed = %v, want [%d]", got, claimed)
	}
	impl := s.Inbox("implementer")
	if got := ids(impl.NeedsResponse); len(got) != 1 || got[0] != answered {
		t.Fatalf("implementer needs response = %v, want [%d]", got, answered)
	}
	if n := len(impl.AwaitingOperator); n != 2 {
		t.Fatalf("awaiting operator = %d entries, want the APPROVE on #%d and note #%d", n, approved, toOperator)
	}
	if !rev.Waiting() || !impl.Waiting() {
		t.Fatal("both agents have something waiting")
	}

	// Items awaiting the operator alone never make an agent's inbox count as waiting,
	// or a Stop hook would block on work the agent cannot do.
	var c builder
	r := c.request("W9", commitA, 0)
	c.reply(r, "APPROVE", commitA)
	s = Compute(c.entries)
	if s.Inbox("implementer").Waiting() || s.Inbox("reviewer").Waiting() {
		t.Fatal("an APPROVE awaiting the operator counts as waiting for an agent")
	}
	if !s.Inbox(Operator).Waiting() {
		t.Fatal("an APPROVE awaiting the operator must count as waiting for the operator")
	}
	c.decide(r, "approve")
	if Compute(c.entries).Inbox(Operator).Waiting() {
		t.Fatal("a decided item still waits for the operator")
	}
}

func TestDecisionOnOperatorNote(t *testing.T) {
	var b builder
	n := b.note("implementer", "operator", 0)
	if !Compute(b.entries).Inbox(Operator).Waiting() {
		t.Fatal("a note to the operator waits for a decision")
	}
	b.decide(n, "deny")
	if Compute(b.entries).Inbox(Operator).Waiting() {
		t.Fatal("a decided note still waits")
	}
}

func TestDecidable(t *testing.T) {
	var b builder
	r := b.request("W1", commitA, 0)
	if _, err := Compute(b.entries).Decidable(r); err == nil {
		t.Fatal("an open request is not for the operator to decide yet")
	}
	approve := b.reply(r, "APPROVE", commitA)
	s := Compute(b.entries)
	for _, id := range []int64{r, approve} {
		got, err := s.Decidable(id)
		if err != nil || got.Entry.ID != r {
			t.Fatalf("decidable(#%d) = %v, %v; want request #%d", id, got, err, r)
		}
	}
	b.decide(r, "approve")
	if _, err := Compute(b.entries).Decidable(r); err == nil {
		t.Fatal("a decided request cannot be decided again")
	}
}
