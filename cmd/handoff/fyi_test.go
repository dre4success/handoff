package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/wait"
)

// H26 (row 92): the operator tells every agent with a session the same thing
// at once, with the message as the one argument. H27 (row 94) named it fyi.

// An fyi reaches each role with a session now, in one write, and no role
// without one. Each listener brings it as it brings a note, and it asks
// nothing of anyone: no inbox, no alert.
func TestH26FyiReachesEverySessionAtOnce(t *testing.T) {
	r := newRepo(t)
	sent, notify := sink(t)
	impl := claude("s-impl", cli(t))
	r.as(t, impl, "role", "implementer")
	lock, err := wait.Listen(filepath.Join(r.dir, ".handoff"), "reviewer", "wait")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	pid, end := cliEnds(t)
	r.as(t, claude("s-rev-2", pid), "role", "reviewer-2")
	end() // reviewer-2's session ended; its binding stays

	out, err := r.atTerminalWith(t, notify, nil, "fyi", "Carry on while I'm away.", "Back at 9.")
	if err != nil {
		t.Fatalf("fyi: %v\n%s", err, out)
	}
	if !strings.Contains(out, "told implementer and reviewer (#1, #2)\r\n") || !strings.Contains(out, "not told, no session: reviewer-2\r\n") {
		t.Fatalf("fyi said:\n%s", out)
	}
	es := r.snap(t).Entries
	if len(es) != 2 {
		t.Fatalf("%d entries, want 2: %+v", len(es), es)
	}
	for i, to := range []string{"implementer", "reviewer"} {
		if e := es[i]; e.Kind != "fyi" || e.From != "operator" || e.To != to || e.Item != "" || e.Body != "Carry on while I'm away. Back at 9." {
			t.Errorf("entry %d: %+v", i+1, e)
		}
	}
	// The implementer's fyi names the session given the role; the
	// reviewer's names none, since only a listener holds that role.
	if es[0].Session != "s-impl" || es[1].Session != "" {
		t.Errorf("sessions %q and %q; want s-impl and none", es[0].Session, es[1].Session)
	}
	if es[0].TS != es[1].TS {
		t.Errorf("written at %s and %s, not in one write", es[0].TS, es[1].TS)
	}

	res := r.run(t, "", "", impl, "wait", "--for", "implementer", "--timeout", "1s")
	if res.code != 0 || res.out != "#1 fyi - operator → implementer: Carry on while I'm away. Back at 9.\n" {
		t.Fatalf("the implementer's wait: exit %d\n%s%s", res.code, res.out, res.err)
	}
	if show := r.ok(t, "", "show", "2"); !strings.HasPrefix(show, "#2 fyi\nfrom: operator → reviewer\n") || !strings.HasSuffix(show, "\nCarry on while I'm away. Back at 9.\n") {
		t.Errorf("show 2:\n%s", show)
	}
	if show := r.ok(t, "", "show", "1"); !strings.Contains(show, "\nsession: s-impl\n") {
		t.Errorf("show 1 names no session:\n%s", show)
	}
	for _, role := range []string{"implementer", "reviewer", "reviewer-2", "operator"} {
		if res := r.h(t, role, "inbox", "--quiet"); res.code != 0 {
			t.Errorf("%s has something waiting after an fyi: exit %d", role, res.code)
		}
	}
	if got := sentTo(t, sent); len(got) != 0 {
		t.Errorf("an fyi told the operator: %q", got)
	}
	if b := r.ok(t, "reviewer", "brief"); !strings.Contains(b, "\n- An fyi from the operator, to every agent with a session: take it as the operator's word, as if said in this session, and act on it. Don't answer it through handoff; anything to say, say in this session.\n") {
		t.Errorf("the brief says nothing of an fyi:\n%s", b)
	}
}

// Only the operator tells, at a terminal, outside any agent's session, with
// a message and no flags. A refusal writes nothing.
func TestH26FyiIsTheOperatorsAlone(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	impl := claude("s-impl", cli(t))
	r.as(t, impl, "role", "implementer")

	r.refused(t, "", "fyi needs a terminal on stdin", "fyi", "Carry on.")
	r.refused(t, "operator", "fyi needs a terminal on stdin", "fyi", "Carry on.")
	r.refused(t, "implementer", "fyi is for the operator; the caller's role is implementer", "fyi", "Carry on.")
	for _, sess := range [][]string{impl, codex("t-no-role")} {
		r.refusedIn(t, sess, "fyi is the operator's, in a terminal of the operator's own, never in an agent's session", "fyi", "Carry on.")
	}
	r.refused(t, "", "give the message", "fyi")
	r.refused(t, "", "give the message", "fyi", " ", "\n")
	r.refused(t, "", "fyi takes no flags, and a message can't start with -", "fyi", "--as", "operator", "Carry on.")
	if n := len(r.snap(t).Entries); n != 0 {
		t.Fatalf("%d entries after refusals, want none", n)
	}
	if res := r.h(t, "", "fyi", "--help"); res.code != 0 || !strings.Contains(res.err, "  fyi     \"<message>\"") {
		t.Fatalf("fyi --help: exit %d\n%s", res.code, res.err)
	}
	// decide's refusal in a session still names the pop-up.
	r.refusedIn(t, impl, "decide is the operator's, in a terminal of the operator's own or the pop-up, never in an agent's session", "decide", "W1", "--approve")
}

// With no agent's session, an fyi reaches no one, so it writes nothing and
// says so.
func TestH26FyiWithNoSessionWritesNothing(t *testing.T) {
	r := newRepo(t)
	out, err := r.atTerminal(t, "", nil, "fyi", "Carry on.")
	if err == nil || !strings.Contains(out, "no agent has a session now, so no one was told; nothing written") {
		t.Fatalf("fyi with no session: %v\n%s", err, out)
	}
	if n := len(r.snap(t).Entries); n != 0 {
		t.Fatalf("%d entries, want none", n)
	}
}

// An fyi cut short by a kill leaves a torn tail, as a post does: posting
// stops until repair, and repair keeps every byte.
func TestH26TornFyiStopsPostingUntilRepair(t *testing.T) {
	r := newRepo(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
	r.atTerminalWith(t, []string{record.FaultEnv + "=torn"}, nil, "fyi", "Carry on.")
	path := filepath.Join(r.dir, ".handoff", record.FileName)
	raw, _ := os.ReadFile(path)
	if len(raw) == 0 || bytes.HasSuffix(raw, []byte("\n")) {
		t.Fatalf("expected a torn tail after the kill; record is %q", raw)
	}
	r.refused(t, "implementer", "repair", "note", "--item", "W1", "--to", "reviewer", "--body", r.body(t, "after"))
	r.ok(t, "reviewer", "repair")
	kept, _ := os.ReadFile(path)
	if !bytes.HasPrefix(kept, raw) {
		t.Fatal("repair changed the torn bytes")
	}
	r.ok(t, "implementer", "note", "--item", "W1", "--to", "reviewer", "--body", r.body(t, "after repair"))
	if snap := r.snap(t); snap.Broken != nil || snap.Torn != nil {
		t.Fatalf("after repair: broken %v, torn %q", snap.Broken, snap.Torn)
	}
}

// H26 round 2 (row 93): an fyi is for the session that held the role when
// it was written. A session that takes the role later never gets it, whether
// the one told ended before reading it (the reviewer's P1a) or lost the role
// to another (P1b); nor does a wait outside any session. The session told
// gets it.
func TestH26AnFyiIsForTheSessionTold(t *testing.T) {
	r := newRepo(t)
	fyi := func(t *testing.T, msg string) {
		t.Helper()
		if out, err := r.atTerminal(t, "", nil, "fyi", msg); err != nil || !strings.Contains(out, "told implementer (") {
			t.Fatalf("fyi %q: %v\n%s", msg, err, out)
		}
	}
	listen := func(t *testing.T, sess []string, id string) result {
		t.Helper()
		return r.run(t, "", hookIn(id, "Stop"), sess, "hook", "listen", "--timeout", "1s")
	}
	gets := func(t *testing.T, res result, msg string) bool {
		t.Helper()
		if res.code != 2 {
			t.Fatalf("hook listen: exit %d\n%s", res.code, res.err)
		}
		return strings.Contains(res.err, msg)
	}

	// P1a: told in the middle of a turn, its process ends before the turn does.
	pid, end := cliEnds(t)
	t.Cleanup(end)
	one := claude("s-one", pid)
	r.as(t, one, "role", "implementer")
	fyi(t, "Merge when the reviewer approves.")
	if e := r.snap(t).Entries[0]; e.Session != "s-one" {
		t.Fatalf("the fyi names session %q, want s-one", e.Session)
	}
	end()
	two := claude("s-two", cli(t))
	r.as(t, two, "role", "implementer")
	if res := listen(t, two, "s-two"); gets(t, res, "Merge when the reviewer approves.") || !strings.Contains(res.err, "nothing new") {
		t.Fatalf("a later session got the old fyi:\n%s", res.err)
	}

	// P1b: told, then the operator gives the role to another session before
	// the first reads it. The first stops listening; the second gets nothing.
	fyi(t, "Carry on, two.")
	three := claude("s-three", cli(t))
	r.as(t, three, "role", "implementer")
	if res := listen(t, three, "s-three"); gets(t, res, "Carry on, two.") {
		t.Fatalf("the session given the role after the fyi got it:\n%s", res.err)
	}
	if res := listen(t, two, "s-two"); res.code == 2 && strings.Contains(res.err, "Carry on, two.") {
		t.Fatalf("the session that lost the role got the fyi after losing it:\n%s", res.err)
	}

	// The session told gets its fyi, and work for the role after it.
	fyi(t, "Carry on, three.")
	r.ok(t, "reviewer", "note", "--item", "W1", "--to", "implementer", "--body", r.body(t, "a note for whoever implements"))
	if res := listen(t, three, "s-three"); !gets(t, res, "fyi - operator → implementer: Carry on, three.") || !gets(t, res, "note W1 reviewer → implementer") {
		t.Fatalf("the session told did not get its fyi and the note:\n%s", res.err)
	}

	// A wait outside any session passes over an fyi that names one.
	fyi(t, "Carry on, three, again.")
	if res := r.run(t, "", "", nil, "wait", "--for", "implementer", "--timeout", "1s"); res.code != 3 || res.out != "" {
		t.Fatalf("a wait outside any session: exit %d\n%s%s", res.code, res.out, res.err)
	}
}

// H27 (row 94): the operator named the command fyi. tell was in no release,
// so no alias is kept: it is an unknown command, and writes nothing.
func TestH27TellIsNowFyi(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.as(t, claude("s-impl", cli(t)), "role", "implementer")
	if out, err := r.atTerminal(t, "", nil, "tell", "Carry on."); err == nil || !strings.Contains(out, `unknown command "tell"`) {
		t.Fatalf("tell: %v\n%s", err, out)
	}
	if n := len(r.snap(t).Entries); n != 0 {
		t.Fatalf("%d entries after tell, want none", n)
	}
	if res := r.h(t, "", "--help"); strings.Contains(res.err, "tell    ") || !strings.Contains(res.err, "decide, fyi, and handoff on its own") {
		t.Fatalf("the usage still names tell:\n%s", res.err)
	}
}
