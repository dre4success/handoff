// Package status computes, from the record alone, the state of each request
// and what is waiting for each role. It describes what happened; it never
// chooses the next piece of work (proposal §4).
package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// Operator is the built-in role for the operator.
const Operator = "operator"

type State string

const (
	Open             State = "open"
	Claimed          State = "claimed"
	Answered         State = "answered" // QUESTION or CHANGES; the requester owes a response
	Superseded       State = "superseded"
	AwaitingOperator State = "awaiting operator"
	Decided          State = "decided"
)

// Request is a request entry and what has happened to it since.
type Request struct {
	Entry        record.Entry
	State        State
	Verdict      *record.Entry // the latest reply from the addressee while the request was live
	SupersededBy int64
	Decision     *record.Entry
	Last         record.Entry // the latest entry that moved the request or showed life on it
}

func (r *Request) live() bool { return r.State != Superseded && r.State != Decided }

// Target is something the operator can decide: an approved request, or an entry
// addressed to the operator.
type Target struct {
	Entry    record.Entry
	Request  *Request // nil when the target is a note
	Decision *record.Entry
}

type Status struct {
	Requests []*Request
	requests map[int64]*Request
	replies  map[int64]*Request // reply id -> its request
	operator []*Target          // entries addressed to the operator, in order
}

// Compute replays the entries in order.
func Compute(entries []record.Entry) *Status {
	s := &Status{requests: map[int64]*Request{}, replies: map[int64]*Request{}}
	for i := range entries {
		e := entries[i]
		switch e.Kind {
		case "request":
			r := &Request{Entry: e, State: Open, Last: e}
			if old, ok := s.requests[e.Supersedes]; ok && old.live() {
				old.State, old.SupersededBy = Superseded, e.ID
			}
			s.Requests = append(s.Requests, r)
			s.requests[e.ID] = r
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
			}
			if e.To == Operator {
				s.operator = append(s.operator, &Target{Entry: e})
			}
		case "decision":
			t, err := s.Decidable(e.Re)
			if err != nil {
				continue
			}
			t.Decision = &entries[i]
			if t.Request != nil {
				t.Request.State, t.Request.Decision = Decided, &entries[i]
			}
			for _, o := range s.operator {
				if o.Entry.ID == t.Entry.ID {
					o.Decision = &entries[i]
				}
			}
		}
	}
	return s
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
	return in
}

// Waiting reports whether the role has something to act on. Items awaiting
// the operator count only for the operator: an agent cannot act on them.
func (in Inbox) Waiting() bool {
	if in.Role == Operator {
		return len(in.AwaitingOperator) > 0
	}
	return len(in.NeedsReview)+len(in.BeingReviewed)+len(in.NeedsResponse) > 0
}

// Owed reports whether, at now, an agent owes a move on a live request whose
// last entry is less than within old: a verdict from its addressee, or a
// response from its requester. What waits for the operator is owed by no agent.
func (s *Status) Owed(now time.Time, within time.Duration) bool {
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
			out = append(out, Alert{Decide, fmt.Sprintf("decide-%d-%d", id, r.Verdict.ID),
				fmt.Sprintf("%s: %s approved #%d. Your decision.", item, r.Entry.To, id), id})
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
			text = fmt.Sprintf("%s: %s hasn't picked up #%d (%s)", item, r.Entry.To, id, age(gone))
		case Claimed:
			text = fmt.Sprintf("%s: %s claimed #%d and has been quiet for %s", item, r.Entry.To, id, age(gone))
		case Answered:
			text = fmt.Sprintf("%s: %s hasn't answered the %s on #%d (%s)", item, r.Entry.From, r.Verdict.Verdict, id, age(gone))
		}
		out = append(out, Alert{Quiet, fmt.Sprintf("quiet-%d-%d", id, r.Last.ID), text, 0})
	}
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

func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h", int(d.Hours()))
}

// Summary is one line describing an entry, for wait and inbox.
func Summary(e record.Entry) string {
	var what string
	switch e.Kind {
	case "request":
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
	return fmt.Sprintf("#%d %s %s %s → %s: %s", e.ID, e.Kind, orDash(e.Item), e.From, to, what)
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
