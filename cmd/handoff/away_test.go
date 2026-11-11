package main

import (
	"strings"
	"testing"
)

// H28 (row 96): the operator goes away with a go-ahead, approving in
// advance each item every reviewer asked approves at one commit, and comes
// back.

// toBoth posts item at commit to reviewer and reviewer-2, and returns the
// two requests' ids.
func (r *repo) toBoth(t *testing.T, item, commit string) (string, string) {
	t.Helper()
	one := r.request(t, item, commit)
	two := idOf(t, r.ok(t, "implementer", "post", "--item", item, "--to", "reviewer-2", "--commit", commit, "--title", item+" work", "--body", r.body(t, "please review "+item)))
	return one, two
}

func (r *repo) approve(t *testing.T, notify []string, reviewer, req, commit string) string {
	t.Helper()
	res := r.run(t, reviewer, "", notify, "reply", req, "--verdict", "APPROVE", "--commit", commit, "--body", r.body(t, "ok"))
	if res.code != 0 {
		t.Fatalf("%s approves #%s: exit %d\n%s", reviewer, req, res.code, res.err)
	}
	return idOf(t, res.out)
}

func TestH28AwayApprovesOnceEveryReviewerApproves(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
	r.as(t, claude("s-rev-2", cli(t)), "role", "reviewer-2")

	out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve.")
	if err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	for _, want := range []string{"away (#1): your go-ahead stands until you run handoff back\r\n",
		goAheadRule + " handoff doesn't read your words: to tell the agents something without approving anything, use handoff fyi.\r\n",
		"told implementer, reviewer and reviewer-2 (#2, #3, #4)\r\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("away said:\n%s\nwant %q", out, want)
		}
	}
	es := r.snap(t).Entries
	if e := es[0]; e.Kind != "away" || e.From != "operator" || e.To != "" || e.Body != "Merge when the reviewers approve." {
		t.Fatalf("the away entry: %+v", e)
	}
	if e := es[1]; e.Kind != "fyi" || e.To != "implementer" || e.Session != "s-impl" || e.Re != 1 ||
		e.Body != "Merge when the reviewers approve.\n\nThis is the operator's go-ahead while away (#1). "+goAheadRule {
		t.Fatalf("the implementer's fyi: %+v", e)
	}

	one, two := r.toBoth(t, "W1", r.commitA)
	// The operator hears that the go-ahead is recorded, then of W1 once
	// both have approved, with who approved; nothing asks a decision.
	r.approve(t, notify, "reviewer", one, r.commitA)
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "Your go-ahead is recorded (#1): Merge when the reviewers approve. Until you run handoff back, items are approved without asking you.") || strings.Contains(got, "W1:") {
		t.Fatalf("away, after one approval of two, the operator was told:\n%s", got)
	}
	last := r.approve(t, notify, "reviewer-2", two, r.commitA)
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: approved under your go-ahead #1, by reviewer and reviewer-2 (#"+one+", #"+two+").") || strings.Contains(got, "Your decision") {
		t.Fatalf("W1 approved under the go-ahead; the operator was told:\n%s", got)
	}

	// The implementer's listener brings the APPROVE saying what it means.
	res := r.run(t, "", hookIn("s-impl", "Stop"), claude("s-impl", cli(t)), "hook", "listen", "--timeout", "1s")
	if res.code != 2 || !strings.Contains(res.err, "\n  W1 is approved under the operator's go-ahead #1: that is the operator's approve\n") {
		t.Fatalf("the implementer's listener: exit %d\n%s", res.code, res.err)
	}
	if show := r.ok(t, "", "show", last); !strings.Contains(show, "\napproved under: #1, the operator's go-ahead while away: that is the operator's approve\n") {
		t.Fatalf("show %s:\n%s", last, show)
	}
	if home := r.ok(t, ""); !strings.HasPrefix(home, "You're away (#1, since ") || !strings.Contains(home, "), with this go-ahead: Merge when the reviewers approve.\nRun handoff back to end it.\n\nNothing needs your decision.\n") {
		t.Fatalf("handoff:\n%s", home)
	}
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--approve"); err == nil || !strings.Contains(out, "nothing on W1 awaits the operator") {
		t.Fatalf("decide W1: %v\n%s", err, out)
	}
	b := r.ok(t, "implementer", "brief")
	for _, want := range []string{
		"\nThe operator is away (#1, since ",
		"). Their go-ahead: \"Merge when the reviewers approve.\" Read it all with `",
		"handoff show 1`. Approved under it so far: W1 (#" + one + ", #" + two + ").\n",
		"\n- While the operator is away with a go-ahead, the APPROVE that completes an item arrives saying it \"is approved under the operator's go-ahead\". That is the operator's approve: act on it as on a decision to approve. It needs every live request on the item approved at one commit, so after a fix for one reviewer, ask each reviewer again.\n",
		"\n- A decision from the operator: act on it. Merge only on approve, or on an approval under the operator's go-ahead.\n",
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("the implementer's brief lacks %q:\n%s", want, b)
		}
	}
}

// After CHANGES from one reviewer, a fix sent only to that reviewer leaves
// approvals at two commits. The go-ahead doesn't cover them, and the
// operator is told why.
func TestH28AwayNeedsOneCommit(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	// With no session, the go-ahead is still recorded; a session's brief
	// shows it when it starts.
	if out, err := r.atTerminal(t, "", nil, "away", "Merge when approved."); err != nil || !strings.Contains(out, "no agent has a session now; each session's brief shows your go-ahead when it starts\r\n") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	if es := r.snap(t).Entries; len(es) != 1 || es[0].Kind != "away" {
		t.Fatalf("away with no session wrote %d entries", len(es))
	}
	one, two := r.toBoth(t, "W1", r.commitA)
	r.approve(t, notify, "reviewer", one, r.commitA)
	r.ok(t, "reviewer-2", "reply", two, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix it"))
	three := idOf(t, r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer-2", "--commit", r.commitB, "--title", "W1 fixed", "--supersedes", two, "--body", r.body(t, "fixed")))
	r.approve(t, notify, "reviewer-2", three, r.commitB)
	got := strings.Join(sentTo(t, sent), "\n")
	if !strings.Contains(got, "but the approvals on W1 name different commits, so your go-ahead doesn't cover it. Your decision.") {
		t.Fatalf("the operator was told:\n%s", got)
	}
	if home := r.ok(t, ""); strings.Contains(home, "Nothing needs your decision.") {
		t.Fatalf("handoff:\n%s", home)
	}
}

// What is approved already, the go-ahead approves at once; the operator
// confirms that first, and a no writes nothing.
func TestH28AwayConfirmsWhatItApprovesNow(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	req := r.request(t, "W1", r.commitA)
	r.approve(t, nil, "reviewer", req, r.commitA)
	n := len(r.snap(t).Entries)
	ask := "Your go-ahead approves now: W1 (#" + req + "). Go ahead? [y/N] "
	out, err := r.atTerminal(t, "", []step{{ask, "n"}}, "away", "Merge when approved.")
	if err == nil || !strings.Contains(out, "not confirmed; nothing written") || len(r.snap(t).Entries) != n {
		t.Fatalf("away, not confirmed: %v\n%s", err, out)
	}
	out, err = r.atTerminalWith(t, notify, []step{{ask, "y"}}, "away", "Merge when approved.")
	if err != nil || !strings.Contains(out, "approved under it now: W1 (#"+req+")\r\n") {
		t.Fatalf("away, confirmed: %v\n%s", err, out)
	}
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: approved under your go-ahead #3, by reviewer (#"+req+").") {
		t.Fatalf("an item the away entry approves; the operator was told:\n%s", got)
	}
	es := r.snap(t).Entries
	if fyi := es[len(es)-1]; fyi.Kind != "fyi" || !strings.HasSuffix(fyi.Body, "Approved under it now: W1 (#"+req+").") {
		t.Fatalf("the fyi: %+v", fyi)
	}
	// The implementer's listener brings the fyi saying what it approved,
	// though the APPROVE came before the go-ahead.
	res := r.run(t, "", hookIn("s-impl", "Stop"), claude("s-impl", cli(t)), "hook", "listen", "--timeout", "1s")
	if res.code != 2 || !strings.Contains(res.err, "\n  W1 approved under the operator's go-ahead #") || !strings.Contains(res.err, " as it was given: that is the operator's approve\n") {
		t.Fatalf("the implementer's listener: exit %d\n%s", res.code, res.err)
	}
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--approve"); err == nil || !strings.Contains(out, "nothing on W1 awaits the operator") {
		t.Fatalf("decide W1: %v\n%s", err, out)
	}
}

// Back ends the go-ahead and says what it approved. Later, an approval
// waits for the operator as before.
func TestH28BackEndsTheGoAhead(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	r.refused(t, "", "back needs a terminal on stdin", "back")
	if out, err := r.atTerminal(t, "", nil, "back"); err == nil || !strings.Contains(out, "you aren't away; nothing written") {
		t.Fatalf("back, not away: %v\n%s", err, out)
	}
	if out, err := r.atTerminal(t, "", nil, "away", "Merge when approved."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	w1 := r.request(t, "W1", r.commitA)
	r.approve(t, notify, "reviewer", w1, r.commitA)
	out, err := r.atTerminal(t, "", nil, "back")
	if err != nil {
		t.Fatalf("back: %v\n%s", err, out)
	}
	for _, want := range []string{"back (#5): the go-ahead of #1 has ended\r\n", "approved under it while you were away: W1 (#" + w1 + ")\r\n", "told implementer (#6)\r\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("back said:\n%s\nwant %q", out, want)
		}
	}
	es := r.snap(t).Entries
	if e := es[len(es)-1]; e.Kind != "fyi" || e.Body != "The operator is back: the go-ahead of #1 has ended. On an APPROVE, wait for the operator's decision again." {
		t.Fatalf("back's fyi: %+v", e)
	}
	w2 := r.request(t, "W2", r.commitA)
	r.approve(t, notify, "reviewer", w2, r.commitA)
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W2: reviewer approved #"+w2+". Your decision.") {
		t.Fatalf("after back, the operator was told:\n%s", got)
	}
	if home := r.ok(t, ""); strings.Contains(home, "You're away") {
		t.Fatalf("handoff after back:\n%s", home)
	}
}

// Only the operator goes away and comes back, at a terminal, outside any
// agent's session. A refusal writes nothing.
func TestH28AwayAndBackAreTheOperatorsAlone(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	impl := claude("s-impl", cli(t))
	r.as(t, impl, "role", "implementer")
	for _, cmd := range []string{"away", "back"} {
		args := []string{cmd}
		if cmd == "away" {
			args = append(args, "Merge when approved.")
		}
		r.refused(t, "", cmd+" needs a terminal on stdin", args...)
		r.refused(t, "implementer", cmd+" is for the operator; the caller's role is implementer", args...)
		r.refusedIn(t, impl, cmd+" is the operator's, in a terminal of the operator's own, never in an agent's session", args...)
	}
	r.refused(t, "", "give the message, in quotes, such as handoff away \"Merge when the reviewers approve.\"", "away")
	r.refused(t, "", "away takes no flags, and a message can't start with -", "away", "--as", "operator", "x")
	r.refused(t, "", "back takes nothing: run handoff back", "back", "now")
	if n := len(r.snap(t).Entries); n != 0 {
		t.Fatalf("%d entries after refusals, want none", n)
	}
	usage := r.h(t, "", "--help").err
	for _, want := range []string{"\n  away    \"<go-ahead>\"", "\n  back    ", "decide, fyi, away, back, and handoff on its own, need no role"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("the usage lacks %q:\n%s", want, usage)
		}
	}
}

// The go-ahead applies one rule, whatever its words say: with both
// reviewers in session, an item sent to one is approved by that one
// approval, even under "Merge when both reviewers approve." (the
// reviewer's K1 on H28). away says so as it is given.
func TestH28OneApprovalIsEnoughForAnItemSentToOne(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
	r.as(t, claude("s-rev-2", cli(t)), "role", "reviewer-2")
	out, err := r.atTerminal(t, "", nil, "away", "Merge when both reviewers approve.")
	if err != nil || !strings.Contains(out, "one approval is enough for an item sent to one reviewer. handoff doesn't read your words") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	req := r.request(t, "W1", r.commitA)
	r.approve(t, notify, "reviewer", req, r.commitA)
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: approved under your go-ahead #1, by reviewer (#"+req+").") || strings.Contains(got, "Your decision") {
		t.Fatalf("the operator was told:\n%s", got)
	}
	if out, err := r.atTerminal(t, "", nil, "back"); err != nil || !strings.Contains(out, "approved under it while you were away: W1 (#"+req+")\r\n") {
		t.Fatalf("back: %v\n%s", err, out)
	}
}
