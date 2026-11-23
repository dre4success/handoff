// Package status computes, from the record alone, the state of each request
// and what is waiting for each role. It describes what happened; it never
// chooses the next piece of work (proposal §4).
package status

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// Operator is the built-in role for the operator.
const Operator = "operator"

// Spare is the built-in role of a session that holds no work: it waits to
// take over a stuck seat while the operator is away (H35). Like operator,
// it isn't listed in the config, and nothing is sent to it.
const Spare = "spare"

type State string

const (
	Open             State = "open"
	Claimed          State = "claimed"
	Answered         State = "answered" // QUESTION or CHANGES; the requester owes a response
	Superseded       State = "superseded"
	AwaitingOperator State = "awaiting operator"
	Decided          State = "decided"
	// Withdrawn: its requester withdrew it before any approval, as when
	// its item was merged or dropped (row 118). Nothing is owed on it.
	Withdrawn State = "withdrawn"
)

// Request is a request entry and what has happened to it since.
type Request struct {
	Entry        record.Entry
	State        State
	Verdict      *record.Entry // the latest reply from the addressee while the request was live
	SupersededBy int64
	Decision     *record.Entry
	// GoAhead is the operator's away entry this request was approved under,
	// with no decision of its own, and CoveredAt the entry that completed
	// it: an APPROVE, or the away entry itself (row 96).
	GoAhead   *record.Entry
	CoveredAt int64
	CoveredBy *record.Entry // the entry CoveredAt names
	Last      record.Entry  // the latest entry that moved the request or showed life on it
	Round     *Round        // the round it is in, for a request posted to several reviewers at once
	// Merged is the requester's merged entry, and Held the operator's hold,
	// that ended the request's approval before git saw its commit in main
	// (row 108).
	Merged, Held *record.Entry
	// WithdrawnBy is the requester's withdraw entry that ended it (row 118).
	WithdrawnBy *record.Entry
}

// Round is the requests one post sent to several reviewers (row 106). It is
// complete once every role on its panel has its request, each matching the
// round's item, requester, commit and panel. Only then do its members
// supersede the requests they replace, all together, so a write cut short
// leaves neither a fragment of the new round nor what is left of the old one
// able to approve the item under the go-ahead.
type Round struct {
	ID       int64 // the id of its first request
	Item     string
	From     string
	Commit   string
	Panel    []string
	Members  map[string]*Request // by the role each asks
	Replaces map[string]int64    // by role: the request each member replaces once the round is complete, as its first request records
	Complete bool
	broken   bool // a member that doesn't match the round: it never completes
}

// MissingText says what a pending round lacks, such as "the round #12 to
// reviewer, reviewer-2 and reviewer-3 has no request to reviewer-3".
func (rd *Round) MissingText() string {
	if rd.broken {
		return fmt.Sprintf("the round #%d's requests don't match each other, which posting again can't repair", rd.ID)
	}
	if missing := rd.Missing(); len(missing) > 0 {
		return fmt.Sprintf("the round #%d to %s has no request to %s", rd.ID, list(rd.Panel, "and"), list(missing, "or"))
	}
	return fmt.Sprintf("the round #%d to %s doesn't match its own requests", rd.ID, list(rd.Panel, "and"))
}

// list joins names as "a, b and c".
func list(names []string, conj string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " " + conj + " " + names[len(names)-1]
}

// Missing lists, in panel order, the roles that have no request in the round.
func (rd *Round) Missing() []string {
	var out []string
	for _, role := range rd.Panel {
		if rd.Members[role] == nil {
			out = append(out, role)
		}
	}
	return out
}

// Pending says whether the round is incomplete: then nothing on its item is
// approved under the go-ahead, whatever is decided on its requests, until
// the same post completes it. A broken round stays pending: a retry can't
// repair it, and only the operator can sort it out.
func (rd *Round) Pending() bool { return !rd.Complete }

// Broken says the round's requests don't match each other, which only an
// edit of the record could do.
func (rd *Round) Broken() bool { return rd.broken }

// shrunk says a request of a complete round was denied while another is
// still live: the go-ahead then approves nothing on its item, since the
// panel the round asked would be smaller than it was.
func (rd *Round) shrunk() bool {
	denied, live := false, false
	for _, m := range rd.Members {
		denied = denied || (m.State == Decided && m.Decision != nil && m.Decision.Decision == "deny")
		live = live || m.live()
	}
	return rd.Complete && denied && live
}

func (r *Request) live() bool {
	return r.State != Superseded && r.State != Decided && r.State != Withdrawn
}

// Withdrawable says whether r's requester may withdraw it now: while it is
// open, claimed or answered with CHANGES or QUESTION. Once approved, it is
// the operator's to decide (row 118).
func (r *Request) Withdrawable() bool {
	return r.State == Open || r.State == Claimed || r.State == Answered
}

// Target is something the operator can decide: an approved request, or an entry
// addressed to the operator.
type Target struct {
	Entry    record.Entry
	Request  *Request // nil when the target is a note
	Decision *record.Entry
}

type Status struct {
	Requests []*Request
	Rounds   []*Round
	// Away is the operator's go-ahead while away: the latest away entry with
	// no back after it, or nil. AwaySince is the first away entry since the
	// last back, where this spell away began (row 96).
	Away, AwaySince *record.Entry
	requests        map[int64]*Request
	rounds          map[int64]*Round
	replies         map[int64]*Request       // reply id -> its request
	operator        []*Target                // entries addressed to the operator, in order
	toMerge         []*Approval              // approvals git doesn't show in main, as the caller set them (row 108)
	fyis            map[fyiKey]fyiTold       // the operator's first fyi to a role about an entry
	authorized      map[workAt]int64         // the latest entry that approved work whole, as the record stood then (row 108)
	heldInMain      []*Approval              // held work git shows in main, as the caller set it (row 108)
	lastTo          map[string]int64         // the latest entry addressed to each role
	lastFrom        map[string]record.Entry  // the latest entry from each role (H35)
	answeredBy      map[int64][]string       // the roles that wrote an entry naming each id with re (H35)
	addressed       map[string][]addressedTo // the entries addressed to each role, in order
	more            []Alert                  // alerts from outside the record, as the caller set them (row 110)
	delivery        map[string]Delivery      // how each role's session stands, as the caller set it (row 113)
	trackedFrom     int64                    // the id tracking began after, as the caller set it (row 108)
}

// Compute replays the entries in order.
func Compute(entries []record.Entry) *Status {
	s := &Status{requests: map[int64]*Request{}, rounds: map[int64]*Round{}, replies: map[int64]*Request{}, fyis: map[fyiKey]fyiTold{}, authorized: map[workAt]int64{}, lastTo: map[string]int64{}, lastFrom: map[string]record.Entry{}, answeredBy: map[int64][]string{}, addressed: map[string][]addressedTo{}}
	for i := range entries {
		e := entries[i]
		s.lastFrom[e.From] = e
		if e.Re != 0 {
			s.answeredBy[e.Re] = append(s.answeredBy[e.Re], e.From)
		}
		if e.To != "" {
			s.lastTo[e.To] = e.ID
			s.addressed[e.To] = append(s.addressed[e.To], addressedTo{e.ID, e.Kind, e.Session, &entries[i]})
		}
		switch e.Kind {
		case "request", "round":
			// A round's requests have a kind of their own, so a handoff
			// older than row 106 counts none of them, and approves nothing
			// on a part of a panel.
			r := &Request{Entry: e, State: Open, Last: e}
			s.Requests = append(s.Requests, r)
			s.requests[e.ID] = r
			if e.Kind == "round" {
				s.join(r)
			} else {
				s.supersede(e.Supersedes, e.ID)
			}
		case "claim":
			// claim is idempotent per role: a restarted session that claims
			// again writes nothing, so only the first claim is in the record.
			if r, ok := s.requests[e.Re]; ok && e.From == r.Entry.To && r.State == Open {
				r.State, r.Last = Claimed, e
			}
		case "reply":
			r, ok := s.requests[e.Re]
			if !ok || e.From != r.Entry.To || !r.live() {
				continue // recorded, changes nothing
			}
			s.replies[e.ID] = r
			r.Verdict, r.Last = &entries[i], e
			if e.Verdict == "APPROVE" {
				r.State = AwaitingOperator
				if s.Away != nil {
					s.cover(r.Entry.Item, &entries[i])
				}
			} else {
				r.State = Answered
			}
		case "note":
			// A note answers a QUESTION or CHANGES only when it links the
			// standing reply, comes from the requester and goes to the
			// reviewer. A linked note to anyone else, an escalation to the
			// operator for one, leaves the answer still owed.
			if r, ok := s.replies[e.Re]; ok && r.State == Answered && r.Verdict.ID == e.Re &&
				e.From == r.Entry.From && e.To == r.Entry.To {
				r.State = Claimed // answered; the review is with the reviewer again
				r.Last = e
				// Its request carries it now: owed while the review is,
				// done with once that is (reviewer-3, #877).
				s.answeredBy[e.ID] = append(s.answeredBy[e.ID], e.To)
			}
			if e.To == Operator {
				s.operator = append(s.operator, &Target{Entry: e})
			}
		case "away":
			// Only the operator's command writes these; an entry of the kind
			// from anyone else is recorded and changes nothing.
			if e.From != Operator {
				continue
			}
			if s.Away == nil {
				s.AwaySince = &entries[i]
			}
			s.Away = &entries[i]
			// What is approved already is approved under the go-ahead now.
			for _, item := range s.awaitingItems() {
				s.cover(item, &entries[i])
			}
		case "back":
			if e.From == Operator {
				s.Away, s.AwaySince = nil, nil
			}
		case "decision":
			t, err := s.Decidable(e.Re)
			if err != nil {
				// A deny on a request approved and not merged holds it
				// (row 108).
				s.hold(&entries[i])
				continue
			}
			t.Decision = &entries[i]
			if t.Request != nil {
				t.Request.State, t.Request.Decision = Decided, &entries[i]
				s.noteAuthorized(t.Request)
			}
			for _, o := range s.operator {
				if o.Entry.ID == t.Entry.ID {
					o.Decision = &entries[i]
				}
			}
		case "merged":
			s.merged(&entries[i])
		case "withdraw":
			// Only the requester withdraws, and only before an approval
			// (row 118); anything else is recorded and changes nothing.
			if r, ok := s.requests[e.Re]; ok && e.From == r.Entry.From && r.Withdrawable() {
				r.State, r.WithdrawnBy, r.Last = Withdrawn, &entries[i], e
			}
		case "fyi":
			if k := (fyiKey{e.To, e.Re}); e.From == Operator && e.Re != 0 && s.fyis[k].id == 0 {
				s.fyis[k] = fyiTold{e.ID, e.Session}
			}
		}
	}
	return s
}

// supersede marks old superseded by the request by, if old is a live request.
func (s *Status) supersede(old, by int64) {
	if o, ok := s.requests[old]; ok && old != 0 && o.live() {
		o.State, o.SupersededBy = Superseded, by
	}
}

// join adds a request to its round. A member that doesn't match the round,
// one for a role off its panel or already in it, or a round whose first
// request isn't its own id, breaks the round: it never completes, so nothing
// on it is approved under the go-ahead (row 106).
func (s *Status) join(r *Request) {
	e := r.Entry
	id := e.Round
	if id == 0 {
		id = e.ID // a round's request with no round: a round of its own, broken
	}
	rd, ok := s.rounds[id]
	if !ok {
		// The round's first request records the whole round: its panel and
		// what each of its requests replaces (row 106).
		replaces := maps.Clone(e.Replaces)
		if replaces == nil {
			replaces = map[string]int64{}
		}
		rd = &Round{ID: id, Item: e.Item, From: e.From, Commit: e.Commit, Panel: e.Panel,
			Members: map[string]*Request{}, Replaces: replaces, broken: e.Round != e.ID || len(e.Panel) == 0}
		s.rounds[id] = rd
		s.Rounds = append(s.Rounds, rd)
	}
	r.Round = rd
	if rd.Complete || e.Item != rd.Item || e.From != rd.From || e.Commit != rd.Commit ||
		!slices.Equal(e.Panel, rd.Panel) || !slices.Contains(rd.Panel, e.To) || rd.Members[e.To] != nil ||
		!sameReplaces(e.Replaces, rd.Replaces) || e.Supersedes != rd.Replaces[e.To] {
		// A stray request, even after the round completed, breaks it, and
		// a broken round is never complete, so it keeps the go-ahead off
		// its item (reviewer-3, #328).
		rd.broken, rd.Complete = true, false
		return
	}
	rd.Members[e.To] = r
	if !rd.broken && len(rd.Members) == len(rd.Panel) {
		rd.Complete = true
		for _, role := range rd.Panel {
			s.supersede(rd.Replaces[role], rd.Members[role].Entry.ID)
		}
	}
}

// roundBlocks says whether a round on item keeps the go-ahead from approving
// it: one incomplete, or one whose panel a denial shrank. So does a request
// withdrawn there, while its reviewer isn't asked again (row 118).
func (s *Status) roundBlocks(item string) bool {
	for _, rd := range s.Rounds {
		if rd.Item == item && (rd.Pending() || rd.shrunk()) {
			return true
		}
	}
	return len(s.withdrawnOn(item)) > 0
}

// withdrawnOn lists the requests withdrawn on item whose reviewer hasn't
// been asked again there since, by a later request from the same
// requester. Each keeps the go-ahead off the item, as a denial in a round
// does (row 106): withdrawing a request that asked for changes must not
// leave the rest to approve the item without the operator (row 118).
func (s *Status) withdrawnOn(item string) []*Request {
	var out []*Request
	for i, r := range s.Requests {
		if r.Entry.Item != item || r.State != Withdrawn {
			continue
		}
		again := false
		for _, o := range s.Requests[i+1:] {
			again = again || o.Entry.Item == item && o.Entry.From == r.Entry.From && o.Entry.To == r.Entry.To
		}
		if !again {
			out = append(out, r)
		}
	}
	return out
}

// sameReplaces compares two rounds' maps of what they replace, nil and
// empty alike.
func sameReplaces(a, b map[string]int64) bool {
	return len(a) == len(b) && maps.Equal(a, b) || len(a) == 0 && len(b) == 0
}

// PendingRound returns a pending round on item, from from if from isn't "".
func (s *Status) PendingRound(item, from string) *Round {
	for _, rd := range s.Rounds {
		if rd.Item == item && (from == "" || rd.From == from) && rd.Pending() {
			return rd
		}
	}
	return nil
}

// cover approves an item under the operator's go-ahead when every live
// request on it is approved, all at one commit: two reviewers asked means
// both approve the commit that is merged. Otherwise the item waits as it
// would without the go-ahead (row 96).
func (s *Status) cover(item string, by *record.Entry) {
	live := s.liveOn(item)
	if len(live) == 0 || s.roundBlocks(item) {
		return
	}
	for _, r := range live {
		if r.State != AwaitingOperator || r.Entry.Commit != live[0].Entry.Commit {
			return
		}
	}
	for _, r := range live {
		r.State, r.GoAhead, r.CoveredAt, r.CoveredBy = Decided, s.Away, by.ID, by
	}
	for _, r := range live {
		s.noteAuthorized(r)
	}
}

// liveOn lists the live requests on an item.
func (s *Status) liveOn(item string) []*Request {
	var live []*Request
	for _, r := range s.Requests {
		if r.Entry.Item == item && r.live() {
			live = append(live, r)
		}
	}
	return live
}

// awaitingItems lists, once each and in order, the items with a request
// awaiting the operator.
func (s *Status) awaitingItems() []string {
	var items []string
	seen := map[string]bool{}
	for _, r := range s.Requests {
		if r.State == AwaitingOperator && !seen[r.Entry.Item] {
			seen[r.Entry.Item] = true
			items = append(items, r.Entry.Item)
		}
	}
	return items
}

// stillReviewed reports whether a live request on the item still waits for
// a verdict or for its requester's response, so that the go-ahead may yet
// cover the item.
func (s *Status) stillReviewed(item string) bool {
	for _, r := range s.liveOn(item) {
		if r.State == Open || r.State == Claimed || r.State == Answered {
			return true
		}
	}
	return false
}

// UnderGoAhead returns the go-ahead an APPROVE's request was approved under,
// if it was, and this reply is the one that stands on it.
func (s *Status) UnderGoAhead(e record.Entry) (*record.Entry, bool) {
	r, ok := s.replies[e.ID]
	if !ok || r.GoAhead == nil || r.Verdict == nil || r.Verdict.ID != e.ID {
		return nil, false
	}
	return r.GoAhead, true
}

// HeldBy is the operator's hold on the request a reply stands on, or nil.
func (s *Status) HeldBy(e record.Entry) *record.Entry {
	if r, ok := s.replies[e.ID]; ok {
		return r.Held
	}
	return nil
}

// GoAheadNote is the line wait and show add to an APPROVE whose item is
// approved under the operator's go-ahead, or to the operator's fyi about a
// go-ahead that approved items the moment it was given; else "".
func (s *Status) GoAheadNote(e record.Entry) string {
	if e.Kind == "fyi" && e.From == Operator && e.Re != 0 {
		// Each item as it stands now: one held since says so, and the
		// others are still the operator's approve (reviewer-2, #485).
		// Each approval it made, by item and requester: a hold is on one
		// requester's work, and another's on the item stands (reviewer-2,
		// #506).
		type work struct{ item, from string }
		var all []work
		requesters := map[string]map[string]bool{}
		for _, r := range s.Requests {
			if r.CoveredAt != e.Re || r.GoAhead == nil || r.GoAhead.ID != e.Re {
				continue
			}
			w := work{r.Entry.Item, r.Entry.From}
			if requesters[w.item] == nil {
				requesters[w.item] = map[string]bool{}
			}
			if !requesters[w.item][w.from] {
				requesters[w.item][w.from] = true
				all = append(all, w)
			}
		}
		var items, held []string
		for _, w := range all {
			name := orDash(w.item)
			if len(requesters[w.item]) > 1 {
				name = fmt.Sprintf("%s (%s's)", name, w.from)
			}
			if h := s.heldOn(w.item, w.from, e.Re); h != nil {
				held = append(held, fmt.Sprintf("%s, then held by the operator (#%d)", name, h.ID))
			} else {
				items = append(items, name)
			}
		}
		var parts []string
		if len(items) > 0 {
			parts = append(parts, fmt.Sprintf("%s approved under the operator's go-ahead #%d as it was given: that is the operator's approve", strings.Join(items, ", "), e.Re))
		}
		if len(held) > 0 {
			parts = append(parts, fmt.Sprintf("approved under the operator's go-ahead #%d as it was given, and held since, so not to merge: %s", e.Re, strings.Join(held, "; ")))
		}
		return strings.Join(parts, "; ")
	}
	g, ok := s.UnderGoAhead(e)
	if !ok {
		return ""
	}
	if h := s.HeldBy(e); h != nil {
		// Held since: no longer the operator's approve (reviewer-3, #475).
		return fmt.Sprintf("%s was approved under the operator's go-ahead #%d, then held by the operator (#%d): don't merge it", orDash(e.Item), g.ID, h.ID)
	}
	return fmt.Sprintf("%s is approved under the operator's go-ahead #%d: that is the operator's approve", orDash(e.Item), g.ID)
}

// ApprovedSince lists the requests approved under a go-ahead from the away
// entry since on, in order.
func (s *Status) ApprovedSince(since int64) []*Request {
	var out []*Request
	for _, r := range s.Requests {
		if r.GoAhead != nil && r.GoAhead.ID >= since {
			out = append(out, r)
		}
	}
	return out
}

func (s *Status) Request(id int64) (*Request, bool) {
	r, ok := s.requests[id]
	return r, ok
}

// Live returns the request on item from one role to another that is neither
// superseded nor decided. A new request on the same item and pair must name
// it with --supersedes.
func (s *Status) Live(item, from, to string) (*Request, bool) {
	for i := len(s.Requests) - 1; i >= 0; i-- {
		r := s.Requests[i]
		if r.Entry.Item == item && r.Entry.From == from && r.Entry.To == to && r.live() {
			return r, true
		}
	}
	return nil, false
}

// ThreadOpen reports whether an item still has a live request, or has none
// at all. Duplicates are refused only within an open thread.
func (s *Status) ThreadOpen(item string) bool {
	seen := false
	for _, r := range s.Requests {
		if r.Entry.Item == item {
			seen = true
			if r.live() {
				return true
			}
		}
	}
	return !seen
}

// Decidable returns what a decision on id would decide: a request awaiting
// the operator (by its own id or its APPROVE's), or an undecided entry
// addressed to the operator.
func (s *Status) Decidable(id int64) (*Target, error) {
	r, ok := s.requests[id]
	if !ok {
		r, ok = s.replies[id]
		if ok && (r.Verdict == nil || r.Verdict.ID != id) {
			return nil, fmt.Errorf("#%d is not the reply that stands on request #%d", id, r.Entry.ID)
		}
	}
	if ok && r.State == AwaitingOperator {
		return &Target{Entry: r.Entry, Request: r}, nil
	}
	for _, o := range s.operator {
		if o.Entry.ID == id && o.Decision == nil {
			return o, nil
		}
	}
	if ok {
		return nil, fmt.Errorf("request #%d is %s, not awaiting the operator", r.Entry.ID, r.State)
	}
	return nil, fmt.Errorf("#%d is not awaiting the operator", id)
}

// Inbox is the four lists of proposal §6. Everything open is listed,
// whatever any cursor says.
type Inbox struct {
	Role             string
	NeedsReview      []*Request
	BeingReviewed    []*Request
	NeedsResponse    []*Request
	AwaitingOperator []*Target
	// Incomplete is the role's own rounds that a cut write left without
	// every request: the role posts the same again to complete each (row 106).
	Incomplete []*Round
	// ToMerge is the role's own approvals not yet merged, as SetToMerge
	// recorded them (row 108).
	ToMerge []*Approval
}

func (s *Status) Inbox(role string) Inbox {
	in := Inbox{Role: role}
	for _, r := range s.Requests {
		switch {
		case r.Entry.To == role && r.State == Open:
			in.NeedsReview = append(in.NeedsReview, r)
		case r.Entry.To == role && r.State == Claimed:
			in.BeingReviewed = append(in.BeingReviewed, r)
		case r.Entry.From == role && r.State == Answered:
			in.NeedsResponse = append(in.NeedsResponse, r)
		}
		if r.State == AwaitingOperator {
			in.AwaitingOperator = append(in.AwaitingOperator, &Target{Entry: r.Entry, Request: r})
		}
	}
	for _, o := range s.operator {
		if o.Decision == nil {
			in.AwaitingOperator = append(in.AwaitingOperator, o)
		}
	}
	for _, rd := range s.Rounds {
		if rd.From == role && rd.Pending() {
			in.Incomplete = append(in.Incomplete, rd)
		}
	}
	for _, ap := range s.toMerge {
		if ap.From == role {
			in.ToMerge = append(in.ToMerge, ap)
		}
	}
	return in
}

// Waiting reports whether the role has something to act on. Items awaiting
// the operator count only for the operator: an agent cannot act on them.
func (in Inbox) Waiting() bool {
	if in.Role == Operator {
		return len(in.AwaitingOperator) > 0
	}
	return len(in.NeedsReview)+len(in.BeingReviewed)+len(in.NeedsResponse)+len(in.Incomplete)+len(in.ToMerge) > 0
}

// Owed reports whether, at now, an agent owes a move on a live request whose
// last entry is less than within old: a verdict from its addressee, or a
// response from its requester. What waits for the operator is owed by no agent.
func (s *Status) Owed(now time.Time, within time.Duration) bool {
	// An approval not yet in main is owed by its requester (row 108).
	if s.owedMerges(now, within) {
		return true
	}
	// A pending round is owed by its requester, who posts it again (row 106).
	for _, rd := range s.Rounds {
		if first := s.requests[rd.ID]; rd.Pending() && first != nil {
			if since, err := time.Parse(time.RFC3339, first.Entry.TS); err == nil && now.Sub(since) < within {
				return true
			}
		}
	}
	for _, r := range s.Requests {
		if r.State != Open && r.State != Claimed && r.State != Answered {
			continue
		}
		if since, err := time.Parse(time.RFC3339, r.Last.TS); err == nil && now.Sub(since) < within {
			return true
		}
	}
	return false
}

// AlertKind says why the operator hears about something.
type AlertKind string

const (
	Quiet  AlertKind = "quiet"  // a move has been owed for too long
	Decide AlertKind = "decide" // waiting for the operator
	// Trouble: handoff itself needs the operator, such as a session that cannot
	// listen for its work (proposal §17 row 67).
	Trouble AlertKind = "trouble"
	// GoAhead: what the operator's go-ahead does while they are away, for
	// them to see, with nothing to decide: that it was recorded, and each item
	// approved under it, with who approved (row 98).
	GoAhead AlertKind = "go-ahead"
	// HeldInMain: work the operator held is in main all the same (row 108).
	HeldInMain AlertKind = "held-in-main"
	// Blocked: a role's session can't take its work, and work waits for it
	// (row 110).
	Blocked AlertKind = "blocked"
)

// Alert is something the operator should hear about. The key names the exact
// situation, so it is sent once; when the work moves on, the key changes.
type Alert struct {
	Kind   AlertKind
	Key    string
	Text   string
	Target int64 // for a decide alert, the entry the operator decides
}

// Alerts lists, at now, what the operator should hear about: each live request whose
// next move has been owed for at least quiet, and everything awaiting the
// operator. It only reads; sending, and sending once, are the caller's.
//
// Who owes the move follows the state: an open or claimed request owes a
// verdict from its addressee, an answered one a response from its requester.
// The clock starts at the request's last entry: the request, the claim, the
// verdict, or the note that answered it. Only an entry restarts it. A session
// that resumes work writes nothing until its next move, so it may be reported
// quiet while it works; telling a live session from a stuck one is presence's
// job, not the record's (proposal §17 row 62).
func (s *Status) Alerts(now time.Time, quiet time.Duration) []Alert {
	var out []Alert
	for _, r := range s.Requests {
		item, id := orDash(r.Entry.Item), r.Entry.ID
		if r.State == AwaitingOperator {
			switch {
			case s.Away == nil:
				text := fmt.Sprintf("%s: %s approved #%d%s. Your decision.", item, r.Entry.To, id, s.othersOn(r))
				if rd := r.Round; rd != nil && rd.Pending() {
					text = fmt.Sprintf("%s: %s approved #%d%s, but %s. Your decision.", item, r.Entry.To, id, s.othersOn(r), rd.MissingText())
				}
				out = append(out, Alert{Decide, fmt.Sprintf("decide-%d-%d", id, r.Verdict.ID), text, id})
			case !s.stillReviewed(r.Entry.Item):
				// Away, an approval waits quietly while the item's other
				// requests are reviewed. Once none is, and the go-ahead still
				// doesn't cover the item, the approvals name different
				// commits, or a round on it is incomplete (row 106), and
				// only the operator can decide (row 96).
				why := fmt.Sprintf("the approvals on %s name different commits", item)
				for _, w := range s.withdrawnOn(r.Entry.Item) {
					why = fmt.Sprintf("%s withdrew its request #%d to %s", w.Entry.From, w.Entry.ID, w.Entry.To)
				}
				for _, rd := range s.Rounds {
					switch {
					case rd.Item != r.Entry.Item:
					case rd.Pending():
						why = rd.MissingText()
					case rd.shrunk():
						why = fmt.Sprintf("you denied a request of the round #%d, which leaves its panel smaller than it asked", rd.ID)
					}
				}
				out = append(out, Alert{Decide, fmt.Sprintf("decide-%d-%d", id, r.Verdict.ID),
					fmt.Sprintf("%s: %s approved #%d, but %s, so your go-ahead doesn't cover it. Your decision.", item, r.Entry.To, id, why), id})
			}
			continue
		}
		if !r.live() {
			continue
		}
		since, err := time.Parse(time.RFC3339, r.Last.TS)
		gone := now.Sub(since)
		if err != nil || gone < quiet {
			continue
		}
		var text string
		switch r.State {
		case Open:
			text = fmt.Sprintf("%s: %s hasn't picked up #%d (%s)%s", item, r.Entry.To, id, age(gone), s.why(r.Entry.To))
		case Claimed:
			text = fmt.Sprintf("%s: %s claimed #%d and has been quiet for %s%s", item, r.Entry.To, id, age(gone), s.why(r.Entry.To))
		case Answered:
			text = fmt.Sprintf("%s: %s hasn't answered the %s on #%d (%s)%s", item, r.Entry.From, r.Verdict.Verdict, id, age(gone), s.why(r.Entry.From))
		}
		out = append(out, Alert{Quiet, fmt.Sprintf("quiet-%d-%d", id, r.Last.ID), text, 0})
	}
	// A round a cut write left incomplete is the requester's to complete, by
	// posting the same again; nothing else says it is owed (row 106).
	for _, rd := range s.Rounds {
		if !rd.Pending() {
			continue
		}
		first := s.requests[rd.ID]
		for _, m := range rd.Members {
			if first == nil || m.Entry.ID < first.Entry.ID {
				first = m
			}
		}
		if first == nil {
			continue
		}
		since, err := time.Parse(time.RFC3339, first.Entry.TS)
		if gone := now.Sub(since); err == nil && gone >= quiet {
			next, why := fmt.Sprintf("%s can post the same again to complete it", rd.From), s.why(rd.From)
			if rd.broken {
				// The operator's to sort out, not the requester's, so the
				// requester's delivery is no reason (reviewer-3, #709).
				next, why = "only you can sort it out", ""
			}
			out = append(out, Alert{Quiet, fmt.Sprintf("round-%d-%d", rd.ID, len(rd.Members)),
				fmt.Sprintf("%s: %s; %s (%s)%s", orDash(rd.Item), rd.MissingText(), next, age(gone), why), 0})
		}
	}
	out = append(out, s.undeliveredAlerts(now, quiet)...)
	out = append(out, s.mergeAlerts(now, quiet)...)
	out = append(out, s.heldAlerts()...)
	out = append(out, s.more...)
	out = append(out, s.goAheadAlerts()...)
	for _, o := range s.operator {
		if o.Decision == nil {
			what := firstLine(o.Entry.Title)
			if what == "" {
				what = firstLine(o.Entry.Body)
			}
			out = append(out, Alert{Decide, fmt.Sprintf("decide-note-%d", o.Entry.ID),
				fmt.Sprintf("%s: %s asks you: %s", orDash(o.Entry.Item), o.Entry.From, what), o.Entry.ID})
		}
	}
	return out
}

// goAheadAlerts are the go-ahead's notifications, while it stands: one that
// it is recorded, and one for each item approved under it, naming the
// reviewers whose approvals did it. They are sent once each, as every alert
// is, from the command that wrote the entry.
func (s *Status) goAheadAlerts() []Alert {
	g := s.Away
	if g == nil {
		return nil
	}
	out := []Alert{{GoAhead, fmt.Sprintf("away-%d", g.ID),
		fmt.Sprintf("Your go-ahead is recorded (#%d): %s Until you run handoff back, items are approved without asking you.", g.ID, firstLine(g.Body)), 0}}
	type group struct {
		item, key       string
		ahead           int64
		reviewers, reqs []string
	}
	var groups []*group
	byKey := map[string]*group{}
	for _, r := range s.ApprovedSince(s.AwaySince.ID) {
		// Keys are file names of digits, letters a to z and dashes, so an
		// item's name, such as W1, is not in one; the entry that completed
		// it and the item's first request name the approval.
		at := fmt.Sprintf("%d-%s", r.CoveredAt, r.Entry.Item)
		gr, ok := byKey[at]
		if !ok {
			gr = &group{item: orDash(r.Entry.Item), key: fmt.Sprintf("go-ahead-%d-%d", r.CoveredAt, r.Entry.ID), ahead: r.GoAhead.ID}
			byKey[at] = gr
			groups = append(groups, gr)
		}
		gr.reviewers = append(gr.reviewers, r.Entry.To)
		gr.reqs = append(gr.reqs, fmt.Sprintf("#%d", r.Entry.ID))
	}
	for _, gr := range groups {
		out = append(out, Alert{GoAhead, gr.key, fmt.Sprintf("%s: approved under your go-ahead #%d, by %s (%s).",
			gr.item, gr.ahead, and(gr.reviewers), strings.Join(gr.reqs, ", ")), 0})
	}
	return out
}

// and joins names as a sentence does: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h", int(d.Hours()))
}

// KindName is how a kind is shown. A round's request is stored as kind
// round, so that a handoff older than row 106 counts none of them, but it is
// a request to review, and shown as one: the round line of show says the rest.
func KindName(kind string) string {
	if kind == "round" {
		return "request"
	}
	return kind
}

// Summary is one line describing an entry, for wait and inbox.
func Summary(e record.Entry) string {
	var what string
	switch e.Kind {
	case "request", "round":
		what = fmt.Sprintf("%s @ %.12s", e.Title, e.Commit)
		if e.Supersedes != 0 {
			what += fmt.Sprintf(" (supersedes #%d)", e.Supersedes)
		}
	case "reply":
		what = fmt.Sprintf("%s on #%d @ %.12s", e.Verdict, e.Re, e.Commit)
	case "decision":
		what = fmt.Sprintf("%s #%d", e.Decision, e.Re)
	case "claim":
		what = fmt.Sprintf("claimed #%d", e.Re)
	case "withdraw":
		what = fmt.Sprintf("withdrew #%d", e.Re)
		if b := firstLine(e.Body); b != "" {
			what += ": " + b
		}
	default:
		what = firstLine(e.Title)
		if what == "" {
			what = firstLine(e.Body)
		}
		if e.Re != 0 {
			what += fmt.Sprintf(" (re #%d)", e.Re)
		}
	}
	to := e.To
	if to == "" {
		to = "-"
	}
	return fmt.Sprintf("#%d %s %s %s → %s: %s", e.ID, KindName(e.Kind), orDash(e.Item), e.From, to, what)
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(s); len(r) > 80 {
		s = string(r[:79]) + "…"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// heldOn is the hold on a request of item, from requester from, covered by
// the go-ahead entry, or nil.
func (s *Status) heldOn(item, from string, goAhead int64) *record.Entry {
	for _, r := range s.Requests {
		if r.Entry.Item == item && r.Entry.From == from && r.CoveredAt == goAhead && r.Held != nil {
			return r.Held
		}
	}
	return nil
}

// LastFor is the id of the latest entry addressed to role, or 0.
func (s *Status) LastFor(role string) int64 { return s.lastTo[role] }

// LastFrom is the latest entry from role, if any (H35: what was last heard
// from a seat).
func (s *Status) LastFrom(role string) (record.Entry, bool) {
	e, ok := s.lastFrom[role]
	return e, ok
}

// SetMore records alerts the caller knows of from outside the record, such
// as a blocked session's (row 110), for Alerts to list.
func (s *Status) SetMore(as []Alert) { s.more = as }

// addressedTo is an entry addressed to a role: its id, kind, and the session
// an fyi named.
type addressedTo struct {
	id      int64
	kind    string
	session string
	entry   *record.Entry
}

// WaitsFor is the latest of what waits for role now, or 0 when nothing
// does (row 110): an entry addressed to it past its cursor that its session
// would be given, a note, a decision or an fyi as much as a request, and
// what its inbox says it owes, a cut round to post again or approved work to
// merge among them. An fyi naming a session other than session is never
// given to it, so it waits for no one, unless it names one the session
// went on from, by /background, as delivery takes it (reviewer-2, #521).
// APPROVEs a listener holds back at now wait for no one, until the first of
// their holds ends, when all of them go, as the listener gives them in one
// wake (rows 110, 111; reviewer-2, #603).
func (s *Status) WaitsFor(role string, cursor int64, session string, was []string, now time.Time) int64 {
	var last int64
	for _, e := range s.Waiting(role, cursor, session, was, now) {
		last = max(last, e.ID)
	}
	return last
}

// Waiting is each entry WaitsFor counts for role, once, oldest first: for a
// request owed, its latest entry, which starts its clock, as the quiet alert
// takes it. H35's stuck check reads how long the oldest has waited, and the
// spare's wake names them.
func (s *Status) Waiting(role string, cursor int64, session string, was []string, now time.Time) []record.Entry {
	var out, held []record.Entry
	release := false
	for _, a := range s.addressed[role] {
		if a.id <= cursor || a.kind == "fyi" && a.session != "" && a.session != session && !slices.Contains(was, a.session) {
			continue
		}
		if until, partial := s.PartialApprove(*a.entry); partial {
			if now.Before(until) {
				held = append(held, *a.entry)
				continue
			}
			release = true
		}
		out = append(out, *a.entry)
	}
	if release {
		out = append(out, held...)
	}
	in := s.Inbox(role)
	for _, rs := range [][]*Request{in.NeedsReview, in.BeingReviewed, in.NeedsResponse} {
		for _, r := range rs {
			out = append(out, r.Last)
		}
	}
	for _, rd := range in.Incomplete {
		for _, m := range rd.Members {
			out = append(out, m.Entry)
		}
	}
	for _, ap := range in.ToMerge {
		out = append(out, *ap.By)
	}
	slices.SortFunc(out, func(a, b record.Entry) int { return cmp.Compare(a.ID, b.ID) })
	return slices.CompactFunc(out, func(a, b record.Entry) bool { return a.ID == b.ID })
}

// OwedBy is the part of Waiting that is work for role now: what its inbox
// says it owes (a review to do, a response, a round to post again, work to
// merge), each by its latest entry, and each note to it past its cursor. A
// request, reply or decision addressed to it past its cursor that is done
// with isn't work: a review answered with show and reply, and approved
// since, stays past the cursor of a reviewer that never listened
// (reviewer-2, #853). H35's stuck check reads this, so a seat isn't taken,
// and the spare used, for work that is over. Oldest first.
func (s *Status) OwedBy(role string, cursor int64) []record.Entry {
	var out []record.Entry
	for _, a := range s.addressed[role] {
		// A note the role has answered, with an entry naming it by re, is
		// done with, though past a cursor that never moved (the reviewer,
		// #865); so is one that answered a CHANGES or QUESTION, which its
		// request carries, in BeingReviewed while the review is owed
		// (reviewer-3, #877).
		if a.id > cursor && a.kind == "note" && !slices.Contains(s.answeredBy[a.id], role) {
			out = append(out, *a.entry)
		}
	}
	in := s.Inbox(role)
	// A broken round only the operator can sort out is owed by no one,
	// whichever seat its members are with (reviewer-2, #862).
	for _, rs := range [][]*Request{in.NeedsReview, in.BeingReviewed, in.NeedsResponse} {
		for _, r := range rs {
			if r.Round == nil || !r.Round.Broken() {
				out = append(out, r.Last)
			}
		}
	}
	// Nor is a broken round its requester's to post again, nor work git
	// can't tell about (rows 106, 108; reviewer-3, #855).
	for _, rd := range in.Incomplete {
		if rd.Broken() {
			continue
		}
		for _, m := range rd.Members {
			out = append(out, m.Entry)
		}
	}
	for _, ap := range in.ToMerge {
		if ap.Err == nil {
			out = append(out, *ap.By)
		}
	}
	slices.SortFunc(out, func(a, b record.Entry) int { return cmp.Compare(a.ID, b.ID) })
	return slices.CompactFunc(out, func(a, b record.Entry) bool { return a.ID == b.ID })
}

// HoldFor is how long an APPROVE waits, at most, before its requester is
// woken for it while the rest of its item is still reviewed (row 111).
const HoldFor = 30 * time.Minute

// HeldApprove says whether a listener holds an APPROVE back from its
// requester now, and until when (row 111): it is the standing verdict on a
// live request awaiting the operator, while another live request on the
// item at the same commit still owes a verdict or a response, the test the
// go-ahead uses (cover, stillReviewed), for at most HoldFor from the
// APPROVE's own time. Read from the record as it stands at every change and
// tick, never decided once: a round that completes, the other request
// replaced at another commit, or a decision on this one ends it. An APPROVE
// on a request already superseded or decided is no such verdict, and goes
// at once.
func (s *Status) HeldApprove(e record.Entry, now time.Time) (time.Time, bool) {
	until, ok := s.PartialApprove(e)
	return until, ok && now.Before(until)
}

// PartialApprove is HeldApprove with no clock: whether e is such an APPROVE,
// and when its hold ends, passed or not.
func (s *Status) PartialApprove(e record.Entry) (time.Time, bool) {
	if e.Kind != "reply" || e.Verdict != "APPROVE" {
		return time.Time{}, false
	}
	r, ok := s.replies[e.ID]
	if !ok || r.State != AwaitingOperator || r.Verdict == nil || r.Verdict.ID != e.ID {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, e.TS)
	if err != nil {
		return time.Time{}, false
	}
	until := at.Add(HoldFor)
	// The go-ahead's own test: every live request on the item at one
	// commit. With mixed commits, nothing is held (reviewer-2 and the
	// reviewer, #535, #536).
	live := s.liveOn(r.Entry.Item)
	for _, o := range live {
		if o.Entry.Commit != r.Entry.Commit {
			return time.Time{}, false
		}
	}
	for _, o := range live {
		if o != r && (o.State == Open || o.State == Claimed || o.State == Answered) {
			return until, true
		}
	}
	return time.Time{}, false
}
