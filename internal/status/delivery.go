package status

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// H34 part 5 (row 113): the quiet alert says why work waits.

// Delivery is how a role's session stands, for the operator's alerts: its
// cursor, its session and the ones it went on from by /background, and why
// work waits for it, as a clause such as ", and nothing is listening for it
// (last listened 40 minutes ago)", or "" when a listener is there.
type Delivery struct {
	Cursor  int64
	Session string
	Was     []string
	Why     string
	// Blocked: the session is blocked (row 110), so the block's own alert
	// names each entry that waits for it as it arrives.
	Blocked bool
	// Listening: something listens for the role now, so what waits is
	// delivered within a second.
	Listening bool
}

// SetDelivery records how each role's session stands, from outside the
// record, and the id tracking began after (row 108), for Alerts: each quiet
// alert then says why its move waits, and a note or an fyi left undelivered
// for quiet is one.
func (s *Status) SetDelivery(d map[string]Delivery, tracked int64) {
	s.delivery, s.trackedFrom = d, tracked
}

// why is the clause a quiet alert owed by role ends with, or "".
func (s *Status) why(role string) string { return s.delivery[role].Why }

// undeliveredAlerts are the quiet alerts for a note, or an fyi for the
// role's session, that has waited at least quiet without being delivered:
// past the role's cursor, and after tracking began, so an old unread one
// doesn't alert the day this ships. One per role, for the oldest; the next
// comes once that is delivered. None for a blocked role, whose block's alert
// already named it, so one problem makes one alert (the reviewer, #645); and
// none for a role something listens for, which is being given it, as when a
// session's own listener starts (#673). It is for a role nothing listens for. A reply is left out, since its request's
// own alert covers it, a held APPROVE among them, and so is an fyi naming a
// session the role's isn't, which can never be delivered to it (row 93).
func (s *Status) undeliveredAlerts(now time.Time, quiet time.Duration) []Alert {
	var out []Alert
	roles := make([]string, 0, len(s.delivery))
	for role := range s.delivery {
		roles = append(roles, role)
	}
	slices.Sort(roles)
	for _, role := range roles {
		d := s.delivery[role]
		if d.Blocked || d.Listening {
			continue
		}
		for _, a := range s.addressed[role] {
			e := a.entry
			if e.ID <= d.Cursor || e.ID <= s.trackedFrom || e.Kind != "note" && e.Kind != "fyi" {
				continue
			}
			if e.Kind == "fyi" && e.Session != "" && e.Session != d.Session && !slices.Contains(d.Was, e.Session) {
				continue
			}
			since, err := time.Parse(time.RFC3339, e.TS)
			if err != nil || now.Sub(since) < quiet {
				break // the oldest is too new, and so is every later one
			}
			text := fmt.Sprintf("%s: %s hasn't been given the note #%d from %s (%s)%s", orDash(e.Item), role, e.ID, e.From, age(now.Sub(since)), d.Why)
			if e.Kind == "fyi" {
				text = fmt.Sprintf("%s hasn't been given your fyi #%d (%s)%s", role, e.ID, age(now.Sub(since)), d.Why)
			}
			out = append(out, Alert{Quiet, fmt.Sprintf("undelivered-%d", e.ID), text, 0})
			break
		}
	}
	return out
}

// OthersOn says, for the operator's decision on r, where the other live
// requests on its item stand: "reviewer-2 asked for changes on #552,
// reviewer-3 hasn't picked up #553", or "". So the alert, the pop-up and the
// terminal, each naming one request, show what approving it alone leaves
// (row 113). A request with no item shares it with nothing.
func (s *Status) OthersOn(r *Request) string {
	if r == nil || r.Entry.Item == "" {
		return ""
	}
	var parts []string
	for _, o := range s.liveOn(r.Entry.Item) {
		if o == r {
			continue
		}
		switch o.State {
		case Open:
			parts = append(parts, fmt.Sprintf("%s hasn't picked up #%d", o.Entry.To, o.Entry.ID))
		case Claimed:
			parts = append(parts, fmt.Sprintf("%s is reviewing #%d", o.Entry.To, o.Entry.ID))
		case Answered:
			what := "asked for changes on"
			if o.Verdict != nil && o.Verdict.Verdict == "QUESTION" {
				what = "asked a question on"
			}
			parts = append(parts, fmt.Sprintf("%s %s #%d", o.Entry.To, what, o.Entry.ID))
		case AwaitingOperator:
			parts = append(parts, fmt.Sprintf("%s approved #%d", o.Entry.To, o.Entry.ID))
		}
	}
	// A reviewer dropped from the item by a withdrawal is named too: the
	// go-ahead doesn't approve without it, and the operator sees why (row
	// 118).
	for _, w := range s.withdrawnOn(r.Entry.Item) {
		parts = append(parts, fmt.Sprintf("%s withdrew its request #%d to %s", w.Entry.From, w.Entry.ID, w.Entry.To))
	}
	return strings.Join(parts, ", ")
}

// othersOn is OthersOn as the alert's text takes it, after the approval it
// names: "; reviewer-2 asked for changes on #552", or "".
func (s *Status) othersOn(r *Request) string {
	if o := s.OthersOn(r); o != "" {
		return "; " + o
	}
	return ""
}
