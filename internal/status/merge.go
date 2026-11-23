package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// Approval is work approved and not yet merged, for its requester to merge
// (row 108): the requests on one item, from one requester, at one commit,
// approved by the operator's decision or under the go-ahead. It is the
// requester's latest request on the item, and none of them is still in
// review, so an approval the work has gone on from is never listed. It ends
// when its commit is in main, which only git can say and the caller checks,
// when the requester records it merged, or when the operator holds it.
type Approval struct {
	Item, From, Commit string
	Requests           []*Request // in order
	// By is what approved it last: a decision, or the entry that completed
	// it under the go-ahead. Its time starts the quiet clock.
	By *record.Entry
	// Decisions are the operator's decisions that approved some of it, and
	// GoAhead the go-ahead that approved the rest, or nil.
	Decisions []*record.Entry
	GoAhead   *record.Entry
	// Hold is the operator's hold on it, for Holds.
	Hold *record.Entry
	// ToldSession is the session the fyi that told it named, if one did:
	// the role's cursor passing it says nothing of a session given the role
	// since (reviewer-2, #485).
	ToldSession string
	// Told is the entry that told the requester, or 0 when none did, as when
	// no session had its role as the go-ahead approved it.
	Told int64
	// Err is git's error when it can't tell whether the commit is in main:
	// the approval stays listed, and is owed by no one. The caller sets it.
	Err error
}

// approved says a request was approved: by the operator's decision, or under
// the go-ahead.
func approved(r *Request) bool {
	return r.State == Decided && (r.GoAhead != nil || (r.Decision != nil && r.Decision.Decision == "approve"))
}

// decidedAt is the id of the entry that decided r, or 0 while it is live.
func decidedAt(r *Request) int64 {
	switch {
	case r.Decision != nil:
		return r.Decision.ID
	case r.GoAhead != nil:
		return r.CoveredAt
	}
	return 0
}

// askOf is the ask a request was posted in: its round, or itself. The
// members of one round are one ask, however late a cut round was completed;
// a request posted alone is an ask of its own (the reviewer and reviewer-2,
// #485, #487).
func askOf(r *Request) int64 {
	if r.Round != nil {
		return r.Round.ID
	}
	return r.Entry.ID
}

// endedAt is when a request ended: its decision, the request that
// superseded it, or its withdrawal; 0 while it is live.
func endedAt(r *Request) int64 {
	switch r.State {
	case Superseded:
		return r.SupersededBy
	case Withdrawn:
		return r.WithdrawnBy.ID
	}
	return decidedAt(r)
}

// askEnded says whether every request of an ask had ended before the entry
// id: a round is one thing, so it ends when its last request does (the
// reviewer and reviewer-2, #528, #529).
func askEnded(ask []*Request, before int64) bool {
	for _, r := range ask {
		if e := endedAt(r); e == 0 || e >= before {
			return false
		}
	}
	return true
}

// approvalAt is the approval, as the record stands, of the requests from one
// requester on one item at one commit, or nil when there is none to merge.
// Each request belongs to an ask, its round or itself, and the latest ask at
// the commit leads. An older ask counts with it, whole, unless every request
// of it had ended before the latest was posted: so separate posts to two
// reviewers at one commit are weighed together, whichever was posted first,
// a round's denial counts while any of the round is still open, and a re-ask
// after a closure starts afresh (the reviewer and reviewer-2, #506, #508,
// #528, #529).
//   - A request there still in review means none yet.
//   - Every request that counts must be approved, with none denied, merged
//     or held, and in a round that is complete and sound: a round's panel is
//     approved whole, as the go-ahead approves it (row 106).
//   - One that counts, superseded by a request at another commit, makes it
//     stale: the work went on, as when the next post replaces the request of
//     a reviewer who asked for changes. One superseded by a request at the
//     same commit was asked again there, and is passed over (reviewer-2,
//     #467, #485).
func (s *Status) approvalAt(item, from, commit string) *Approval {
	asks := map[int64][]*Request{}
	var order []int64
	var latest int64
	for _, r := range s.Requests {
		if r.Entry.Item != item || r.Entry.From != from || r.Entry.Commit != commit {
			continue
		}
		if r.live() {
			return nil // still in review
		}
		a := askOf(r)
		if asks[a] == nil {
			order = append(order, a)
		}
		asks[a] = append(asks[a], r)
		latest = max(latest, a)
	}
	if latest == 0 {
		return nil
	}
	var counts []*Request
	for _, a := range order {
		if a != latest && askEnded(asks[a], latest) {
			continue // ended, every request of it, before the latest: history
		}
		for _, r := range asks[a] {
			if r.State == Withdrawn {
				// Never approved, and asks nothing now: the rest of its ask
				// is the approval, as the operator decided it. The go-ahead
				// approves none of it (roundBlocks; row 118).
				continue
			}
			if r.State == Superseded {
				if by, ok := s.requests[r.SupersededBy]; ok && by.Entry.Commit == commit {
					continue // asked again at this commit
				}
				return nil // the work went on
			}
			counts = append(counts, r)
		}
	}
	ap := &Approval{Item: item, From: from, Commit: commit}
	for _, r := range counts {
		if !approved(r) || r.Merged != nil || r.Held != nil {
			return nil
		}
		if rd := r.Round; rd != nil && (rd.Pending() || rd.Broken()) {
			return nil
		}
		ap.Requests = append(ap.Requests, r)
		by := r.Decision
		if r.GoAhead != nil {
			by, ap.GoAhead = r.CoveredBy, r.GoAhead
		} else {
			ap.Decisions = append(ap.Decisions, r.Decision)
		}
		if by != nil && (ap.By == nil || by.ID > ap.By.ID) {
			ap.By = by
		}
	}
	if ap.By == nil {
		return nil
	}
	// What told the requester: the decision or APPROVE that approved it last,
	// addressed to it; or, when the go-ahead itself approved it, the fyi the
	// away command sent the session that had its role then (reviewer-2,
	// #467, #485).
	switch {
	case ap.By.To == from:
		ap.Told = ap.By.ID
	case ap.By.Kind == "away":
		f := s.fyis[fyiKey{from, ap.By.ID}]
		ap.Told, ap.ToldSession = f.id, f.session
	}
	return ap
}

// fyiKey names the operator's fyi to a role about an entry, such as a
// go-ahead.
type fyiKey struct {
	to string
	re int64
}

// fyiTold is that fyi: its id, and the session it named.
type fyiTold struct {
	id      int64
	session string
}

// closed finds the approval e closes, its request named by e.Re, as the
// record stands when e is replayed: the exact requests of that approval, and
// no earlier one at the same commit (reviewer-2, #467).
func (s *Status) closed(e *record.Entry) *Approval {
	r, ok := s.requests[e.Re]
	if !ok {
		return nil
	}
	ap := s.approvalAt(r.Entry.Item, r.Entry.From, r.Entry.Commit)
	if ap == nil {
		return nil
	}
	for _, m := range ap.Requests {
		if m == r {
			return ap
		}
	}
	return nil
}

// hold is the operator's deny on a request approved and not merged: it holds
// every request of its approval (row 108). Anything else is recorded and
// changes nothing.
func (s *Status) hold(e *record.Entry) {
	if e.From != Operator || e.Decision != "deny" {
		return
	}
	if ap := s.closed(e); ap != nil {
		for _, m := range ap.Requests {
			m.Held = e
		}
	}
}

// merged is the requester's record that it merged an approval whose commit
// changed, by a squash or a rebase (row 108).
func (s *Status) merged(e *record.Entry) {
	if ap := s.closed(e); ap != nil && e.From == ap.From {
		for _, m := range ap.Requests {
			m.Merged = e
		}
	}
}

// Approvals lists the approvals the record doesn't show ended, approved
// after since, the record's last id when tracking began, so older ones are
// history (row 108). Whether each commit is in main is the caller's to ask
// git.
func (s *Status) Approvals(since int64) []*Approval {
	type at struct{ item, from, commit string }
	seen := map[at]bool{}
	var out []*Approval
	for _, r := range s.Requests {
		k := at{r.Entry.Item, r.Entry.From, r.Entry.Commit}
		if seen[k] {
			continue
		}
		seen[k] = true
		if ap := s.approvalAt(k.item, k.from, k.commit); ap != nil && ap.By.ID > since {
			out = append(out, ap)
		}
	}
	return out
}

// Holds lists the work the operator held, one approval for each hold made
// after since, with Hold set: for the caller to ask git whether its commit
// reached main anyway, which the operator is told (the reviewer, #468).
func (s *Status) Holds(since int64) []*Approval {
	var out []*Approval
	byHold := map[int64]*Approval{}
	for _, r := range s.Requests {
		h := r.Held
		if h == nil || h.ID <= since || s.approvedSince(r, h.ID) {
			continue // asked again at its commit and approved since: it may be merged
		}
		ap, ok := byHold[h.ID]
		if !ok {
			ap = &Approval{Item: r.Entry.Item, From: r.Entry.From, Commit: r.Entry.Commit, By: h, Hold: h}
			byHold[h.ID] = ap
			out = append(out, ap)
		}
		ap.Requests = append(ap.Requests, r)
	}
	return out
}

// SetHeldInMain records the held work whose commit git shows in main.
func (s *Status) SetHeldInMain(as []*Approval) { s.heldInMain = as }

// HeldInMain is what SetHeldInMain recorded.
func (s *Status) HeldInMain() []*Approval { return s.heldInMain }

// heldAlerts tell the operator once that work they held is in main all the
// same: the case a hold exists for (the reviewer, #468).
func (s *Status) heldAlerts() []Alert {
	var out []Alert
	for _, ap := range s.heldInMain {
		out = append(out, Alert{HeldInMain, fmt.Sprintf("held-in-main-%d", ap.Hold.ID),
			fmt.Sprintf("%s: you held %s's work @ %.12s (#%d), and its commit is in main", orDash(ap.Item), ap.From, ap.Commit, ap.Hold.ID), 0})
	}
	return out
}

// SetToMerge records the approvals git doesn't show in main, for Inbox, Owed
// and Alerts: status never runs git (row 108).
func (s *Status) SetToMerge(as []*Approval) { s.toMerge = as }

// ToMerge is what SetToMerge recorded.
func (s *Status) ToMerge() []*Approval { return s.toMerge }

// Reviewers are the roles whose approvals approved it, in order.
func (ap *Approval) Reviewers() []string {
	var out []string
	for _, r := range ap.Requests {
		out = append(out, r.Entry.To)
	}
	return out
}

// Line describes it, as the requester's inbox and the operator's view list
// it: "H1 @ 1a2b3c4d5e6f, approved under the operator's go-ahead #9 on the
// APPROVE of reviewer and reviewer-2 (#4, #5)".
func (ap *Approval) Line() string {
	var how []string
	if len(ap.Decisions) > 0 {
		var ids []string
		for _, d := range ap.Decisions {
			ids = append(ids, fmt.Sprintf("#%d", d.ID))
		}
		how = append(how, fmt.Sprintf("by the operator (%s)", strings.Join(ids, ", ")))
	}
	if ap.GoAhead != nil {
		how = append(how, fmt.Sprintf("under the operator's go-ahead #%d", ap.GoAhead.ID))
	}
	var reqs []string
	for _, r := range ap.Requests {
		reqs = append(reqs, fmt.Sprintf("#%d", r.Entry.ID))
	}
	return fmt.Sprintf("%s @ %.12s, approved %s on the APPROVE of %s (%s)",
		orDash(ap.Item), ap.Commit, strings.Join(how, " and "), and(ap.Reviewers()), strings.Join(reqs, ", "))
}

// since is when it was approved, from By's time.
func (ap *Approval) since() (time.Time, error) { return time.Parse(time.RFC3339, ap.By.TS) }

// owedMerges says whether a merge git can see isn't done was approved less
// than within ago, for Owed.
func (s *Status) owedMerges(now time.Time, within time.Duration) bool {
	for _, ap := range s.toMerge {
		if t, err := ap.since(); ap.Err == nil && err == nil && now.Sub(t) < within {
			return true
		}
	}
	return false
}

// mergeAlerts are the quiet alerts for merges owed longer than quiet. One
// that git can't tell about raises none (row 108).
func (s *Status) mergeAlerts(now time.Time, quiet time.Duration) []Alert {
	var out []Alert
	for _, ap := range s.toMerge {
		t, err := ap.since()
		if ap.Err != nil || err != nil || now.Sub(t) < quiet {
			continue
		}
		out = append(out, Alert{Quiet, fmt.Sprintf("merge-%d-%d", ap.Requests[0].Entry.ID, ap.By.ID),
			fmt.Sprintf("%s: %s hasn't merged %s (%s)%s", orDash(ap.Item), ap.From, strings.TrimPrefix(ap.Line(), orDash(ap.Item)+" @ "), age(now.Sub(t)), s.why(ap.From)), 0})
	}
	return out
}

// workAt names a requester's work on an item at a commit.
type workAt struct{ item, from, commit string }

// noteAuthorized notes, as the record is replayed, when r's work at its
// commit is approved whole by approvalAt's rule as the record stands then:
// so an approval stays a fact about the past, and a request opened later
// doesn't take it back (reviewer-2 and the reviewer, #542, #543).
func (s *Status) noteAuthorized(r *Request) {
	k := workAt{r.Entry.Item, r.Entry.From, r.Entry.Commit}
	if ap := s.approvalAt(k.item, k.from, k.commit); ap != nil && ap.By.ID > s.authorized[k] {
		s.authorized[k] = ap.By.ID
	}
}

// approvedSince says whether r's work, at its commit, was approved anew,
// whole, after the entry id, as the record stood when it was; or recorded
// merged after it (reviewer-2 and the reviewer, #528, #529, #542, #543).
func (s *Status) approvedSince(r *Request, id int64) bool {
	if s.authorized[workAt{r.Entry.Item, r.Entry.From, r.Entry.Commit}] > id {
		return true
	}
	for _, o := range s.Requests {
		if o.Entry.Item == r.Entry.Item && o.Entry.From == r.Entry.From && o.Entry.Commit == r.Entry.Commit && o.Merged != nil && o.Merged.ID > id {
			return true
		}
	}
	return false
}
