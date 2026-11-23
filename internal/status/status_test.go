package status

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

// H34 part 7 (row 106): a round whose requests don't match each other, which
// only an edit of the record could make, never completes: it keeps the
// go-ahead from approving its item, and says it can't be repaired.
func TestH34p7ABrokenRoundBlocksAndSaysSo(t *testing.T) {
	panel := []string{"reviewer", "reviewer-2"}
	member := func(to, commit string, round int64) record.Entry {
		return record.Entry{Kind: "round", Item: "W1", From: "implementer", To: to, Commit: commit, Round: round, Panel: panel, Title: "W1"}
	}
	for name, es := range map[string][]record.Entry{
		"another commit": {member("reviewer", commitA, 2), member("reviewer-2", commitB, 2)},
		"another round":  {member("reviewer", commitA, 1), member("reviewer-2", commitA, 1)},
		"no round":       {member("reviewer", commitA, 0), member("reviewer-2", commitA, 0)},
		// reviewer-3, #328: a stray request after the round completed.
		"a stray after completing": {member("reviewer", commitA, 2), member("reviewer-2", commitA, 2), member("reviewer-2", commitA, 2)},
	} {
		b := &builder{}
		b.add(record.Entry{Kind: "away", From: Operator, Body: "go"})
		var ids []int64
		for _, e := range es {
			ids = append(ids, b.add(e))
		}
		for _, id := range ids {
			b.add(record.Entry{Kind: "reply", Item: "W1", From: b.entries[id-1].To, To: "implementer", Re: id, Verdict: "APPROVE", Commit: b.entries[id-1].Commit})
		}
		s := Compute(b.entries)
		if len(s.Rounds) == 0 {
			t.Fatalf("%s: no round", name)
		}
		for _, rd := range s.Rounds {
			if rd.Complete || !rd.Pending() {
				t.Fatalf("%s: round #%d complete %v, pending %v", name, rd.ID, rd.Complete, rd.Pending())
			}
		}
		if !s.Rounds[0].Broken() || !s.roundBlocks("W1") {
			t.Fatalf("%s: broken %v, blocks %v", name, s.Rounds[0].Broken(), s.roundBlocks("W1"))
		}
		for _, id := range ids {
			if r, _ := s.Request(id); r.GoAhead != nil {
				t.Fatalf("%s: #%d approved under the go-ahead", name, id)
			}
		}
		if got := s.Rounds[0].MissingText(); got != "the round #"+itoa(s.Rounds[0].ID)+"'s requests don't match each other, which posting again can't repair" {
			t.Fatalf("%s: %q", name, got)
		}
	}
}

func itoa(n int64) string { return fmt.Sprint(n) }

// Row 108: an approval not in main is owed by its requester, keeps the Mac
// awake within the cap, and waits in its inbox; one git can't tell about is
// listed and owed by no one; held or merged, it is done.
func TestH34p1AnApprovalNotInMainIsOwed(t *testing.T) {
	b := &builder{}
	req := b.request("W1", commitA, 0)
	b.reply(req, "APPROVE", commitA)
	now := time.Now().UTC()
	for i := range b.entries {
		b.entries[i].TS = now.Add(-time.Hour).Format(time.RFC3339)
	}
	d := b.decide(req, "approve")
	b.entries[d-1].TS = now.Add(-time.Minute).Format(time.RFC3339)
	s := Compute(b.entries)
	aps := s.Approvals(0)
	if len(aps) != 1 || aps[0].By.ID != d || aps[0].Requests[0].Entry.ID != req {
		t.Fatalf("approvals: %+v", aps)
	}
	if s.Owed(now, AwakeCap) {
		t.Fatal("owed before the caller set what git says")
	}
	s.SetToMerge(aps)
	if !s.Owed(now, AwakeCap) || s.Owed(now, 30*time.Second) || !s.Inbox("implementer").Waiting() || s.Inbox("reviewer").Waiting() {
		t.Fatal("an approval not in main: not owed, or not waiting for its requester alone")
	}
	aps[0].Err = fmt.Errorf("can't tell")
	if s.Owed(now, AwakeCap) || !s.Inbox("implementer").Waiting() {
		t.Fatal("one git can't tell about: owed, or not listed")
	}
	if len(s.Approvals(d)) != 0 {
		t.Fatal("an approval up to the tracked id is listed")
	}
	b.add(record.Entry{Kind: "decision", Item: "W1", From: Operator, To: "implementer", Re: req, Decision: "deny"})
	if len(Compute(b.entries).Approvals(0)) != 0 {
		t.Fatal("held, still listed")
	}
}

// AwakeCap stands in for wait.AwakeWithin, which status can't import.
const AwakeCap = 2 * time.Hour

// reviewer-2's #467, point 1: one request of a round a cut write left
// incomplete, approved by the operator's decision, is no approval to merge:
// a round's panel is approved whole.
func TestH34p1AnIncompleteRoundIsNoApproval(t *testing.T) {
	b := &builder{}
	id := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA, Round: 1, Panel: []string{"reviewer", "reviewer-2"}, Title: "W1"})
	b.reply(id, "APPROVE", commitA)
	b.decide(id, "approve")
	if aps := Compute(b.entries).Approvals(0); len(aps) != 0 {
		t.Fatalf("approvals of a cut round: %+v", aps[0])
	}
}

// reviewer-2's #467, point 2: what tells the requester of work the go-ahead
// itself approved is the fyi to its session, not the APPROVE before it; with
// no such fyi, nothing did.
func TestH34p1WhatToldTheRequester(t *testing.T) {
	for _, fyi := range []bool{true, false} {
		b := &builder{}
		req := b.request("W1", commitA, 0)
		b.reply(req, "APPROVE", commitA)
		away := b.add(record.Entry{Kind: "away", From: Operator, Body: "go"})
		var told int64
		if fyi {
			told = b.add(record.Entry{Kind: "fyi", From: Operator, To: "implementer", Re: away, Session: "s-1", Body: "go"})
		}
		aps := Compute(b.entries).Approvals(0)
		if len(aps) != 1 || aps[0].By.ID != away || aps[0].Told != told {
			t.Fatalf("fyi %v: %+v", fyi, aps)
		}
	}
}

// reviewer-2's #467, point 3: closing an approval closes its own requests,
// and leaves an earlier approval at the same commit as it was.
func TestH34p1ClosingAnApprovalLeavesAnEarlierOne(t *testing.T) {
	b := &builder{}
	first := b.request("W1", commitA, 0)
	b.reply(first, "APPROVE", commitA)
	b.decide(first, "approve")
	m1 := b.add(record.Entry{Kind: "merged", Item: "W1", From: "implementer", Re: first, Commit: commitB})
	second := b.request("W1", commitA, 0)
	b.reply(second, "APPROVE", commitA)
	b.decide(second, "approve")
	m2 := b.add(record.Entry{Kind: "merged", Item: "W1", From: "implementer", Re: second, Commit: commitB})
	s := Compute(b.entries)
	r1, _ := s.Request(first)
	r2, _ := s.Request(second)
	if r1.Merged == nil || r1.Merged.ID != m1 || r2.Merged == nil || r2.Merged.ID != m2 {
		t.Fatalf("merged: first %+v, second %+v", r1.Merged, r2.Merged)
	}
}

// reviewer-2's note on the design in #467: an approval is stale only when
// the record shows a newer review replaced it, as when the requester's next
// round supersedes the request of a reviewer who asked for changes; one the
// requester keeps open beside newer work stays listed.
func TestH34p1StaleOnlyWhenANewerReviewReplacedIt(t *testing.T) {
	b := &builder{}
	panel := []string{"reviewer", "reviewer-2"}
	r1 := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA, Round: 1, Panel: panel, Title: "W1"})
	r2 := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commitA, Round: 1, Panel: panel, Title: "W1"})
	b.reply(r1, "APPROVE", commitA)
	b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer-2", To: "implementer", Re: r2, Verdict: "CHANGES", Commit: commitA})
	b.decide(r1, "approve") // in a pop-up, while the CHANGES stands
	if aps := Compute(b.entries).Approvals(0); len(aps) != 0 {
		t.Fatalf("approved while a CHANGES stands: %+v", aps[0])
	}
	b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commitB, Supersedes: r2, Title: "W1"})
	if aps := Compute(b.entries).Approvals(0); len(aps) != 0 {
		t.Fatalf("the round the work went on from: %+v", aps[0])
	}
	// Kept open: W2 approved at A, then a new request on W2 at B that
	// supersedes nothing.
	w2 := b.request("W2", commitA, 0)
	b.reply(w2, "APPROVE", commitA)
	b.decide(w2, "approve")
	b.request("W2", commitB, 0)
	if aps := Compute(b.entries).Approvals(0); len(aps) != 1 || aps[0].Item != "W2" || aps[0].Commit != commitA {
		t.Fatalf("W2 kept open: %+v", aps)
	}
}

// reviewer-2's #485, point 1: a round cut after its first request, that
// request denied, the round completed late and its second request approved:
// one ask, with a denial in it, so no approval.
func TestH34p1ARoundCompletedLateKeepsItsDenial(t *testing.T) {
	b := &builder{}
	panel := []string{"reviewer", "reviewer-2"}
	first := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA, Round: 1, Panel: panel, Title: "W1"})
	b.reply(first, "APPROVE", commitA)
	b.decide(first, "deny")
	second := b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commitA, Round: 1, Panel: panel, Title: "W1"})
	b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer-2", To: "implementer", Re: second, Verdict: "APPROVE", Commit: commitA})
	b.decide(second, "approve")
	if aps := Compute(b.entries).Approvals(0); len(aps) != 0 {
		t.Fatalf("a round with a denial listed: %+v", aps[0])
	}
}

// reviewer-2's #485, point 3: work at A, superseded by work at B, which is
// superseded by work back at A, approved: the fresh ask at A is listed.
func TestH34p1WorkBackAtACommitIsAskedAnew(t *testing.T) {
	b := &builder{}
	a1 := b.request("W1", commitA, 0)
	b1 := b.request("W1", commitB, a1)
	a2 := b.request("W1", commitA, b1)
	b.reply(a2, "APPROVE", commitA)
	b.decide(a2, "approve")
	aps := Compute(b.entries).Approvals(0)
	if len(aps) != 1 || aps[0].Commit != commitA || aps[0].Requests[0].Entry.ID != a2 {
		t.Fatalf("approvals: %+v", aps)
	}
}

// reviewer-2's #485, point 2: the go-ahead's fyi, read after the operator
// held one of the items it approved, says so of that one, and still calls
// the others the operator's approve.
func TestH34p1TheGoAheadsFyiSaysWhatWasHeld(t *testing.T) {
	b := &builder{}
	w1 := b.request("W1", commitA, 0)
	b.reply(w1, "APPROVE", commitA)
	w2 := b.add(record.Entry{Kind: "request", Item: "W2", From: "implementer", To: "reviewer", Commit: commitB, Title: "W2"})
	b.add(record.Entry{Kind: "reply", Item: "W2", From: "reviewer", To: "implementer", Re: w2, Verdict: "APPROVE", Commit: commitB})
	away := b.add(record.Entry{Kind: "away", From: Operator, Body: "go"})
	fyi := b.add(record.Entry{Kind: "fyi", From: Operator, To: "implementer", Re: away, Session: "s-1", Body: "go"})
	hold := b.add(record.Entry{Kind: "decision", Item: "W1", From: Operator, To: "implementer", Re: w1, Decision: "deny"})
	s := Compute(b.entries)
	note := s.GoAheadNote(b.entries[fyi-1])
	if !strings.Contains(note, "W2 approved under the operator's go-ahead") || !strings.Contains(note, fmt.Sprintf("W1, then held by the operator (#%d)", hold)) || strings.Contains(note, "W1, W2") {
		t.Fatalf("the fyi's note: %q", note)
	}
	// What told the requester of W2 is that fyi, to the session it named.
	for _, ap := range s.Approvals(0) {
		if ap.Item == "W2" && (ap.Told != fyi || ap.ToldSession != "s-1") {
			t.Fatalf("W2 told by #%d to %q", ap.Told, ap.ToldSession)
		}
	}
}

// story builds a record for TestH34p1WhichApprovalsCount.
type story struct{ builder }

const (
	commitC = "cccccccccccccccccccccccccccccccccccccccc"
	commitD = "dddddddddddddddddddddddddddddddddddddddd"
)

// ask posts a request on item at commit to a reviewer; round and panel make
// it a round's member; it supersedes the request given, if any.
func (b *story) ask(item, commit, to string, round int64, panel []string, supersedes int64) int64 {
	kind := "request"
	if round != 0 {
		kind = "round"
	}
	return b.add(record.Entry{Kind: kind, Item: item, From: "implementer", To: to, Commit: commit, Round: round, Panel: panel, Supersedes: supersedes, Title: item})
}

// says is the reviewer's verdict on a request.
func (b *story) says(req int64, verdict string) {
	r := b.entries[req-1]
	b.add(record.Entry{Kind: "reply", Item: r.Item, From: r.To, To: r.From, Re: req, Verdict: verdict, Commit: r.Commit})
}

func (b *story) decides(req int64, decision string) {
	r := b.entries[req-1]
	b.add(record.Entry{Kind: "decision", Item: r.Item, From: Operator, To: r.From, Re: req, Decision: decision})
}

func (b *story) away() { b.add(record.Entry{Kind: "away", From: Operator, Body: "go"}) }

// The cases of "which approvals count", as a table (the reviewer's note in
// #508): each row a story, and what is listed to merge, as item@commit and
// its reviewers.
func TestH34p1WhichApprovalsCount(t *testing.T) {
	both := []string{"reviewer", "reviewer-2"}
	rows := []struct {
		name string
		tell func(b *story)
		want string
	}{
		{"a round, both approve under the go-ahead", func(b *story) {
			b.away()
			r1 := b.ask("W1", commitC, "reviewer", 2, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 2, both, 0)
			b.says(r1, "APPROVE")
			b.says(r2, "APPROVE")
		}, "W1@c:reviewer,reviewer-2"},
		{"a round, one approved in a pop-up while the other asks for changes", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r1, "APPROVE")
			b.says(r2, "CHANGES")
			b.decides(r1, "approve")
		}, ""},
		{"tonight: the next post replaces the request with changes", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r1, "APPROVE")
			b.says(r2, "CHANGES")
			b.decides(r1, "approve")
			b.ask("W1", commitD, "reviewer-2", 0, nil, r2)
		}, ""},
		{"separate posts, the one with changes first", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			p2 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p1, "CHANGES")
			b.says(p2, "APPROVE")
			b.decides(p2, "approve")
			b.ask("W1", commitD, "reviewer-2", 0, nil, p1)
		}, ""},
		{"separate posts, the one with changes second", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			p2 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			b.says(p2, "CHANGES")
			b.says(p1, "APPROVE")
			b.decides(p1, "approve")
			b.ask("W1", commitD, "reviewer-2", 0, nil, p2)
		}, ""},
		{"separate posts, both approved, the first then denied", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			p2 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.says(p2, "APPROVE")
			b.decides(p2, "approve")
			b.decides(p1, "deny")
		}, ""},
		{"separate posts, the first denied, then the second approved", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			p2 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.says(p2, "APPROVE")
			b.decides(p1, "deny")
			b.decides(p2, "approve")
		}, ""},
		{"separate posts, approved together under the go-ahead", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			p2 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			b.away()
			b.says(p1, "APPROVE")
			b.says(p2, "APPROVE")
		}, "W1@c:reviewer,reviewer-2"},
		{"a re-ask after a recorded merge", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.decides(p1, "approve")
			b.add(record.Entry{Kind: "merged", Item: "W1", From: "implementer", Re: p1, Commit: commitD})
			p2 := b.ask("W1", commitC, "reviewer-2", 0, nil, 0)
			b.says(p2, "APPROVE")
			b.decides(p2, "approve")
		}, "W1@c:reviewer-2"},
		{"A, then B, then back to A", func(b *story) {
			a1 := b.ask("W1", commitA, "reviewer", 0, nil, 0)
			b1 := b.ask("W1", commitB, "reviewer", 0, nil, a1)
			a2 := b.ask("W1", commitA, "reviewer", 0, nil, b1)
			b.says(a2, "APPROVE")
			b.decides(a2, "approve")
		}, "W1@a:reviewer"},
		{"a cut round completed late, its first denied", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			b.says(r1, "APPROVE")
			b.decides(r1, "deny")
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r2, "APPROVE")
			b.decides(r2, "approve")
		}, ""},
		{"a cut round never completed, its one request approved", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			b.says(r1, "APPROVE")
			b.decides(r1, "approve")
		}, ""},
		{"held, then asked again and approved", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.decides(p1, "approve")
			b.decides(p1, "deny") // the hold
			p2 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p2, "APPROVE")
			b.decides(p2, "approve")
		}, "W1@c:reviewer"},
		{"denied, then asked again and approved", func(b *story) {
			p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.decides(p1, "deny")
			p2 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
			b.says(p2, "APPROVE")
			b.decides(p2, "approve")
		}, "W1@c:reviewer"},
		{"a round with a denial, and a third reviewer asked alone while it is open", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r1, "APPROVE")
			b.decides(r1, "deny")
			p3 := b.ask("W1", commitC, "reviewer-3", 0, nil, 0)
			b.says(r2, "APPROVE")
			b.decides(r2, "approve")
			b.says(p3, "APPROVE")
			b.decides(p3, "approve")
		}, ""},
		{"a round with a denial, and no one else asked", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r1, "APPROVE")
			b.decides(r1, "deny")
			b.says(r2, "APPROVE")
			b.decides(r2, "approve")
		}, ""},
		{"a round ended whole, then a reviewer asked alone", func(b *story) {
			r1 := b.ask("W1", commitC, "reviewer", 1, both, 0)
			r2 := b.ask("W1", commitC, "reviewer-2", 1, both, 0)
			b.says(r1, "APPROVE")
			b.decides(r1, "deny")
			b.says(r2, "APPROVE")
			b.decides(r2, "deny")
			p3 := b.ask("W1", commitC, "reviewer-3", 0, nil, 0)
			b.says(p3, "APPROVE")
			b.decides(p3, "approve")
		}, "W1@c:reviewer-3"},
		{"kept open beside newer work on the item", func(b *story) {
			p1 := b.ask("W2", commitA, "reviewer", 0, nil, 0)
			b.says(p1, "APPROVE")
			b.decides(p1, "approve")
			b.ask("W2", commitB, "reviewer", 0, nil, 0)
		}, "W2@a:reviewer"},
	}
	for _, row := range rows {
		b := &story{}
		row.tell(b)
		var got []string
		for _, ap := range Compute(b.entries).Approvals(0) {
			got = append(got, fmt.Sprintf("%s@%.1s:%s", ap.Item, ap.Commit, strings.Join(ap.Reviewers(), ",")))
		}
		if strings.Join(got, " ") != row.want {
			t.Errorf("%s: listed %q, want %q", row.name, strings.Join(got, " "), row.want)
		}
	}
}

// reviewer-2's #506, point 2: two requesters' work on one item under one
// go-ahead, one of them held: the fyi's note holds that one only.
func TestH34p1TheFyiHoldsOneRequestersWork(t *testing.T) {
	b := &story{}
	imp := b.ask("W1", commitC, "reviewer", 0, nil, 0)
	other := b.add(record.Entry{Kind: "request", Item: "W1", From: "reviewer-2", To: "reviewer", Commit: commitC, Title: "W1"})
	b.says(imp, "APPROVE")
	b.says(other, "APPROVE")
	away := b.add(record.Entry{Kind: "away", From: Operator, Body: "go"})
	fyi := b.add(record.Entry{Kind: "fyi", From: Operator, To: "reviewer-2", Re: away, Session: "s-2", Body: "go"})
	hold := b.add(record.Entry{Kind: "decision", Item: "W1", From: Operator, To: "implementer", Re: imp, Decision: "deny"})
	note := Compute(b.entries).GoAheadNote(b.entries[fyi-1])
	if !strings.Contains(note, "W1 (reviewer-2's) approved under the operator's go-ahead") || !strings.Contains(note, fmt.Sprintf("W1 (implementer's), then held by the operator (#%d)", hold)) {
		t.Fatalf("the fyi's note: %q", note)
	}
}

// reviewer-2's and the reviewer's #528, #529, point 2: held work asked again
// as a round, one request approved and the other denied, isn't approved
// anew, so the hold stands, and its warning if the commit reaches main.
func TestH34p1APartialReapprovalLeavesTheHold(t *testing.T) {
	b := &story{}
	both := []string{"reviewer", "reviewer-2"}
	p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
	b.says(p1, "APPROVE")
	b.decides(p1, "approve")
	b.decides(p1, "deny") // the hold
	r1 := b.ask("W1", commitC, "reviewer", 9, both, 0)
	r2 := b.ask("W1", commitC, "reviewer-2", 9, both, 0)
	// A round's id is its first request's.
	b.entries[r1-1].Round, b.entries[r2-1].Round = r1, r1
	b.says(r1, "APPROVE")
	b.decides(r1, "approve")
	b.says(r2, "APPROVE")
	b.decides(r2, "deny")
	s := Compute(b.entries)
	if len(s.Approvals(0)) != 0 {
		t.Fatal("listed to merge")
	}
	if hs := s.Holds(0); len(hs) != 1 || hs[0].Hold == nil {
		t.Fatalf("the hold: %+v", hs)
	}
}

// reviewer-2's #542 and the reviewer's #543: a hold lifted by a whole
// re-approval stays lifted, a follow-up review opened at the commit later
// notwithstanding (V); and when the re-approval is held in its turn, only
// that second hold stands.
func TestH34p1AHoldLiftedStaysLifted(t *testing.T) {
	t.Run("a follow-up opened after the re-approval", func(t *testing.T) {
		b := &story{}
		p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
		b.says(p1, "APPROVE")
		b.decides(p1, "approve")
		b.decides(p1, "deny") // the hold
		p2 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
		b.says(p2, "APPROVE")
		b.decides(p2, "approve")
		b.ask("W1", commitC, "reviewer-2", 0, nil, 0) // the follow-up, open
		if hs := Compute(b.entries).Holds(0); len(hs) != 0 {
			t.Fatalf("the lifted hold came back: %+v", hs[0].Hold)
		}
	})
	t.Run("the re-approval held in its turn", func(t *testing.T) {
		b := &story{}
		p1 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
		b.says(p1, "APPROVE")
		b.decides(p1, "approve")
		b.decides(p1, "deny")
		p2 := b.ask("W1", commitC, "reviewer", 0, nil, 0)
		b.says(p2, "APPROVE")
		b.decides(p2, "approve")
		b.decides(p2, "deny")
		second := int64(len(b.entries))
		hs := Compute(b.entries).Holds(0)
		if len(hs) != 1 || hs[0].Hold.ID != second {
			t.Fatalf("holds standing: %d, want only #%d", len(hs), second)
		}
	})
}

// reviewer-2's #521, point 3: an fyi naming the session a successor went on
// from, by /background, waits for the successor, as delivery takes it; one
// naming a session with no tie to it waits for no one.
func TestH34p2AnFyiForTheSessionWentOnFromWaits(t *testing.T) {
	b := &builder{}
	fyi := b.add(record.Entry{Kind: "fyi", From: Operator, To: "reviewer", Session: "s-old", Body: "go"})
	s := Compute(b.entries)
	if got := s.WaitsFor("reviewer", 0, "s-new", []string{"s-old"}, time.Now()); got != fyi {
		t.Fatalf("for the successor: %d, want #%d", got, fyi)
	}
	if got := s.WaitsFor("reviewer", 0, "s-other", nil, time.Now()); got != 0 {
		t.Fatalf("for a session with no tie: %d, want 0", got)
	}
}

// Rows 110 and 111: an APPROVE held back from its requester is no work
// waiting for it, for a block's alert, until its hold ends.
func TestH34p3AHeldApproveIsNoWorkWaiting(t *testing.T) {
	b := &builder{}
	r1 := b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: "c", Title: "W1"})
	b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer-2", Commit: "c", Title: "W1"})
	at := time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)
	ok := b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: r1, Verdict: "APPROVE", Commit: "c", TS: at.Format(time.RFC3339)})
	s := Compute(b.entries)
	if got := s.WaitsFor("implementer", 0, "", nil, at.Add(time.Minute)); got != 0 {
		t.Fatalf("while held: %d, want 0", got)
	}
	if got := s.WaitsFor("implementer", 0, "", nil, at.Add(HoldFor)); got != ok {
		t.Fatalf("once its hold ends: %d, want #%d", got, ok)
	}
}

// reviewer-2's #603: two APPROVEs held ten minutes apart, a third review
// open. Nothing waits before the first's 30 minutes; then both go, as the
// listener gives them in one wake, so the newer names the work at the
// first's deadline and at the second's alike.
func TestH34p3HeldApprovesWaitAsTheListenerGivesThem(t *testing.T) {
	b := &builder{}
	r1 := b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: "c", Title: "W1"})
	r2 := b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer-2", Commit: "c", Title: "W1"})
	b.add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer-3", Commit: "c", Title: "W1"})
	at := time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)
	b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: r1, Verdict: "APPROVE", Commit: "c", TS: at.Format(time.RFC3339)})
	newer := b.add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer-2", To: "implementer", Re: r2, Verdict: "APPROVE", Commit: "c", TS: at.Add(10 * time.Minute).Format(time.RFC3339)})
	s := Compute(b.entries)
	for _, c := range []struct {
		after time.Duration
		want  int64
	}{{15 * time.Minute, 0}, {HoldFor, newer}, {HoldFor + 10*time.Minute, newer}} {
		if got := s.WaitsFor("implementer", 0, "", nil, at.Add(c.after)); got != c.want {
			t.Fatalf("at %s: %d, want %d", c.after, got, c.want)
		}
	}
}

// reviewer-3's #709: a broken round is the operator's to sort out, so its
// quiet alert doesn't end with why its requester's work waits.
func TestH34p5ABrokenRoundsAlertGivesNoRequesterWhy(t *testing.T) {
	panel := []string{"reviewer", "reviewer-2"}
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	b := &builder{}
	b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA, Round: 2, Panel: panel, Title: "W1", TS: old})
	b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commitB, Round: 2, Panel: panel, Title: "W1", TS: old})
	s := Compute(b.entries)
	s.SetDelivery(map[string]Delivery{"implementer": {Why: ", and nothing is listening for it (never listened)"}}, 0)
	var got []string
	for _, al := range s.Alerts(time.Now(), time.Minute) {
		if strings.HasPrefix(al.Key, "round-") {
			got = append(got, al.Text)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0], "only you can sort it out (") || strings.Contains(got[0], "nothing is listening") {
		t.Fatalf("the broken round's alert: %q", got)
	}
}

// H35 part 2 (row 116): Waiting is what WaitsFor counts, oldest first, a
// request owed by its latest entry, which starts its clock.
func TestH35p2WaitingIsWhatWaitsForCounts(t *testing.T) {
	b := &builder{}
	req := b.request("W1", commitA, 0)
	claim := b.claim(req)
	note := b.note("implementer", "reviewer", 0)
	s := Compute(b.entries)
	now := time.Now()
	w := s.Waiting("reviewer", 0, "", nil, now)
	var ids []int64
	for _, e := range w {
		ids = append(ids, e.ID)
	}
	if fmt.Sprint(ids) != fmt.Sprint([]int64{req, claim, note}) {
		t.Fatalf("waiting %v, want the request, its claim and the note", ids)
	}
	if last := s.WaitsFor("reviewer", 0, "", nil, now); last != note {
		t.Fatalf("WaitsFor %d, want %d", last, note)
	}
	if w := s.Waiting("reviewer", note, "", nil, now); len(w) != 1 || w[0].ID != claim {
		t.Fatalf("past the cursor: %v, want only the claimed request's claim", w)
	}
}

// reviewer-2's and reviewer-3's #853, #855: OwedBy is what a seat owes now.
// A review answered is the requester's, not the reviewer's, though its
// request is past the reviewer's cursor; a claimed review is timed from its
// claim.
func TestH35p2OwedByIsWhatTheSeatOwesNow(t *testing.T) {
	b := &builder{}
	req := b.request("W1", commitA, 0)
	claim := b.claim(req)
	if w := Compute(b.entries).OwedBy("reviewer", 0); len(w) != 1 || w[0].ID != claim {
		t.Fatalf("claimed: %v, want only the claim", w)
	}
	reply := b.reply(req, "CHANGES", commitA)
	s := Compute(b.entries)
	if w := s.OwedBy("reviewer", 0); len(w) != 0 {
		t.Fatalf("answered, the reviewer owes %v", w)
	}
	if w := s.OwedBy("implementer", 0); len(w) != 1 || w[0].ID != reply {
		t.Fatalf("the implementer owes %v, want the CHANGES", w)
	}
}

// reviewer-2's #862: a broken round is owed by no one, neither its
// requester nor any reviewer it went to.
func TestH35p2ABrokenRoundIsOwedByNoOne(t *testing.T) {
	panel := []string{"reviewer", "reviewer-2"}
	b := &builder{}
	b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer", Commit: commitA, Round: 1, Panel: panel, Title: "W1"})
	b.add(record.Entry{Kind: "round", Item: "W1", From: "implementer", To: "reviewer-2", Commit: commitB, Round: 1, Panel: panel, Title: "W1"})
	s := Compute(b.entries)
	if len(s.Rounds) == 0 || !s.Rounds[0].Broken() {
		t.Fatal("the round isn't broken")
	}
	for _, role := range []string{"implementer", "reviewer", "reviewer-2"} {
		if w := s.OwedBy(role, 0); len(w) != 0 {
			t.Fatalf("%s owes %v of a broken round", role, w)
		}
	}
}

// The reviewer's #865: a note the role answered with re is done with,
// though past its cursor.
func TestH35p2ANoteAnsweredIsntOwed(t *testing.T) {
	b := &builder{}
	n := b.note("implementer", "reviewer", 0)
	if w := Compute(b.entries).OwedBy("reviewer", 0); len(w) != 1 || w[0].ID != n {
		t.Fatalf("a note unanswered: %v", w)
	}
	b.note("reviewer", "implementer", n)
	if w := Compute(b.entries).OwedBy("reviewer", 0); len(w) != 0 {
		t.Fatalf("a note answered: %v", w)
	}
}

// reviewer-3's #877: a note that answers a CHANGES is its request's: owed
// while the review is, as BeingReviewed, and not once the request is done.
func TestH35p2ANoteAnsweringAVerdictIsItsRequests(t *testing.T) {
	b := &builder{}
	req := b.request("W1", commitA, 0)
	reply := b.reply(req, "CHANGES", commitA)
	answer := b.note("implementer", "reviewer", reply)
	if w := Compute(b.entries).OwedBy("reviewer", 0); len(w) != 1 || w[0].ID != answer {
		t.Fatalf("the answer, while the review is owed: %v", w)
	}
	b.reply(req, "APPROVE", commitA)
	if w := Compute(b.entries).OwedBy("reviewer", 0); len(w) != 0 {
		t.Fatalf("the reviewer owes %v once it approved", w)
	}
}
