package main

import (
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// H34 part 5 (row 113): the quiet alert says why work waits.

// deliveries is how each role's session stands now, for the operator's
// alerts: its cursor, its session and why work waits for it. The delivery
// state is read before the cursor, as whenFyiGets reads them (#624): a
// listener that takes the lock, delivers and lets it go between the two
// reads has written the cursor by then, so what it delivered isn't taken for
// undelivered (reviewer-3, #709).
func (a *app) deliveries() map[string]status.Delivery {
	out := map[string]status.Delivery{}
	for _, r := range a.cfg.Roles {
		d, ok := a.deliveryTo(r.Name)
		testPause("delivery-" + r.Name) // between the two reads, for a test
		cur, _ := wait.ReadCursor(a.dir, r.Name)
		out[r.Name] = status.Delivery{Cursor: cur, Session: d.session, Was: d.was, Why: whyClause(d, ok), Blocked: ok && d.block != nil, Listening: ok && d.listening}
	}
	return out
}

// whyClause is what a quiet alert owed by a role ends with: its session
// can't be woken, or is at a permission prompt; nothing listens for it, and
// when something last did; or no session has the role. Nothing when a
// listener is there.
func whyClause(d delivery, ok bool) string {
	switch {
	case !ok:
		return ", and no session has the role"
	case d.block != nil && d.block.Reason == blockPermission:
		return ", and its session is " + d.block.why()
	case d.block != nil:
		return ", and its session can't be woken (" + d.block.why() + ")"
	case !d.listening:
		return ", and nothing is listening for it (" + lastListened(d.last) + ")"
	}
	return ""
}
