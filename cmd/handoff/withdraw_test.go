package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

// H38 (row 118): the requester withdraws a request no one need answer now,
// and the spare taking the implementer seat is told to keep the old
// session's changes.

func TestH38TheRequesterWithdrawsARequest(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	r.ok(t, "reviewer", "claim", req)
	r.refused(t, "reviewer", "#"+req+" was asked for by implementer; only it can withdraw it", "withdraw", req)
	res := r.h(t, "implementer", "withdraw", req, "--note", "dropped by the operator")
	w := idOf(t, res.out)
	if want := "posted #" + w + " withdraw W1 implementer → reviewer: withdrew #" + req + ": dropped by the operator\n"; res.code != 0 || res.out != want || res.err != "" {
		t.Fatalf("withdraw: exit %d %q %q\nwant %q", res.code, res.out, res.err, want)
	}
	if out := r.ok(t, "implementer", "withdraw", req); !strings.HasPrefix(out, "exists #"+w+" withdraw") {
		t.Fatalf("withdraw again: %q", out)
	}
	if out := r.ok(t, "reviewer", "inbox"); strings.Contains(out, "#"+req+" request") {
		t.Fatalf("the reviewer's inbox:\n%s", out)
	}
	if out := r.ok(t, "reviewer", "show", req); !strings.Contains(out, "state: withdrawn by implementer (#"+w+")\n") {
		t.Fatalf("show:\n%s", out)
	}
	// The reviewer's listener gives it the withdrawal.
	if res := r.h(t, "reviewer", "wait", "--timeout", "2s"); res.code != 0 || !strings.Contains(res.out, "#"+w+" withdraw W1 implementer → reviewer: withdrew #"+req+": dropped by the operator") {
		t.Fatalf("the reviewer's wait: exit %d %q", res.code, res.out)
	}
	if out := r.ok(t, "reviewer", "brief"); !strings.Contains(out, "- A withdraw of a request to you: its requester no longer wants it answered. Stop reviewing it; nothing is owed on it.\n") {
		t.Fatalf("the reviewer's brief:\n%s", out)
	}
	if out := r.ok(t, "implementer", "brief"); !strings.Contains(out, " withdraw --as implementer <request>`. A request withdrawn keeps the go-ahead off its item until that reviewer is asked again: the operator decides it.\n") {
		t.Fatalf("the implementer's brief:\n%s", out)
	}
	// A verdict after it is refused, and nothing is written: a listener
	// older than H38, still running, would take a late APPROVE for one
	// (reviewer-2, #906).
	before := len(r.snap(t).Entries)
	for _, v := range []string{"APPROVE", "CHANGES"} {
		r.refused(t, "reviewer", "request #"+req+" was withdrawn by implementer (#"+w+"), so nothing is owed on it; nothing written",
			"reply", req, "--verdict", v, "--commit", r.commitA, "--body", r.body(t, v))
	}
	if n := len(r.snap(t).Entries); n != before {
		t.Fatalf("a refused reply wrote: %d entries, then %d", before, n)
	}
	if out := r.ok(t, "implementer", "inbox"); strings.Contains(out, "#"+req+" request") {
		t.Fatalf("the implementer's inbox:\n%s", out)
	}
	// Nothing to supersede; a new request needs none.
	r.refused(t, "implementer", "#"+req+" was withdrawn (#"+w+"), so there is nothing to supersede; post without --supersedes",
		"post", "--item", "W1", "--to", "reviewer", "--commit", r.commitB, "--title", "W1 again", "--body", r.body(t, "again"), "--supersedes", req)
	r.request(t, "W1", r.commitB)

	// Answered with CHANGES, it leaves the requester's inbox.
	changes := r.request(t, "W2", r.commitA)
	r.ok(t, "reviewer", "reply", changes, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix"))
	r.ok(t, "implementer", "withdraw", changes)
	if out := r.ok(t, "implementer", "inbox"); strings.Contains(out, "#"+changes+" request") {
		t.Fatalf("the implementer's inbox after withdrawing a CHANGES:\n%s", out)
	}
	// Approved, it is the operator's to decide.
	approved := r.request(t, "W3", r.commitA)
	r.ok(t, "reviewer", "reply", approved, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	r.refused(t, "implementer", "#"+approved+" is approved and waits for the operator's decision, so it is theirs to end now; nothing written", "withdraw", approved)
}

// Withdrawing one reviewer's request while another's on the item is still
// live says that the go-ahead won't approve the item.
func TestH38WithdrawSaysTheOperatorDecides(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	one, two := r.toBoth(t, "W1", r.commitA)
	res := r.h(t, "implementer", "withdraw", two)
	want := "handoff: W1 is still in review (#" + one + "). While reviewer-2 isn't asked again on it, the operator's go-ahead doesn't approve it: the operator decides it.\n"
	if res.code != 0 || res.err != want {
		t.Fatalf("withdraw: exit %d %q\nwant %q", res.code, res.err, want)
	}
	if res := r.h(t, "implementer", "withdraw", one); res.code != 0 || res.err != "" {
		t.Fatalf("the last one: exit %d %q", res.code, res.err)
	}
}

// The spare woken for the implementer seat is told to keep the changes the
// old session didn't commit; for a reviewer seat, it isn't.
func TestH38TheImplementerSeatsWakeSaysToKeepItsChanges(t *testing.T) {
	t.Parallel()
	keep := " Its old session may have left changes it didn't commit: don't discard, stash or overwrite them; tell the operator what you find."
	for _, seat := range []string{"implementer", "reviewer"} {
		t.Run(seat, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			pid, end := cliEnds(t)
			r.as(t, claude("s-seat", pid), "role", seat)
			sp := claude("s-spare", cli(t))
			r.as(t, sp, "role", "spare")
			if seat == "implementer" {
				r.ok(t, "reviewer", "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "n"))
			} else {
				r.request(t, "W1", r.commitA)
			}
			if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
				t.Fatalf("away: %v\n%s", err, out)
			}
			end()
			r.setQuiet(t, "1s")
			time.Sleep(2100 * time.Millisecond)
			res := r.run(t, "", "", sp, "wait", "--timeout", "5s")
			if res.code != 0 || !strings.Contains(res.out, seat+"'s seat is stuck: its session's process has ended.") {
				t.Fatalf("the spare's wait: exit %d %q %q", res.code, res.out, res.err)
			}
			if got := strings.Contains(res.out, " role "+seat+"`: the operator's cover is their word for it."+keep); got != (seat == "implementer") {
				t.Fatalf("the wake for %s, with the line %v: %q", seat, got, res.out)
			}
		})
	}
}

// A round cut short, whose replaced request was withdrawn since, completes
// as it was posted, as with one decided since; the withdrawal stands.
func TestH38ACutRoundCompletesPastAWithdrawal(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("round 1: exit %d\n%s", res.code, res.err)
	}
	old := r.requests(t, "W1")
	all := strings.Join(ids(old), ",")
	if cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitB, "--supersedes", all); cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	r.ok(t, "implementer", "withdraw", ids(old)[2])
	if res := r.postRound(t, notify, "W1", r.commitB, "--supersedes", all); res.code != 0 || strings.Count(res.out, "posted #") != 2 {
		t.Fatalf("completing: exit %d\n%s%s", res.code, res.out, res.err)
	}
	if req := r.state(t, old[2].ID); req.State != status.Withdrawn {
		t.Fatalf("the withdrawn old request: %s", req.State)
	}
	if es := r.requests(t, "W1"); !r.state(t, es[3].ID).Round.Complete {
		t.Fatal("round 2 is not complete")
	}
}
