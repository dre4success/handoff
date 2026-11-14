package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// H30: a role follows a Claude Code session sent to the background (proposal
// §17 rows 99, 100). /background ends the session, and its conversation goes
// on in a new id, in another process, with a SessionStart whose source is
// fork. The old transcript's last lines name the new id. /fork starts the
// same kind of session, keeps the old one, and writes no such line
// (evidence/h30/). The new session usually starts a moment before the old
// process ends (the reviewer's real runs, evidence/reviewer/H30-345a0e7/).

// hookInT is what Claude Code writes to a hook's stdin, with its transcript.
func hookInT(session, event, transcript string, source ...string) string {
	in := map[string]string{"session_id": session, "hook_event_name": event, "transcript_path": transcript}
	if event == "SessionStart" {
		in["source"] = append(source, "startup")[0]
	}
	b, _ := json.Marshal(in)
	return string(b)
}

// wentOnLine is the line Claude Code ends a transcript with after /background.
func wentOnLine(id, next string, at time.Time) string {
	return `{"type":"continued-in","timestamp":"` + at.UTC().Format("2006-01-02T15:04:05.000Z") + `","sessionId":"` + id + `","continuedInSessionId":"` + next + `"}`
}

// folder is a folder of transcripts, as Claude Code keeps them for one
// project. Its path gives a session's transcript there.
type folder string

func newFolder(t *testing.T) folder {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "-Users-someone-project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return folder(dir)
}

func (f folder) path(id string) string { return filepath.Join(string(f), id+".jsonl") }

// write writes id's transcript, with these lines between a turn and the short
// lines Claude Code ends one with.
func (f folder) write(t *testing.T, id string, lines ...string) {
	t.Helper()
	all := append([]string{
		`{"type":"user","sessionId":"` + id + `","message":{"role":"user","content":"Run the shell command sleep 25."}}`,
		`{"type":"system","subtype":"informational","content":"Backgrounding after the current tool finishes…","sessionId":"` + id + `"}`,
	}, lines...)
	all = append(all,
		`{"type":"cost-state","sessionId":"`+id+`","totalCostUSD":0.01}`,
		`{"type":"last-prompt","lastPrompt":"Run the shell command sleep 25.","sessionId":"`+id+`"}`)
	if err := os.WriteFile(f.path(id), []byte(strings.Join(all, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// wentOn writes, for each session, a transcript saying it went on as the one
// named, now.
func (f folder) wentOn(t *testing.T, moves map[string]string) {
	t.Helper()
	for id, next := range moves {
		f.write(t, id, wentOnLine(id, next, time.Now()))
	}
}

// boundTo reads a session's binding.
func boundTo(t *testing.T, r *repo, id string) (b binding, ok bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "sessions", id+".json"))
	if err != nil {
		return b, false
	}
	return b, json.Unmarshal(data, &b) == nil
}

// The new session's SessionStart runs while the old process still ends. Its
// hook waits for that, and its brief gives it the role.
func TestH30RoleFollowsBackground(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-old", oldPID), "role", "implementer")
	given, _ := boundTo(t, r, "s-old")
	f := newFolder(t)
	f.wentOn(t, map[string]string{"s-old": "s-new"})
	newPID := cli(t)
	moved := claude("s-new", newPID)
	done := make(chan result)
	go func() {
		done <- r.run(t, "", hookInT("s-new", "SessionStart", f.path("s-new"), "fork"), moved, "hook", "brief")
	}()
	time.Sleep(500 * time.Millisecond)
	end()
	if res := <-done; !strings.Contains(res.out, "you are implementer") {
		t.Fatalf("brief at the new session's start:\n%s%s", res.out, res.err)
	}
	if out := r.as(t, moved, "role"); !strings.Contains(out, "You are implementer") {
		t.Fatalf("role after /background: %s", out)
	}
	r.refusedIn(t, claude("s-old", oldPID), "no role", "inbox")
	b, ok := boundTo(t, r, "s-new")
	if !ok || b.Role != "implementer" || b.PID != newPID || b.Since != given.Since || !slices.Equal(b.Was, []string{"s-old"}) {
		t.Fatalf("binding after /background: %+v (given %+v)", b, given)
	}
	if _, ok := boundTo(t, r, "s-old"); ok {
		t.Fatal("the old session's binding is still there")
	}
}

// Claude Code starts the brief and the listener at once at SessionStart. Both
// wait, one moves the role, and both have it.
func TestH30BothStartHooksHaveTheRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-old", oldPID), "role", "implementer")
	f := newFolder(t)
	f.wentOn(t, map[string]string{"s-old": "s-new"})
	moved := claude("s-new", cli(t))
	start := hookInT("s-new", "SessionStart", f.path("s-new"), "fork")
	done := make(chan result, 3)
	for range 3 {
		go func() { done <- r.run(t, "", start, moved, "hook", "brief") }()
	}
	time.Sleep(500 * time.Millisecond)
	end()
	for range 3 {
		if res := <-done; !strings.Contains(res.out, "you are implementer") {
			t.Fatalf("one of the start's hooks:\n%s%s", res.out, res.err)
		}
	}
}

// A session that still runs after the wait keeps its role. Once it has
// ended, the next hook moves it: here a Stop, and the listener listens.
func TestH30ARunningSessionKeepsItsRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-old", oldPID), "role", "reviewer")
	f := newFolder(t)
	f.wentOn(t, map[string]string{"s-old": "s-new"})
	moved := append(claude("s-new", cli(t)), "HANDOFF_TEST_CONTINUED_WAIT=300ms")
	start := time.Now()
	if res := r.run(t, "", hookInT("s-new", "SessionStart", f.path("s-new"), "fork"), moved, "hook", "brief"); !strings.Contains(res.out, "has no role") {
		t.Fatalf("brief while the old session runs:\n%s%s", res.out, res.err)
	}
	if waited := time.Since(start); waited < 300*time.Millisecond {
		t.Fatalf("the hook waited %v, not the 300ms given", waited)
	}
	if out := r.as(t, claude("s-old", oldPID), "role"); !strings.Contains(out, "You are reviewer") {
		t.Fatalf("the running session lost its role: %s", out)
	}
	end()
	heard := r.run(t, "", hookInT("s-new", "Stop", f.path("s-new")), moved, "hook", "listen", "--timeout", "2s")
	if heard.code != exitWake || !strings.Contains(heard.err, "nothing new for reviewer") {
		t.Fatalf("Stop after the old session ended: exit %d\n%s", heard.code, heard.err)
	}
}

// A session sent to the background twice before any of its hooks ran gets
// the role from the first.
func TestH30RoleFollowsTwoMoves(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-1", oldPID), "role", "reviewer")
	end()
	f := newFolder(t)
	f.wentOn(t, map[string]string{"s-1": "s-2", "s-2": "s-3"})
	pid := cli(t)
	if res := r.run(t, "", hookInT("s-3", "Stop", f.path("s-3")), claude("s-3", pid), "hook", "brief"); !strings.Contains(res.out, "you are reviewer") {
		t.Fatalf("after two moves:\n%s%s", res.out, res.err)
	}
	if b, ok := boundTo(t, r, "s-3"); !ok || b.Role != "reviewer" || b.PID != pid || !slices.Equal(b.Was, []string{"s-1"}) {
		t.Fatalf("binding: %+v", b)
	}
}

// A session resumed after /background keeps its id, and its transcript keeps
// the line. The operator then gives it the role, and closes it. The line is
// older than the role, so the background session doesn't take it back.
func TestH30AnOldLineDoesntTakeALaterRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	f.write(t, "s-old", wentOnLine("s-old", "s-new", time.Now().Add(-time.Hour)), `{"type":"user","sessionId":"s-old","message":{"role":"user","content":"Back to work."}}`)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-old", oldPID), "role", "implementer")
	end()
	sess := claude("s-new", cli(t))
	for _, in := range []string{hookInT("s-new", "SessionStart", f.path("s-new"), "fork"), hookInT("s-new", "Stop", f.path("s-new"))} {
		if res := r.run(t, "", in, sess, "hook", "brief"); !strings.Contains(res.out, "has no role") {
			t.Fatalf("%s:\n%s%s", in, res.out, res.err)
		}
	}
	if out := r.as(t, claude("s-old", oldPID), "role"); !strings.Contains(out, "You are implementer") {
		t.Fatalf("the role given later was taken: %s", out)
	}
}

// Nothing but the line naming it moves a role. Each line is written after
// the role is given, so that only the condition named keeps it.
func TestH30OnlyTheLineNamingItMovesARole(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, old, transcriptOf string
		lines                   func(now time.Time) []string // in old's transcript; nil writes none
	}{
		// /fork keeps the old session and writes no line.
		{"fork", "s-a", "s-b", func(time.Time) []string { return []string{} }},
		{"another session named", "s-a", "s-b", func(now time.Time) []string { return []string{wentOnLine("s-a", "s-c", now)} }},
		{"a transcript not named after the session", "s-a", "s-c", func(now time.Time) []string { return []string{wentOnLine("s-a", "s-b", now)} }},
		{"a line of another type", "s-a", "s-b", func(now time.Time) []string {
			return []string{`{"type":"user","tag":"continued-in","timestamp":"` + now.UTC().Format(time.RFC3339Nano) + `","continuedInSessionId":"s-b"}`}
		}},
		{"a later line naming another session", "s-a", "s-b", func(now time.Time) []string {
			return []string{wentOnLine("s-a", "s-b", now), wentOnLine("s-a", "s-c", now)}
		}},
		{"a line with no time", "s-a", "s-b", func(time.Time) []string {
			return []string{`{"type":"continued-in","sessionId":"s-a","continuedInSessionId":"s-b"}`}
		}},
		{"no transcript", "s-a", "s-b", nil},
		{"an id that isn't a plain name", "../s-a", "s-b", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			oldPID, end := cliEnds(t)
			r.as(t, claude(c.old, oldPID), "role", "implementer")
			end()
			now := time.Now().Add(time.Millisecond)
			f := newFolder(t)
			switch {
			case c.old == "../s-a":
				// The line is there, one folder up, where the id points.
				os.WriteFile(filepath.Join(string(f), "..", "s-a.jsonl"), []byte(wentOnLine("../s-a", "s-b", now)+"\n"), 0o600)
			case c.lines != nil:
				f.write(t, c.old, c.lines(now)...)
			}
			sess := claude("s-b", cli(t))
			for _, in := range []string{hookInT("s-b", "SessionStart", f.path(c.transcriptOf), "fork"), hookInT("s-b", "Stop", f.path(c.transcriptOf))} {
				if res := r.run(t, "", in, sess, "hook", "brief"); !strings.Contains(res.out, "has no role") {
					t.Fatalf("%s:\n%s%s", in, res.out, res.err)
				}
			}
			r.refusedIn(t, sess, "no role", "inbox")
			if out := r.as(t, claude(c.old, oldPID), "role"); !strings.Contains(out, "You are implementer") {
				t.Fatalf("the old session lost its role: %s", out)
			}
		})
	}
}

// Two sessions whose lines both name one move nothing to it.
func TestH30TwoSessionsNamingOneMoveNothing(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	f := newFolder(t)
	// Both are given their roles while they run, as giving a role drops the
	// bindings of sessions that have ended.
	var ends []func()
	for id, role := range map[string]string{"s-a1": "implementer", "s-a2": "reviewer"} {
		pid, end := cliEnds(t)
		r.as(t, claude(id, pid), "role", role)
		ends = append(ends, end)
	}
	for _, end := range ends {
		end()
	}
	f.wentOn(t, map[string]string{"s-a1": "s-b", "s-a2": "s-b"})
	if res := r.run(t, "", hookInT("s-b", "Stop", f.path("s-b")), claude("s-b", cli(t)), "hook", "brief"); !strings.Contains(res.out, "has no role") {
		t.Fatalf("two sessions naming one:\n%s%s", res.out, res.err)
	}
}

// An fyi the old session hadn't read reaches the session it went on as: the
// conversation it was told in goes on there. After /clear it does not (row 93).
func TestH30AnFyiForTheOldSessionReachesTheNewOne(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	_, notify := sink(t)
	oldPID, end := cliEnds(t)
	r.as(t, claude("s-old", oldPID), "role", "implementer")
	if out, err := r.atTerminalWith(t, notify, nil, "fyi", "Carry on."); err != nil || !strings.Contains(out, "told implementer (#1)") {
		t.Fatalf("fyi: %v\n%s", err, out)
	}
	end()
	f := newFolder(t)
	f.wentOn(t, map[string]string{"s-old": "s-new"})
	heard := r.run(t, "", hookInT("s-new", "Stop", f.path("s-new")), claude("s-new", cli(t)), "hook", "listen", "--timeout", "2s")
	if heard.code != exitWake || !strings.Contains(heard.err, "#1 fyi - operator → implementer: Carry on.") {
		t.Fatalf("the new session's listener: exit %d\n%s", heard.code, heard.err)
	}
}

// The line is found among the last 64 KiB of a long transcript, and a named
// pipe in place of a transcript is passed over at once.
func TestH30ReadingTheTranscript(t *testing.T) {
	t.Parallel()
	f := newFolder(t)
	at := time.Date(2026, 11, 13, 22, 18, 15, 118e6, time.UTC)
	long := strings.Repeat(`{"type":"assistant","message":{"content":"`+strings.Repeat("x", 1000)+`"}}`+"\n", 2000)
	f.write(t, "s-old", long+wentOnLine("s-old", "s-new", at))
	if next, when := continuedIn(f.path("s-old")); next != "s-new" || !when.Equal(at) {
		t.Fatalf("continuedIn = %q, %v", next, when)
	}
	if next, _ := continuedIn(f.path("s-none")); next != "" {
		t.Fatalf("no file: %q", next)
	}
	if err := syscall.Mkfifo(f.path("s-pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan string)
	go func() { next, _ := continuedIn(f.path("s-pipe")); done <- next }()
	select {
	case next := <-done:
		if next != "" {
			t.Fatalf("a named pipe: %q", next)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reading a named pipe waited")
	}
}
