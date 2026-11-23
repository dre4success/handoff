package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// A block, kept in state, for any CLI (row 110). When handoff knows a
// session can't take its work, it writes a block into that session's own
// binding: what it saw, and when. Senders, the inbox, the brief and the
// operator see it; the record never holds it, since a block ends when the
// operator types, and the record can't take an entry back.

// The reasons a session can be blocked.
const (
	// blockCap: Antigravity lets hooks continue a turn 10 times in a row,
	// and ignores the next, so the session can't be woken until the
	// operator types in it (row 102).
	blockCap = "cap"
	// blockPermission: the session stopped at a permission prompt.
	blockPermission = "permission"
)

// block is a binding's Block.
type block struct {
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"` // what was seen, such as the prompt
	At     string `json:"at"`
}

// why says, for a line, what a block is and since when: "Antigravity's 10
// wakes used, since 00:01".
func (b *block) why() string {
	since := b.At
	if t, err := time.Parse(time.RFC3339Nano, b.At); err == nil {
		since = t.Local().Format("15:04")
	}
	switch b.Reason {
	case blockCap:
		return fmt.Sprintf("Antigravity's %d wakes used, since %s", agyContinuations, since)
	case blockPermission:
		what := "at a permission prompt"
		if b.Detail != "" {
			what += ": " + firstLine(b.Detail)
		}
		return fmt.Sprintf("%s, since %s", what, since)
	}
	return fmt.Sprintf("%s, since %s", b.Reason, since)
}

// listened notes that a listener of s took role's lock, at the generation
// it started from, and clears s's block: a session that listens again can
// take its work (row 110). A listener of another generation notes nothing.
func (a *app) listened(s session, start binding) {
	unlock, err := a.lockSessions()
	if err != nil {
		return
	}
	defer unlock()
	b, ok := a.readBinding(s.ID)
	if !ok || b.Role == "" || b.Role != start.Role || b.Gen != start.Gen {
		return
	}
	b.Listened, b.Block = time.Now().UTC().Format(time.RFC3339Nano), nil
	a.writeBinding(b)
}

// writeBlock writes a block into s's binding, fenced: only while it holds
// the role at the generation the hook started under, no listener holds the
// role's lock, the hook began after a listener of it last took the lock,
// and after the hook that wrote the block it holds, if any, began. So an old
// hook can't block a replacement, or the same session once it left the role
// and came back; a late hook from before the operator typed can't block a
// session that has listened since; and an older sighting can't replace a
// newer one (reviewer-2, #590). A block's At is when its hook began, as it
// saw it. It never creates a binding: a session with none can't hold a
// block. It returns the binding it wrote, so the caller acts on that block
// and no later one.
func (a *app) writeBlock(s session, start binding, began time.Time, reason, detail string) (blockWrite, binding) {
	unlock, err := a.lockSessions()
	if err != nil {
		return blockFailed, binding{}
	}
	defer unlock()
	b, ok := a.readBinding(s.ID)
	switch {
	case !ok || b.Role == "" || b.Role != start.Role || b.Gen != start.Gen:
		return blockStale, binding{}
	case wait.Listening(a.dir, b.Role):
		return blockStale, binding{}
	}
	if t, err := time.Parse(time.RFC3339Nano, b.Listened); err == nil && !began.After(t) {
		return blockStale, binding{}
	}
	// The newest sighting the binding knows of: the block it holds, and for
	// a permission report, the last command of the session (answered).
	var newest time.Time
	if b.Block != nil {
		newest, _ = time.Parse(time.RFC3339Nano, b.Block.At)
	}
	if t, err := time.Parse(time.RFC3339Nano, b.Answered); err == nil && reason == blockPermission && t.After(newest) {
		newest = t
	}
	if !newest.IsZero() && !began.After(newest) {
		return blockStale, binding{}
	}
	b.Block = &block{Reason: reason, Detail: detail, At: began.UTC().Format(time.RFC3339Nano)}
	if a.writeBinding(b) != nil {
		return blockFailed, binding{}
	}
	return blockWritten, b
}

// blockWrite is what writeBlock did.
type blockWrite int

const (
	blockWritten blockWrite = iota
	// blockStale: the fence refused it. What the hook saw is no longer so:
	// the role moved, or something listens for it, or the session has
	// listened since (reviewer-2 and reviewer-3, #494, #496).
	blockStale
	// blockFailed: the binding couldn't be written.
	blockFailed
)

// answered is run once a command from a session ends, whatever it did, and
// never for a hook or the operator's pop-up: the session ran it, so the
// operator had answered any permission prompt its hooks saw before the
// command opened the record (row 110; reviewer-3, #496; reviewer-2, #521,
// #590, #596, #610). Under the sessions' lock, if the binding is the one the
// command opened under, it clears a permission block from no later than
// that, and raises answered to it, so a report from no later, held before
// its write, can't block the session afterwards, whether or not a block was
// there to see. A report from after it is untouched. Antigravity's cap
// clears only when the session listens again.
func (a *app) answered() {
	if a.openedSession == "" {
		return
	}
	unlock, err := a.lockSessions()
	if err != nil {
		return
	}
	defer unlock()
	cur, ok := a.readBinding(a.openedSession)
	if !ok || cur.Role == "" || cur.Gen != a.openedGen {
		return
	}
	at := a.opened.UTC()
	if cur.Block != nil && cur.Block.Reason == blockPermission {
		if t, err := time.Parse(time.RFC3339Nano, cur.Block.At); err == nil && !t.After(at) {
			cur.Block = nil
		}
	}
	if t, err := time.Parse(time.RFC3339Nano, cur.Answered); err != nil || at.After(t) {
		cur.Answered = at.Format(time.RFC3339Nano)
	}
	a.writeBinding(cur)
}

// delivery is whether handoff can deliver to role now: blocked, with the
// block and the session; listening; or not listening now, with when it last
// listened. holder still says who has the role.
type delivery struct {
	session   string
	was       []string // the sessions it went on from, by /background
	block     *block
	listening bool
	last      time.Time // when a listener last took the lock, or zero
}

func (a *app) deliveryTo(role string) (delivery, bool) {
	bs, _ := a.bindings()
	var d delivery
	found := false
	for _, b := range bs {
		if b.Role != role || !b.running() {
			continue
		}
		found = true
		d.session, d.was, d.block = b.Session, b.Was, b.Block
		if t, err := time.Parse(time.RFC3339Nano, b.Listened); err == nil {
			d.last = t
		}
	}
	d.listening = wait.Listening(a.dir, role)
	if !found && !d.listening {
		return d, false
	}
	if p, err := wait.ReadPresence(a.dir, role); err == nil {
		for _, t := range []time.Time{p.Since, p.Ended} {
			if t.After(d.last) {
				d.last = t
			}
		}
	}
	return d, true
}

// lastListened says when, for a line: "last listened 3 minutes ago", or
// "last listened on 19 November".
func lastListened(t time.Time) string {
	switch d := time.Since(t); {
	case t.IsZero():
		return "never listened"
	case d < time.Minute:
		return "last listened just now"
	case d < time.Hour:
		return fmt.Sprintf("last listened %s ago", plural(int(d.Minutes()), "minute"))
	case d < 24*time.Hour:
		return fmt.Sprintf("last listened %s ago", plural(int(d.Hours()), "hour"))
	}
	return "last listened on " + t.Local().Format("2 January")
}

// blockedLine is what a sender, the inbox and the brief are told of a
// blocked role, or "".
func blockedLine(role string, d delivery) string {
	if d.block == nil {
		return ""
	}
	if d.block.Reason == blockPermission {
		return fmt.Sprintf("%s's session is %s. This waits until the operator answers it.", role, d.block.why())
	}
	return fmt.Sprintf("%s can't be woken until the operator types in its session (%s). This waits.", role, d.block.why())
}

// blockAlerts are the operator's alerts for blocked roles with work
// waiting: one when the block is written with work waiting, and again each
// time new work arrives for the role. Each is keyed by session, generation,
// block and the latest entry for the role, so it is sent once (row 110).
func (a *app) blockAlerts(st *status.Status) []status.Alert {
	var out []status.Alert
	bs, _ := a.bindings()
	for _, b := range bs {
		if al, ok := a.blockAlertOf(st, b); ok {
			out = append(out, al)
		}
	}
	return out
}

// blockAlertOf is one binding's block alert, if it is blocked and work
// waits for it. Its key names the exact session, generation, block and
// work: the session by a hash of its id, so no two share one (reviewer-2,
// #521).
func (a *app) blockAlertOf(st *status.Status, b binding) (status.Alert, bool) {
	if b.Block == nil || !b.running() {
		return status.Alert{}, false
	}
	cur, _ := wait.ReadCursor(a.dir, b.Role)
	last := st.WaitsFor(b.Role, cur, b.Session, b.Was, a.store.Now())
	if last == 0 {
		return status.Alert{}, false // nothing waits: a note waits as much as a request (#494, #495)
	}
	at, _ := time.Parse(time.RFC3339Nano, b.Block.At)
	sum := sha256.Sum256([]byte(b.Session))
	key := fmt.Sprintf("block-%s-%d-%d-%d", hex.EncodeToString(sum[:8]), b.Gen, at.UnixNano(), last)
	text := fmt.Sprintf("%s can't be woken until you type in its session (%s), and work waits for it (#%d).", b.Role, b.Block.why(), last)
	if b.Block.Reason == blockPermission {
		text = fmt.Sprintf("%s is %s, and work waits for it (#%d).", b.Role, b.Block.why(), last)
	}
	return status.Alert{Kind: status.Blocked, Key: key, Text: text}, true
}

// workWaits says whether anything waits for role's session now.
func (a *app) workWaits(role, session string, was []string) bool {
	snap, err := a.store.Load()
	if err != nil {
		return false
	}
	cur, _ := wait.ReadCursor(a.dir, role)
	return a.status(snap).WaitsFor(role, cur, session, was, a.store.Now()) != 0
}

// blockAlertNow is the alert of the block in b, the binding a hook wrote,
// with the work waiting as the record stands now, if any: a prompt's hook
// sends it as its notice, so what the notice says is what its key marks
// sent, and never a later block's or later work's (reviewer-2, #521, #590,
// #596).
func (a *app) blockAlertNow(b binding) (status.Alert, bool) {
	snap, err := a.store.Load()
	if err != nil {
		return status.Alert{}, false
	}
	return a.blockAlertOf(a.status(snap), b)
}

// plural is "1 minute" or "3 minutes".
func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// whenFyiGets is the receipt's line for the fyi e, or "" (row 112): only
// for a session it is for, its own or one that went on from it by
// /background, as delivery takes it, and only while it is still to be
// delivered. The delivery state is read before the cursor, so an fyi
// delivered in between gets no line (reviewer-2, #624).
func (a *app) whenFyiGets(e record.Entry) string {
	d, ok := a.deliveryTo(e.To)
	if !ok || e.Session != "" && d.session != e.Session && !slices.Contains(d.was, e.Session) {
		return "" // the role moved to a session the fyi isn't for
	}
	if cur, _ := wait.ReadCursor(a.dir, e.To); cur >= e.ID {
		return "" // delivered already
	}
	return whenItGets(e.To, d)
}

// whenItGets says, after an fyi, when a session told that can't take it now
// gets it, or "" (row 112): blocked, or not listening now.
func whenItGets(role string, d delivery) string {
	switch {
	case d.block != nil && d.block.Reason == blockPermission:
		return fmt.Sprintf("%s's session is at a permission prompt; it gets this once you answer it.", role)
	case d.block != nil:
		return fmt.Sprintf("%s's session can't be woken; it gets this when you type in it. To reach the role now, give it to another session and send again.", role)
	case !d.listening:
		return fmt.Sprintf("%s's session isn't listening now; it gets this the next time it listens.", role)
	}
	return ""
}
