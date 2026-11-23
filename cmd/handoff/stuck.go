package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dre4success/handoff/internal/notify"
	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// H35 part 2 (row 116): while the operator is away with cover, a seat is
// stuck when work has waited for it for quiet_after and its session can't
// take it, by the design's table of signs; the spare's listener wakes the
// spare once for each (docs/proposals/H35_DESIGN.md, "When a seat is
// stuck").

// stuckSeat is a seat the spare may take: the role, why its session can't
// take its work, and what waits for it.
type stuckSeat struct {
	role  string
	why   string
	waits []int64
	// implements: the seat may be implementing, so its old session may
	// have left work it didn't commit (row 118).
	implements bool
}

// stuckSeats lists the stuck seats at now, in the config's order. None
// unless the operator is away with cover. For each seat with a session:
//   - work has waited for it for quiet_after, from the oldest such entry:
//     what it owes now (status.OwedBy), not all that is past its cursor, so a
//     review done with and approved isn't work (reviewer-2, #853). The
//     operator's fyis aren't work either: the away entry itself sends one to
//     every seat with a session, so a stuck seat with nothing else waiting
//     would be taken, and the spare used, for no work (the reviewer's note,
//     #602, asks the operator the same of a block's alert);
//   - nothing listens for it now, since a listener is given what waits
//     within a second;
//   - and one of the signs holds. The exact ones, at quiet_after, for any
//     seat: Antigravity's cap (part 2 of H34's block), or its session's
//     process ended, where the binding has a process to watch. The others,
//     at twice quiet_after, for a reviewer seat only: a permission prompt
//     (a block), or nothing heard, which is no listener, no entry from the
//     role and no command from its session for that long. Silence never
//     takes the implementer seat: a long turn looks the same (the reviewer,
//     #655).
//
// A seat no session has ever held isn't in the table: the operator is told
// of its work, as before.
func (a *app) stuckSeats(st *status.Status, now time.Time) []stuckSeat {
	if st.Away == nil || st.Away.Cover == nil {
		return nil
	}
	allReview := true
	for _, rc := range a.cfg.Roles {
		allReview = allReview && rc.Reviews
	}
	bs, _ := a.bindings()
	var out []stuckSeat
	for _, rc := range a.cfg.Roles {
		var b binding
		found := false
		for _, x := range bs {
			if x.Role == rc.Name {
				b, found = x, true
			}
		}
		if !found || wait.Listening(a.dir, rc.Name) {
			continue
		}
		cur, _ := wait.ReadCursor(a.dir, rc.Name)
		w := st.OwedBy(rc.Name, cur)
		if len(w) == 0 {
			continue
		}
		oldest, err := time.Parse(time.RFC3339, w[0].TS)
		if err != nil {
			continue
		}
		waited := now.Sub(oldest)
		// With every role reviewing, a seat may be implementing, so only
		// the exact signs take it.
		reviewer := rc.Reviews && !allReview
		var why string
		switch {
		case waited < a.quiet:
		case b.Block != nil && b.Block.Reason == blockCap:
			why = "its session can't be woken (" + b.Block.why() + ")"
		case b.ended():
			why = "its session's process has ended"
		case !reviewer || waited < 2*a.quiet:
		case b.Block != nil && b.Block.Reason == blockPermission:
			// The prompt itself has stood that long, not only the work: a
			// prompt the operator answers stays recorded through a long
			// turn, so a fresh one is no sign (reviewer-3, #855).
			if at, err := time.Parse(time.RFC3339Nano, b.Block.At); err == nil && now.Sub(at) >= 2*a.quiet {
				why = "its session is " + b.Block.why()
			}
		default:
			if heard := a.lastHeard(st, b); now.Sub(heard) >= 2*a.quiet {
				why = "nothing has been heard from it " + sinceHeard(heard, now)
			}
		}
		if why == "" {
			continue
		}
		s := stuckSeat{role: rc.Name, why: why, implements: !reviewer}
		for _, e := range w {
			s.waits = append(s.waits, e.ID)
		}
		out = append(out, s)
	}
	return out
}

// lastHeard is the last sign of life from b's seat: the session being given
// it, a listener taking the lock or letting it go, an entry from the role,
// or a command from its session, which raises its binding's answered (row
// 110).
func (a *app) lastHeard(st *status.Status, b binding) time.Time {
	last, _ := time.Parse(time.RFC3339Nano, b.Since)
	if d, ok := a.deliveryTo(b.Role); ok && d.last.After(last) {
		last = d.last
	}
	if e, ok := st.LastFrom(b.Role); ok {
		if t, err := time.Parse(time.RFC3339, e.TS); err == nil && t.After(last) {
			last = t
		}
	}
	if t, err := time.Parse(time.RFC3339Nano, b.Answered); err == nil && t.After(last) {
		last = t
	}
	return last
}

// sinceHeard says how long nothing has been heard, for the wake: "for 64
// minutes", or "since it was given the seat" when never.
func sinceHeard(t, now time.Time) string {
	switch d := now.Sub(t); {
	case t.IsZero():
		return "since it was given the seat"
	case d < time.Minute:
		return "for " + plural(int(d.Seconds()), "second")
	case d < 2*time.Hour:
		return "for " + plural(int(d.Minutes()), "minute")
	default:
		return "for " + plural(int(d.Hours()), "hour")
	}
}

// coverGone says why cover no longer stands, or "" while a running session
// holds the spare at the generation it names: "isn't running", or "is no
// longer the spare", as after a /clear of the spare cut short (the reviewer,
// #822, #854).
func (a *app) coverGone(cover *record.Cover) string {
	bs, _ := a.bindings()
	for _, b := range bs {
		if a.isSpare(b.Role) && b.Gen == cover.Gen && b.Lost == nil {
			if b.running() {
				return ""
			}
			return "isn't running"
		}
	}
	return "is no longer the spare"
}

// spareBlocked says, for handoff and away, why a blocked spare can't cover.
func spareBlocked(d delivery) string {
	if d.block.Reason == blockPermission {
		return fmt.Sprintf("the spare's session is %s, so it can't cover a stuck seat until you answer it", d.block.why())
	}
	return fmt.Sprintf("the spare can't be woken until you type in its session (%s), so it can't cover a stuck seat", d.block.why())
}

// spareWake is the Wake of the spare's listener, or nil for any other role:
// at each check, the first stuck seat it hasn't been woken for under this
// away entry, said once (H35, "The takeover", step 1). Only the spare the
// away entry names is woken, by its generation, which a move by /clear or
// /background keeps (reviewer-2, #727). Nothing is written to the record:
// the wake is like an alert, sent once by its key.
func (a *app) spareWake(role string, start binding) func(*status.Status) (string, func(func(string) error) error) {
	if !a.isSpare(role) {
		return nil
	}
	return func(st *status.Status) (string, func(func(string) error) error) {
		g := st.Away
		if g == nil || g.Cover == nil || g.Cover.Gen != start.Gen {
			return "", nil
		}
		for _, s := range a.stuckSeats(st, time.Now()) {
			key := fmt.Sprintf("spare-%d-%s", g.ID, s.role)
			done, err := notify.Path(a.dir, key)
			if err != nil || exists(done) {
				continue
			}
			seat := s.role
			// Under the listener's fence: checked again, the wake said as
			// it stands now, then marked sent (reviewer-2, #853, #862, #871).
			return spareWakeText(s), func(print func(string) error) error {
				now, ok := a.stillStuck(g.ID, start.Gen, seat)
				if !ok {
					return wait.ErrStale
				}
				if err := print(spareWakeText(now)); err != nil {
					return err
				}
				return notify.MarkSent(a.dir, key)
			}
		}
		return "", nil
	}
}

// stillStuck reads the record again, under the fence's lock, and gives the
// seat as it stands now, if the away entry the wake was chosen under still
// stands with cover for this spare and the seat is still stuck: the
// operator back, or the seat come back to life, drops the wake (reviewer-2,
// #862). So does a record broken or torn, which refuses every write, take's
// too: the wake stays unspent for after the repair (reviewer-2, #871).
func (a *app) stillStuck(away, gen int64, seat string) (stuckSeat, bool) {
	snap, err := a.store.Current()
	if err != nil || snap.Broken != nil || snap.Torn != nil {
		return stuckSeat{}, false
	}
	st := a.status(snap)
	if g := st.Away; g == nil || g.ID != away || g.Cover == nil || g.Cover.Gen != gen {
		return stuckSeat{}, false
	}
	for _, s := range a.stuckSeats(st, time.Now()) {
		if s.role == seat {
			return s, true
		}
	}
	return stuckSeat{}, false
}

// spareWakeText is what the spare is told of a stuck seat.
func spareWakeText(s stuckSeat) string {
	var ids []string
	for _, id := range s.waits {
		ids = append(ids, fmt.Sprintf("#%d", id))
	}
	verb := "wait"
	if len(ids) == 1 {
		verb = "waits"
	}
	text := fmt.Sprintf("%s's seat is stuck: %s. %s %s for it. Take it with `%s role %s`: the operator's cover is their word for it.",
		s.role, s.why, listOf(ids, "and"), verb, shellQuote(selfCommand()), s.role)
	if s.implements {
		// Work the old session didn't commit is in the working tree, and
		// only there (row 118).
		text += " Its old session may have left changes it didn't commit: don't discard, stash or overwrite them; tell the operator what you find."
	}
	return text
}

// ended says the session's process has ended, where the CLI gave its own
// process to watch, as Claude Code does: its pid is gone, or the binding was
// marked gone for that. A host process alone, a hook's parent, isn't the
// design's exact sign (reviewer-2, #853), so a Codex or Antigravity binding
// never ends this way.
func (b binding) ended() bool {
	return b.PID > 0 && !alive(b.PID) || b.Gone && b.CLI == "claude"
}

// exists says whether a file is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
