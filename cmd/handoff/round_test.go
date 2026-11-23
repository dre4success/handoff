package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// H34 part 7 (row 106): one post asks several reviewers, as one round, in
// one write. A round is complete only once every role on its panel has its
// request, so a write cut short approves nothing under the go-ahead, and the
// same post again completes it.

var panel = []string{"reviewer", "reviewer-2", "reviewer-3"}

// newPanelRepo is newRepo with a third reviewer.
func newPanelRepo(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	r.ok(t, "", "roles", "add", "reviewer-3")
	return r
}

// postRound posts item at commit to all three reviewers, with extra flags.
func (r *repo) postRound(t *testing.T, env []string, item, commit string, extra ...string) result {
	t.Helper()
	args := append([]string{"post", "--item", item, "--to", strings.Join(panel, ","), "--commit", commit,
		"--title", item + " work", "--body", r.body(t, "please review "+item)}, extra...)
	return r.run(t, "implementer", "", env, args...)
}

// requests returns the record's requests on item, in order.
func (r *repo) requests(t *testing.T, item string) []record.Entry {
	t.Helper()
	var out []record.Entry
	for _, e := range r.snap(t).Entries {
		if (e.Kind == "request" || e.Kind == "round") && e.Item == item {
			out = append(out, e)
		}
	}
	return out
}

func (r *repo) state(t *testing.T, id int64) *status.Request {
	t.Helper()
	req, ok := status.Compute(r.snap(t).Entries).Request(id)
	if !ok {
		t.Fatalf("#%d is not a request", id)
	}
	return req
}

func ids(es []record.Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, strconv.FormatInt(e.ID, 10))
	}
	return out
}

func TestH34p7OnePostIsOneRoundInOneWrite(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	res := r.postRound(t, nil, "W1", r.commitA)
	if res.code != 0 {
		t.Fatalf("post to three: exit %d\n%s", res.code, res.err)
	}
	es := r.requests(t, "W1")
	if len(es) != 3 {
		t.Fatalf("%d requests, want 3:\n%s", len(es), res.out)
	}
	for i, e := range es {
		if e.Kind != "round" || e.To != panel[i] || e.Round != es[0].ID || !slices.Equal(e.Panel, panel) || e.Commit != r.commitA || e.TS != es[0].TS || e.ID != es[0].ID+int64(i) {
			t.Fatalf("request %d: %+v", i, e)
		}
	}
	// Stored as kind round, a round's request is shown as a request
	// (the reviewer, #329).
	for i, role := range panel {
		if want := "posted #" + strconv.FormatInt(es[i].ID, 10) + " request W1 implementer → " + role + ": W1 work"; !strings.Contains(res.out, want) {
			t.Fatalf("the receipt doesn't say %q:\n%s", want, res.out)
		}
	}
	if in := r.ok(t, "", "inbox", "--for", "reviewer-2"); !strings.Contains(in, "  #"+strconv.FormatInt(es[1].ID, 10)+" request W1 implementer → reviewer-2: W1 work") || strings.Contains(in, " round W1") {
		t.Fatalf("reviewer-2's inbox:\n%s", in)
	}
	if show := r.ok(t, "", "show", strconv.FormatInt(es[1].ID, 10)); !strings.HasPrefix(show, "#"+strconv.FormatInt(es[1].ID, 10)+" request W1\n") {
		t.Fatalf("show:\n%s", show)
	}
	if show := r.ok(t, "", "show", strconv.FormatInt(es[1].ID, 10)); !strings.Contains(show, "\nround: #"+strconv.FormatInt(es[0].ID, 10)+", to reviewer, reviewer-2 and reviewer-3\n") {
		t.Fatalf("show:\n%s", show)
	}

	// Posted again, the complete round writes nothing and says what exists.
	again := r.postRound(t, nil, "W1", r.commitA)
	if again.code != 0 || strings.Count(again.out, "exists #") != 3 || strings.Contains(again.out, "posted") {
		t.Fatalf("the same round again: exit %d\n%s%s", again.code, again.out, again.err)
	}
	if n := len(r.requests(t, "W1")); n != 3 {
		t.Fatalf("posting the round again wrote: %d requests", n)
	}

	// The refusals.
	r.refused(t, "implementer", "--to names reviewer twice",
		"post", "--item", "W2", "--to", "reviewer,reviewer", "--commit", r.commitA, "--title", "t", "--body", r.body(t, "b"))
	r.refused(t, "implementer", "--supersedes takes one id for each role in --to, and --to names one",
		"post", "--item", "W1", "--to", "reviewer", "--commit", r.commitB, "--title", "t", "--body", r.body(t, "b"), "--supersedes", "1,2")
	one, two := strconv.FormatInt(es[0].ID, 10), strconv.FormatInt(es[1].ID, 10)
	r.refused(t, "implementer", "request #"+strconv.FormatInt(es[2].ID, 10)+" on W1 to reviewer-3 is still open",
		"post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitB, "--title", "t", "--body", r.body(t, "b"), "--supersedes", one+","+two)
	r.refused(t, "implementer", "is not a request on W1 from implementer to reviewer or reviewer-2",
		"post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitB, "--title", "t", "--body", r.body(t, "b"), "--supersedes", one+","+r.request(t, "W9", r.commitA))
	w2a := r.request(t, "W2", r.commitA)
	w2b := r.request(t, "W2", r.commitB, "--supersedes", w2a)
	r.refused(t, "implementer", "#"+w2a+" and #"+w2b+" both go to reviewer; --supersedes takes one for each role",
		"post", "--item", "W2", "--to", "reviewer,reviewer-2", "--commit", r.commitB, "--title", "t", "--body", r.body(t, "b"), "--supersedes", w2a+","+w2b)
}

func TestH34p7GoAheadNeedsTheWholeRound(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("post: exit %d\n%s", res.code, res.err)
	}
	es := r.requests(t, "W1")
	for i, role := range panel {
		r.approve(t, notify, role, strconv.FormatInt(es[i].ID, 10), r.commitA)
		covered := r.state(t, es[0].ID).GoAhead != nil
		if covered != (i == 2) {
			t.Fatalf("after %d of 3 approvals, approved under the go-ahead: %v", i+1, covered)
		}
	}
}

func TestH34p7ACutRoundApprovesNothingAndTheSamePostCompletesIt(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	sent, notify := sink(t)
	if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitA)
	if cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	es := r.requests(t, "W1")
	if len(es) != 1 || es[0].To != "reviewer" || es[0].Round != es[0].ID {
		t.Fatalf("after the cut: %+v", es)
	}
	first := strconv.FormatInt(es[0].ID, 10)

	// Its one request is reviewed and approved, and approves nothing.
	r.approve(t, notify, "reviewer", first, r.commitA)
	if req := r.state(t, es[0].ID); req.GoAhead != nil || req.State != status.AwaitingOperator {
		t.Fatalf("the cut round's one approval: %s, under %v", req.State, req.GoAhead)
	}
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: reviewer approved #"+first+", but the round #"+first+" to reviewer, reviewer-2 and reviewer-3 has no request to reviewer-2 or reviewer-3, so your go-ahead doesn't cover it. Your decision.") {
		t.Fatalf("the operator was told:\n%s", got)
	}
	in := r.ok(t, "", "inbox", "--for", "implementer")
	if !strings.Contains(in, "\nincomplete, to post again as it was:\n  round #"+first+" on W1, to reviewer, reviewer-2 and reviewer-3 @ "+r.commitA[:12]+": no request to reviewer-2 or reviewer-3\n") {
		t.Fatalf("the implementer's inbox:\n%s", in)
	}
	if res := r.h(t, "", "inbox", "--for", "implementer", "--quiet"); res.code != exitWaiting {
		t.Fatalf("inbox --quiet for the implementer: exit %d", res.code)
	}
	if show := r.ok(t, "", "show", first); !strings.Contains(show, "\nround state: incomplete: no request to reviewer-2 or reviewer-3, so nothing on W1 is approved under the go-ahead; implementer posts the same again to complete it\n") {
		t.Fatalf("show:\n%s", show)
	}

	// Anything else on the item is refused until it is complete.
	r.refused(t, "implementer", "round #"+first+" on W1, to reviewer, reviewer-2 and reviewer-3 at "+r.commitA[:12]+", is incomplete: post it again as it was",
		"post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "t", "--body", r.body(t, "b"))
	r.refused(t, "implementer", "is incomplete: post it again as it was",
		"post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitB, "--title", "t", "--body", r.body(t, "b"))
	r.refused(t, "implementer", "is incomplete: post it again as it was",
		"post", "--item", "W1", "--to", "reviewer-2", "--commit", r.commitA, "--title", "t", "--body", r.body(t, "b"))

	// The same post, with no --key, writes only what is missing.
	done := r.postRound(t, notify, "W1", r.commitA)
	if done.code != 0 || !strings.Contains(done.out, "exists #"+first+" ") || strings.Count(done.out, "posted #") != 2 {
		t.Fatalf("the same post again: exit %d\n%s%s", done.code, done.out, done.err)
	}
	es = r.requests(t, "W1")
	if len(es) != 3 || es[1].Round != es[0].ID || es[2].Round != es[0].ID || es[1].To != "reviewer-2" || es[2].To != "reviewer-3" {
		t.Fatalf("after completing: %+v", es)
	}
	if in := r.ok(t, "", "inbox", "--for", "implementer"); strings.Contains(in, "incomplete") {
		t.Fatalf("the inbox still shows it incomplete:\n%s", in)
	}
	// Complete, it is approved once the other two approve.
	r.approve(t, notify, "reviewer-2", strconv.FormatInt(es[1].ID, 10), r.commitA)
	if r.state(t, es[0].ID).GoAhead != nil {
		t.Fatal("approved under the go-ahead with one approval still owed")
	}
	r.approve(t, notify, "reviewer-3", strconv.FormatInt(es[2].ID, 10), r.commitA)
	if r.state(t, es[0].ID).GoAhead == nil {
		t.Fatal("not approved under the go-ahead once all three approved")
	}
}

func TestH34p7ANewRoundSupersedesTheOldOneWhole(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("round 1: exit %d\n%s", res.code, res.err)
	}
	old := r.requests(t, "W1")
	olds := strings.Join(ids(old), ",")
	if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	// Round 1 is answered; reviewer approves it, so the item would be
	// covered if that approval and the others met at one commit.
	r.approve(t, notify, "reviewer", ids(old)[0], r.commitA)

	// Round 2, cut after its first request.
	cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitB, "--supersedes", olds)
	if cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	es := r.requests(t, "W1")
	if len(es) != 4 {
		t.Fatalf("after the cut: %d requests", len(es))
	}
	nu := es[3]
	if nu.Round != nu.ID || nu.To != "reviewer" || nu.Supersedes != old[0].ID {
		t.Fatalf("the cut round's request: %+v", nu)
	}
	// Staged: all three old requests are still live, and nothing is
	// approved under the go-ahead, whatever is approved.
	for _, e := range old {
		if st := r.state(t, e.ID).State; st == status.Superseded || st == status.Decided {
			t.Fatalf("old #%d is %s while its replacement is incomplete", e.ID, st)
		}
	}
	r.approve(t, notify, "reviewer-2", ids(old)[1], r.commitA)
	r.approve(t, notify, "reviewer-3", ids(old)[2], r.commitA)
	r.approve(t, notify, "reviewer", strconv.FormatInt(nu.ID, 10), r.commitB)
	for _, id := range []int64{old[0].ID, old[1].ID, old[2].ID, nu.ID} {
		if req := r.state(t, id); req.GoAhead != nil {
			t.Fatalf("#%d approved under the go-ahead while a round on W1 is incomplete", id)
		}
	}

	// The same post completes it, and the old round is superseded whole.
	done := r.postRound(t, notify, "W1", r.commitB, "--supersedes", olds)
	if done.code != 0 || strings.Count(done.out, "posted #") != 2 {
		t.Fatalf("completing round 2: exit %d\n%s%s", done.code, done.out, done.err)
	}
	es = r.requests(t, "W1")
	if len(es) != 6 {
		t.Fatalf("after completing: %d requests", len(es))
	}
	for i, e := range old {
		req := r.state(t, e.ID)
		if req.State != status.Superseded || req.SupersededBy != es[3+i].ID {
			t.Fatalf("old #%d: %s, by #%d; want superseded by #%d", e.ID, req.State, req.SupersededBy, es[3+i].ID)
		}
	}
	if r.state(t, nu.ID).GoAhead != nil {
		t.Fatal("round 2 approved under the go-ahead on one approval of three")
	}
	r.approve(t, notify, "reviewer-2", strconv.FormatInt(es[4].ID, 10), r.commitB)
	r.approve(t, notify, "reviewer-3", strconv.FormatInt(es[5].ID, 10), r.commitB)
	if r.state(t, nu.ID).GoAhead == nil {
		t.Fatal("round 2 not approved under the go-ahead once all three approved")
	}
}

// deny has the operator deny an approved request at a terminal.
func (r *repo) deny(t *testing.T, notify []string, id string) {
	t.Helper()
	if out, err := r.atTerminalWith(t, notify, []step{{"[y/N]", "yes"}}, "decide", id, "--deny"); err != nil {
		t.Fatalf("decide %s --deny: %v\n%s", id, err, out)
	}
}

// reviewer-2's first case in #308: denying the only request a cut round
// wrote kept no barrier, and the old round was approved under the go-ahead.
func TestH34p7ADecisionOnACutRoundKeepsItsBarrier(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("round 1: exit %d\n%s", res.code, res.err)
	}
	old := r.requests(t, "W1")
	if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	if cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitB, "--supersedes", strings.Join(ids(old), ",")); cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	nu := r.requests(t, "W1")[3]
	r.approve(t, notify, "reviewer", strconv.FormatInt(nu.ID, 10), r.commitB)
	r.deny(t, notify, strconv.FormatInt(nu.ID, 10))
	if st := r.state(t, nu.ID).State; st != status.Decided {
		t.Fatalf("the cut round's request: %s, want decided", st)
	}
	for i, role := range panel {
		r.approve(t, notify, role, ids(old)[i], r.commitA)
	}
	for _, e := range old {
		if req := r.state(t, e.ID); req.GoAhead != nil {
			t.Fatalf("old #%d approved under the go-ahead while the round replacing it is incomplete", e.ID)
		}
	}
	if in := r.ok(t, "", "inbox", "--for", "implementer"); !strings.Contains(in, "round #"+strconv.FormatInt(nu.ID, 10)+" on W1, to reviewer, reviewer-2 and reviewer-3 @ "+r.commitB[:12]+": no request to reviewer-2 or reviewer-3") {
		t.Fatalf("the inbox:\n%s", in)
	}
}

// A denial can't shrink a complete round's panel: the other two approving
// approves nothing under the go-ahead.
func TestH34p7ADenialShrinksNoPanel(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	sent, notify := sink(t)
	if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
		t.Fatalf("away: %v\n%s", err, out)
	}
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("post: exit %d\n%s", res.code, res.err)
	}
	es := r.requests(t, "W1")
	r.approve(t, notify, "reviewer", ids(es)[0], r.commitA)
	r.deny(t, notify, ids(es)[0])
	r.approve(t, notify, "reviewer-2", ids(es)[1], r.commitA)
	r.approve(t, notify, "reviewer-3", ids(es)[2], r.commitA)
	for _, e := range es[1:] {
		if req := r.state(t, e.ID); req.GoAhead != nil || req.State != status.AwaitingOperator {
			t.Fatalf("#%d after a denial of the round's first: %s, under %v", e.ID, req.State, req.GoAhead)
		}
	}
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "but you denied a request of the round #"+ids(es)[0]+", which leaves its panel smaller than it asked, so your go-ahead doesn't cover it. Your decision.") {
		t.Fatalf("the operator was told:\n%s", got)
	}
}

// reviewer-2's third case in #308: the same post again after every approval
// wrote a second round. With a key or without, it writes nothing.
func TestH34p7TheSamePostAfterApprovalWritesNothing(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "k1"} {
		r := newPanelRepo(t)
		_, notify := sink(t)
		if out, err := r.atTerminalWith(t, notify, nil, "away", "Merge when the reviewers approve."); err != nil {
			t.Fatalf("away: %v\n%s", err, out)
		}
		var extra []string
		if key != "" {
			extra = []string{"--key", key}
		}
		body := r.body(t, "please review W1")
		post := func() result {
			args := append([]string{"post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitA, "--title", "W1 work", "--body", body}, extra...)
			return r.run(t, "implementer", "", notify, args...)
		}
		if res := post(); res.code != 0 {
			t.Fatalf("key %q: post: exit %d\n%s", key, res.code, res.err)
		}
		es := r.requests(t, "W1")
		for i, role := range panel {
			r.approve(t, notify, role, ids(es)[i], r.commitA)
		}
		if r.state(t, es[0].ID).GoAhead == nil {
			t.Fatalf("key %q: not approved under the go-ahead", key)
		}
		again := post()
		if again.code != 0 || strings.Count(again.out, "exists #") != 3 || strings.Contains(again.out, "posted") {
			t.Fatalf("key %q: the same post after approval: exit %d\n%s%s", key, again.code, again.out, again.err)
		}
		if n := len(r.requests(t, "W1")); n != 3 {
			t.Fatalf("key %q: %d requests after posting again, want 3", key, n)
		}
	}
}

// reviewer-2's fourth case in #308: completing a cut round with another
// title, body or base is refused, writing nothing, and completing it takes
// the first request's own base.
func TestH34p7CompletingARoundTakesItsOwnTemplate(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	body := r.body(t, "please review W1")
	post := func(env []string, title, b string, extra ...string) result {
		args := append([]string{"post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitB, "--title", title, "--body", b}, extra...)
		return r.run(t, "implementer", "", env, args...)
	}
	if cut := post([]string{"HANDOFF_TEST_FAULT=first-line"}, "W1 work", body, "--base", r.commitA); cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	first := r.requests(t, "W1")[0]
	for _, c := range []struct {
		name string
		res  result
	}{
		{"another title", post(nil, "W1 other", body, "--base", r.commitA)},
		{"another body", post(nil, "W1 work", r.body(t, "something else"), "--base", r.commitA)},
		{"another base", post(nil, "W1 work", body, "--base", r.base)},
		{"a key", post(nil, "W1 work", body, "--base", r.commitA, "--key", "k")},
	} {
		if c.res.code != 2 || !strings.Contains(c.res.err, "is incomplete: post it again as it was, with the same title, body and key") {
			t.Fatalf("%s: exit %d\n%s", c.name, c.res.code, c.res.err)
		}
		if n := len(r.requests(t, "W1")); n != 1 {
			t.Fatalf("%s wrote: %d requests", c.name, n)
		}
	}
	// With the base left to default, completing takes the first request's.
	done := post(nil, "W1 work", body)
	if done.code != 0 {
		t.Fatalf("completing: exit %d\n%s", done.code, done.err)
	}
	for _, e := range r.requests(t, "W1") {
		if e.Base != first.Base || e.Base != r.commitA || e.Title != first.Title || e.Body != first.Body {
			t.Fatalf("#%d: base %.12s, title %q; want base %.12s, title %q", e.ID, e.Base, e.Title, first.Base, first.Title)
		}
	}
}

// reviewer-3's #314 and the reviewer's #316 on keys: a key works for a round
// as for one request. The same post with the same key finds its round
// whatever its state; the key with other content is refused.
func TestH34p7AKeyGuardsARoundAsItDoesARequest(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	body := r.body(t, "please review W1")
	post := func(title, b string, extra ...string) result {
		args := append([]string{"post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitA, "--title", title, "--body", b, "--key", "k1"}, extra...)
		return r.run(t, "implementer", "", notify, args...)
	}
	if res := post("W1 work", body); res.code != 0 {
		t.Fatalf("post: exit %d\n%s", res.code, res.err)
	}
	es := r.requests(t, "W1")
	// Other content under the same key, the round still open: refused, by key.
	if res := post("W1 other", r.body(t, "something else")); res.code != 2 || !strings.Contains(res.err, `key "k1/reviewer" was used by #`+ids(es)[0]+" with different content; nothing written") {
		t.Fatalf("the key with other content: exit %d\n%s", res.code, res.err)
	}
	// Without a key, other content while the round is open: still open.
	if res := r.run(t, "implementer", "", notify, "post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitA, "--title", "W1 other", "--body", body); res.code != 2 || !strings.Contains(res.err, "is still open") {
		t.Fatalf("other content, no key: exit %d\n%s", res.code, res.err)
	}
	// A new round supersedes it; the first post again, with its key, exists.
	if res := r.run(t, "implementer", "", notify, "post", "--item", "W1", "--to", strings.Join(panel, ","), "--commit", r.commitB, "--title", "W1 r2", "--body", body, "--supersedes", strings.Join(ids(es), ",")); res.code != 0 {
		t.Fatalf("round 2: exit %d\n%s", res.code, res.err)
	}
	if res := post("W1 work", body); res.code != 0 || strings.Count(res.out, "exists #") != 3 || strings.Contains(res.out, "posted") {
		t.Fatalf("the first post again, after supersession: exit %d\n%s%s", res.code, res.out, res.err)
	}
	if n := len(r.requests(t, "W1")); n != 6 {
		t.Fatalf("%d requests, want 6", n)
	}
	// A single request's key reused for a round is refused too.
	single := idOf(t, r.ok(t, "implementer", "post", "--item", "W2", "--to", "reviewer", "--commit", r.commitA, "--title", "W2", "--body", body, "--key", "k2/reviewer"))
	if res := r.run(t, "implementer", "", notify, "post", "--item", "W3", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W3", "--body", body, "--key", "k2"); res.code != 2 || !strings.Contains(res.err, `key "k2/reviewer" was used by #`+single+" with different content") {
		t.Fatalf("a request's key for a round: exit %d\n%s", res.code, res.err)
	}
}

// The reviewer's #316: a cut round is a move its requester owes. The
// operator hears of it once it has gone quiet, and with no go-ahead, the
// approval on it says the round is incomplete.
func TestH34p7ACutRoundIsOwedAndSaysSo(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	sent, notify := sink(t)
	if cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitA); cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	first := ids(r.requests(t, "W1"))[0]
	r.approve(t, notify, "reviewer", first, r.commitA)
	missing := "the round #" + first + " to reviewer, reviewer-2 and reviewer-3 has no request to reviewer-2 or reviewer-3"
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: reviewer approved #"+first+", but "+missing+". Your decision.") {
		t.Fatalf("with no go-ahead, the operator was told:\n%s", got)
	}
	if out, _ := r.atTerminal(t, "", []step{{"[y/N]", "n"}}, "decide", first, "--approve"); !strings.Contains(out, "  but "+missing+"\r\n") {
		t.Fatalf("decide showed:\n%s", out)
	}
	r.setQuiet(t, "1s")
	res := r.run(t, "", "", notify, "wait", "--for", "reviewer-3", "--timeout", "3s")
	if res.code != 3 {
		t.Fatalf("wait: exit %d\n%s", res.code, res.err)
	}
	if got := strings.Join(sentTo(t, sent), "\n"); !strings.Contains(got, "W1: "+missing+"; implementer can post the same again to complete it (") {
		t.Fatalf("once quiet, the operator was told:\n%s", got)
	}
	if home := r.ok(t, ""); !strings.Contains(home, missing+"; implementer can post the same again to complete it") {
		t.Fatalf("handoff:\n%s", home)
	}
}

// reviewer-2's first case in #331: a listener from before rounds can't see
// a round replace a request, so no round is posted while one listens.
func TestH34p7NoRoundWhileAnOlderListenerRuns(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	dir := filepath.Join(r.dir, ".handoff")
	lock, err := wait.Listen(dir, "implementer", "hook")
	if err != nil {
		t.Fatal(err)
	}
	// What a handoff before row 106 writes: no "rounds".
	old := `{"v":1,"pid":4242,"since":"2026-11-22T00:00:00Z","how":"hook"}` + "\n"
	if err := os.WriteFile(wait.PresencePath(dir, "implementer"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	res := r.postRound(t, nil, "W1", r.commitA)
	if res.code != 2 || !strings.Contains(res.err, "an older handoff listens for implementer (pid 4242), which can't see a round; nothing written. An idle listener ends once something is delivered to its role, and the next one is this handoff's: send that role a note, then post again") {
		t.Fatalf("a round with an older listener: exit %d\n%s", res.code, res.err)
	}
	if n := len(r.requests(t, "W1")); n != 0 {
		t.Fatalf("%d requests written", n)
	}
	// One request is as before; it needs no round.
	r.request(t, "W2", r.commitA)
	lock.Close()
	// A listener from this handoff is fine.
	lock, err = wait.Listen(dir, "implementer", "hook")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if res := r.postRound(t, nil, "W1", r.commitA); res.code != 0 {
		t.Fatalf("a round with a listener that reads rounds: exit %d\n%s", res.code, res.err)
	}
}

// reviewer-2's second case in #331: the round records its whole mapping, so
// a retry can't change it, and a request it replaces that the operator has
// decided since doesn't stop it being completed as posted.
func TestH34p7ARoundKeepsItsMappingAcrossADecision(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	_, notify := sink(t)
	if res := r.postRound(t, notify, "W1", r.commitA); res.code != 0 {
		t.Fatalf("round 1: exit %d\n%s", res.code, res.err)
	}
	old := r.requests(t, "W1")
	r.approve(t, notify, "reviewer-3", ids(old)[2], r.commitA)
	all := strings.Join(ids(old), ",")
	if cut := r.postRound(t, append([]string{"HANDOFF_TEST_FAULT=first-line"}, notify...), "W1", r.commitB, "--supersedes", all); cut.code == 0 {
		t.Fatalf("the cut post exited 0:\n%s", cut.out)
	}
	if nu := r.requests(t, "W1")[3]; len(nu.Replaces) != 3 || nu.Replaces["reviewer-3"] != old[2].ID {
		t.Fatalf("the round's first request records %v", nu.Replaces)
	}
	r.deny(t, notify, ids(old)[2])
	// The same post, with reviewer-3's old request left out: refused.
	short := strings.Join(ids(old)[:2], ",")
	if res := r.postRound(t, notify, "W1", r.commitB, "--supersedes", short); res.code != 2 || !strings.Contains(res.err, "is incomplete: post it again as it was") {
		t.Fatalf("a changed mapping: exit %d\n%s", res.code, res.err)
	}
	if n := len(r.requests(t, "W1")); n != 4 {
		t.Fatalf("the changed mapping wrote: %d requests", n)
	}
	// The same post as it was completes it; the decision stands.
	if res := r.postRound(t, notify, "W1", r.commitB, "--supersedes", all); res.code != 0 || strings.Count(res.out, "posted #") != 2 {
		t.Fatalf("completing: exit %d\n%s%s", res.code, res.out, res.err)
	}
	es := r.requests(t, "W1")
	for i := 0; i < 2; i++ {
		if req := r.state(t, old[i].ID); req.State != status.Superseded || req.SupersededBy != es[3+i].ID {
			t.Fatalf("old #%d: %s by #%d", old[i].ID, req.State, req.SupersededBy)
		}
	}
	if req := r.state(t, old[2].ID); req.State != status.Decided || req.Decision.Decision != "deny" {
		t.Fatalf("the denied old request: %s", req.State)
	}
	if !r.state(t, es[3].ID).Round.Complete {
		t.Fatal("round 2 is not complete")
	}
}

// reviewer-2's third case in #331: a round's key can't open a second round
// with another panel, nor a single request.
func TestH34p7ARoundsKeyCoversEveryPanel(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	r.ok(t, "", "roles", "add", "reviewer-4")
	body := r.body(t, "please review W1")
	post := func(to string) result {
		return r.run(t, "implementer", "", nil, "post", "--item", "W1", "--to", to, "--commit", r.commitA, "--title", "W1 work", "--body", body, "--key", "k")
	}
	if res := post("reviewer,reviewer-2"); res.code != 0 {
		t.Fatalf("round: exit %d\n%s", res.code, res.err)
	}
	first := ids(r.requests(t, "W1"))[0]
	if res := post("reviewer-3,reviewer-4"); res.code != 2 || !strings.Contains(res.err, `key "k/reviewer" was used by #`+first+" with different content; nothing written") {
		t.Fatalf("the key for another panel: exit %d\n%s", res.code, res.err)
	}
	if res := post("reviewer-3"); res.code != 2 || !strings.Contains(res.err, `key "k" was used by round #`+first+"; nothing written") {
		t.Fatalf("the key for one request: exit %d\n%s", res.code, res.err)
	}
	if n := len(r.requests(t, "W1")); n != 2 {
		t.Fatalf("%d requests, want 2", n)
	}
}

// reviewer-2's #355: the operator's own wait counts as a listener, and a key
// that only begins as a round's key is another key, both ways round.
func TestH34p7TheOperatorsWaitAndKeysThatShareAPrefix(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	dir := filepath.Join(r.dir, ".handoff")
	lock, err := wait.Listen(dir, "operator", "wait")
	if err != nil {
		t.Fatal(err)
	}
	old := `{"v":1,"pid":4343,"since":"2026-11-22T00:00:00Z","how":"wait"}` + "\n"
	if err := os.WriteFile(wait.PresencePath(dir, "operator"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := r.postRound(t, nil, "W1", r.commitA); res.code != 2 || !strings.Contains(res.err, "an older handoff listens for operator (pid 4343)") {
		t.Fatalf("a round with the operator's older wait: exit %d\n%s", res.code, res.err)
	}
	lock.Close()

	body := r.body(t, "b")
	single := func(item, key string) result {
		return r.run(t, "implementer", "", nil, "post", "--item", item, "--to", "reviewer", "--commit", r.commitA, "--title", item, "--body", body, "--key", key)
	}
	round := func(item, key string) result {
		return r.run(t, "implementer", "", nil, "post", "--item", item, "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", item, "--body", body, "--key", key)
	}
	for _, step := range []struct {
		name string
		res  func() result
	}{
		{"a request keyed release/checks", func() result { return single("W5", "release/checks") }},
		{"then a round keyed release", func() result { return round("W6", "release") }},
		{"a round keyed rel2", func() result { return round("W7", "rel2") }},
		{"then a request keyed rel2/checks", func() result { return single("W8", "rel2/checks") }},
	} {
		if res := step.res(); res.code != 0 {
			t.Fatalf("%s: exit %d\n%s", step.name, res.code, res.err)
		}
	}
	// A request keyed as one of a round's would be is still refused.
	if res := single("W9", "rel2/reviewer"); res.code != 2 || !strings.Contains(res.err, `key "rel2/reviewer" was used by #`) {
		t.Fatalf("a request keyed as a round's request: exit %d\n%s", res.code, res.err)
	}
}
