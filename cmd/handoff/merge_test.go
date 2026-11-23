package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// H34 part 1 (row 108): work approved by the operator's decision, or under
// the go-ahead, stays in its requester's inbox as "approved, to merge" until
// its commit is in main, the requester records a squash or rebase merge, or
// the operator holds it.

// decided approves item at commit, by the reviewer's APPROVE and the
// operator's decision at a terminal, and returns the request's id.
func (r *repo) decided(t *testing.T, item, commit string) string {
	t.Helper()
	req := r.request(t, item, commit)
	r.ok(t, "reviewer", "reply", req, "--verdict", "APPROVE", "--commit", commit, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", req, "--approve"); err != nil {
		t.Fatalf("decide #%s: %v\n%s", req, err, out)
	}
	return req
}

// toMerge is the implementer's inbox, from its "approved, to merge" list on.
func (r *repo) toMerge(t *testing.T) string {
	t.Helper()
	out := r.ok(t, "implementer", "inbox")
	if i := strings.Index(out, "approved, to merge:"); i >= 0 {
		return out[i:]
	}
	return ""
}

func (r *repo) mergeMain(t *testing.T, commit string) {
	t.Helper()
	mustGit(t, r.dir, "switch", "-q", "main")
	mustGit(t, r.dir, "merge", "-q", "--ff-only", commit)
	mustGit(t, r.dir, "switch", "-q", "w1")
}

func TestH34p1ApprovedWorkIsListedUntilItsCommitIsInMain(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	req := r.decided(t, "W1", r.commitA)
	list := r.toMerge(t)
	if !strings.Contains(list, "W1 @ "+r.commitA[:12]+", approved by the operator (#") || !strings.Contains(list, "on the APPROVE of reviewer (#"+req+"): not in main yet; not yet delivered") {
		t.Fatalf("the implementer's inbox:\n%s", list)
	}
	if res := r.run(t, "implementer", "", nil, "inbox", "--quiet"); res.code != exitWaiting {
		t.Fatalf("inbox --quiet: exit %d", res.code)
	}
	// Delivered, the line says no more.
	r.run(t, "implementer", "", nil, "wait", "--timeout", "2s")
	if list := r.toMerge(t); strings.Contains(list, "not yet delivered") || !strings.Contains(list, "not in main yet") {
		t.Fatalf("after delivery:\n%s", list)
	}
	// Owed: quiet once quiet_after passes.
	r.setQuiet(t, "1s")
	time.Sleep(1100 * time.Millisecond)
	if out := r.ok(t, "implementer", "inbox"); !strings.Contains(out, "W1: implementer hasn't merged "+r.commitA[:12]+", approved by the operator") {
		t.Fatalf("gone quiet:\n%s", out)
	}
	// A denied request is never listed.
	w2 := r.request(t, "W2", r.commitB)
	r.ok(t, "reviewer", "reply", w2, "--verdict", "APPROVE", "--commit", r.commitB, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "W2", "--deny"); err != nil {
		t.Fatalf("deny W2: %v\n%s", err, out)
	}
	if list := r.toMerge(t); strings.Contains(list, "W2") {
		t.Fatalf("a denied request listed:\n%s", list)
	}
	// In main: done.
	r.mergeMain(t, r.commitA)
	if list := r.toMerge(t); list != "" {
		t.Fatalf("after the merge:\n%s", list)
	}
	if res := r.run(t, "implementer", "", nil, "inbox", "--quiet"); res.code != exitOK {
		t.Fatalf("inbox --quiet after the merge: exit %d", res.code)
	}
}

// Under the go-ahead, two reviewers approving one commit make one line and
// one alert; two items stay independent, one merged.
func TestH34p1OneLinePerItemAndCommitUnderTheGoAhead(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if out, err := r.atTerminal(t, "", nil, "away", "Merge when approved."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	post := func(item, commit string) []string {
		out := r.ok(t, "implementer", "post", "--item", item, "--to", "reviewer,reviewer-2", "--commit", commit, "--title", item, "--body", r.body(t, "b"))
		var ids []string
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			ids = append(ids, idOf(t, l))
		}
		return ids
	}
	w1, w2 := post("W1", r.commitA), post("W2", r.commitB)
	for _, reqs := range []struct {
		ids    []string
		commit string
	}{{w1, r.commitA}, {w2, r.commitB}} {
		for i, role := range []string{"reviewer", "reviewer-2"} {
			r.ok(t, role, "reply", reqs.ids[i], "--verdict", "APPROVE", "--commit", reqs.commit, "--body", r.body(t, "ok"))
		}
	}
	list := r.toMerge(t)
	want := "W1 @ " + r.commitA[:12] + ", approved under the operator's go-ahead #"
	if strings.Count(list, want) != 1 || !strings.Contains(list, "on the APPROVE of reviewer and reviewer-2 (#"+w1[0]+", #"+w1[1]+")") || strings.Count(list, "W2 @ ") != 1 {
		t.Fatalf("the implementer's inbox:\n%s", list)
	}
	r.setQuiet(t, "1s")
	time.Sleep(1100 * time.Millisecond)
	out := r.ok(t, "implementer", "inbox")
	if strings.Count(out, "W1: implementer hasn't merged") != 1 || strings.Count(out, "W2: implementer hasn't merged") != 1 {
		t.Fatalf("one alert each:\n%s", out)
	}
	r.mergeMain(t, r.commitA)
	if list := r.toMerge(t); strings.Contains(list, "W1 @") || !strings.Contains(list, "W2 @") {
		t.Fatalf("after merging W1 only:\n%s", list)
	}
}

// The operator holds work approved and not merged with decide --deny: every
// request in it, and the requester, which had the approval, is woken.
func TestH34p1TheOperatorHoldsApprovedWork(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if out, err := r.atTerminal(t, "", nil, "away", "Merge when approved."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	req := r.request(t, "W1", r.commitA)
	approve := idOf(t, r.ok(t, "reviewer", "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")))
	r.run(t, "implementer", "", nil, "wait", "--timeout", "2s") // the approval delivered
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--approve"); err == nil || !strings.Contains(out, "approved already, and not merged; --deny holds it") {
		t.Fatalf("decide --approve on approved work: %v\n%s", err, out)
	}
	out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "W1", "--deny")
	if err != nil || !strings.Contains(out, "Hold W1 @ "+r.commitA[:12]+"? implementer is told not to merge it.") {
		t.Fatalf("decide --deny on approved work: %v\n%s", err, out)
	}
	if list := r.toMerge(t); list != "" {
		t.Fatalf("held, still listed:\n%s", list)
	}
	woke := r.run(t, "implementer", "", nil, "wait", "--timeout", "2s")
	if woke.code != 0 || !strings.Contains(woke.out, "decision W1 operator → implementer: deny #"+req) {
		t.Fatalf("the requester's wait: exit %d\n%s%s", woke.code, woke.out, woke.err)
	}
	es := r.snap(t).Entries
	hold := es[len(es)-1]
	if hold.Body != "Held: approved, but don't merge it." {
		t.Fatalf("the hold: %+v", hold)
	}
	// show names the hold (the reviewer, #468).
	if out := r.ok(t, "", "show", req); !strings.Contains(out, fmt.Sprintf("approval: approved, then held by the operator (#%d)", hold.ID)) {
		t.Fatalf("show #%s:\n%s", req, out)
	}
	// And the APPROVE no longer says it is the operator's approve
	// (reviewer-3, #475).
	if out := r.ok(t, "", "show", approve); !strings.Contains(out, fmt.Sprintf("then held by the operator (#%d): don't merge it", hold.ID)) || strings.Contains(out, "that is the operator's approve") {
		t.Fatalf("show #%s:\n%s", approve, out)
	}
	// Merged all the same: the operator is told.
	r.mergeMain(t, r.commitA)
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, fmt.Sprintf("held by you, but in main:\n  W1 @ %s for implementer (held in #%d)", r.commitA[:12], hold.ID)) {
		t.Fatalf("the operator's view:\n%s%s", home.out, home.err)
	}
	// Held is done: nothing more to hold.
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--deny"); err == nil || !strings.Contains(out, "nothing on W1 awaits the operator") {
		t.Fatalf("a second hold: %v\n%s", err, out)
	}
}

// An item with a request awaiting the operator and other work approved and
// not merged: decide <item> decides the first and names the other.
func TestH34p1DecideNamesApprovedWorkNotMerged(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	first := r.decided(t, "W1", r.commitA)
	other := idOf(t, r.ok(t, "reviewer-2", "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitB, "--title", "W1 more", "--body", r.body(t, "b")))
	r.ok(t, "reviewer", "reply", other, "--verdict", "APPROVE", "--commit", r.commitB, "--body", r.body(t, "ok"))
	// Two awaiting: the error says to give the id, whatever is approved
	// beside them (the reviewer, #468).
	third := idOf(t, r.ok(t, "reviewer", "post", "--item", "W1", "--to", "reviewer-2", "--commit", r.commitB, "--title", "W1 too", "--body", r.body(t, "b")))
	r.ok(t, "reviewer-2", "reply", third, "--verdict", "APPROVE", "--commit", r.commitB, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--deny"); err == nil || !strings.Contains(out, "W1 has 2 entries awaiting the operator") {
		t.Fatalf("decide W1 --deny with two awaiting: %v\n%s", err, out)
	}
	out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", other, "--approve")
	if err != nil || !strings.Contains(out, "also on W1, approved and not merged: W1 @ "+r.commitA[:12]) || !strings.Contains(out, "to hold it, handoff decide "+first+" --deny") {
		t.Fatalf("decide W1: %v\n%s", err, out)
	}
	if d, _ := r.decision(t, other); d != "approve" {
		t.Fatalf("decided #%s: %q", other, d)
	}
}

// After a squash the approved commit never reaches main: the requester
// records it, and only it, with a commit that is in main. The operator's
// view shows it as recorded on the requester's word.
func TestH34p1MergedRecordsASquash(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	req := r.decided(t, "W1", r.commitA)
	mustGit(t, r.dir, "switch", "-q", "main")
	mustGit(t, r.dir, "merge", "-q", "--squash", r.commitA)
	mustGit(t, r.dir, "commit", "-q", "-m", "W1, squashed")
	squash := mustGit(t, r.dir, "rev-parse", "HEAD")
	mustGit(t, r.dir, "switch", "-q", "w1")
	// A note doesn't close it.
	r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "rebasing after a conflict"))
	if list := r.toMerge(t); !strings.Contains(list, "W1 @ "+r.commitA[:12]) {
		t.Fatalf("after a note:\n%s", list)
	}
	r.refused(t, "reviewer", "only it records the merge", "merged", "W1", squash)
	r.refused(t, "implementer", "is not in main", "merged", "W1", r.commitB)
	r.refused(t, "operator", "merged is for the agent that asked for the work", "merged", "W1", squash)
	if out := r.ok(t, "implementer", "merged", req, squash); !strings.Contains(out, "merged W1 implementer → -") {
		t.Fatalf("merged: %s", out)
	}
	if out := r.ok(t, "", "show", req); !strings.Contains(out, "approval: merged as "+squash[:12]+", recorded by implementer (#") {
		t.Fatalf("show #%s:\n%s", req, out)
	}
	if list := r.toMerge(t); list != "" {
		t.Fatalf("after merged:\n%s", list)
	}
	r.refused(t, "implementer", "nothing on W1 is approved and not merged", "merged", "W1", squash)
	home := r.run(t, "", "", nil)
	if !strings.Contains(home.out, "W1 @ "+r.commitA[:12]+": merged as "+squash[:12]+", recorded by implementer") {
		t.Fatalf("the operator's view:\n%s%s", home.out, home.err)
	}
}

// When git can't tell, the line says why, nothing alerts, and wait goes on.
func TestH34p1WhenGitCantTell(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.decided(t, "W1", r.commitA)
	path := filepath.Join(r.dir, ".handoff", "config.json")
	b, _ := os.ReadFile(path)
	var c map[string]any
	json.Unmarshal(b, &c)
	c["main"] = "no-such-branch"
	b, _ = json.Marshal(c)
	os.WriteFile(path, b, 0o600)
	if list := r.toMerge(t); !strings.Contains(list, "can't tell: git merge-base --is-ancestor "+r.commitA[:12]+" refs/heads/no-such-branch: fatal: Not a valid object name") {
		t.Fatalf("the implementer's inbox:\n%s", list)
	}
	r.setQuiet(t, "1s")
	time.Sleep(1100 * time.Millisecond)
	if out := r.ok(t, "implementer", "inbox"); strings.Contains(out, "hasn't merged") {
		t.Fatalf("an alert for what git can't tell:\n%s", out)
	}
	r.run(t, "implementer", "", nil, "wait", "--timeout", "2s") // the decision
	if res := r.run(t, "implementer", "", nil, "wait", "--timeout", "2s"); res.code != 3 {
		t.Fatalf("wait: exit %d\n%s", res.code, res.err)
	}
}

// Approvals before tracking began are history; a request posted before it
// and approved after is tracked.
func TestH34p1ApprovalsBeforeTrackingAreHistory(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.decided(t, "W1", r.commitA)
	w2 := r.request(t, "W2", r.commitB)
	// As if the first handoff with row 108 opened the project now.
	os.Remove(filepath.Join(r.dir, ".handoff", "tracking.json"))
	r.ok(t, "implementer", "inbox")
	r.ok(t, "reviewer", "reply", w2, "--verdict", "APPROVE", "--commit", r.commitB, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "W2", "--approve"); err != nil {
		t.Fatalf("decide W2: %v\n%s", err, out)
	}
	if list := r.toMerge(t); strings.Contains(list, "W1 @") || !strings.Contains(list, "W2 @") {
		t.Fatalf("the implementer's inbox:\n%s", list)
	}
}

// The approval of work is not listed once a newer review replaced it, as
// when the requester's next post supersedes the request of a reviewer who
// asked for changes, while the operator approved the other in a pop-up;
// work the requester keeps open beside newer work stays listed (row 108;
// reviewer-2, #467).
func TestH34p1AnApprovalTheWorkWentOnFromIsNotListed(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		ids = append(ids, idOf(t, l))
	}
	r.ok(t, "reviewer", "reply", ids[0], "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	r.ok(t, "reviewer-2", "reply", ids[1], "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix"))
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", ids[0], "--approve"); err != nil {
		t.Fatalf("decide #%s: %v\n%s", ids[0], err, out)
	}
	if list := r.toMerge(t); list != "" {
		t.Fatalf("approved while a CHANGES stands:\n%s", list)
	}
	r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer-2", "--commit", r.commitB, "--title", "W1 fixed", "--supersedes", ids[1], "--body", r.body(t, "b"))
	if list := r.toMerge(t); list != "" {
		t.Fatalf("the work went on, still listed:\n%s", list)
	}
	// Kept open: W2 approved at A, then a request on W2 at B that supersedes
	// nothing.
	r.decided(t, "W2", r.commitA)
	r.request(t, "W2", r.commitB)
	if list := r.toMerge(t); !strings.Contains(list, "W2 @ "+r.commitA[:12]) {
		t.Fatalf("W2 kept open, not listed:\n%s", list)
	}
}

// A session that lost its role can't record a merge (row 107).
func TestH34p1AMarkedSessionCantRecordAMerge(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	old := claude("s-old", cli(t))
	r.as(t, old, "role", "implementer")
	r.decided(t, "W1", r.commitA)
	r.as(t, claude("s-new", cli(t)), "role", "implementer")
	r.refusedIn(t, old, "you no longer hold implementer", "merged", "W1", r.base)
}

// reviewer-3's #475: two requesters' work approved on one item, nothing
// awaiting the operator: decide <item> --deny says to give the id.
func TestH34p1HoldingOneOfTwoApprovalsTakesTheId(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	first := r.decided(t, "W1", r.commitA)
	other := idOf(t, r.ok(t, "reviewer-2", "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitB, "--title", "W1 more", "--body", r.body(t, "b")))
	r.ok(t, "reviewer", "reply", other, "--verdict", "APPROVE", "--commit", r.commitB, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", other, "--approve"); err != nil {
		t.Fatalf("decide #%s: %v\n%s", other, err, out)
	}
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--deny"); err == nil || !strings.Contains(out, "W1 has 2 approvals not merged (#"+first+", #"+other+"); give the id") {
		t.Fatalf("decide W1 --deny: %v\n%s", err, out)
	}
}
