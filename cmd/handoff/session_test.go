package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/wait"
)

// H12: roles handed to running sessions (proposal §17 row 73). A session is
// known by the ids its CLI gives it, so these tests stand in for sessions
// with those variables. A process that stands in for a CLI is a sleep.

// cli starts a process that stands in for a CLI's process, and returns its
// pid. It ends with the test.
func cli(t *testing.T) int {
	t.Helper()
	pid, end := cliEnds(t)
	t.Cleanup(end)
	return pid
}

// cliEnds is cli, with a way to end the process sooner.
func cliEnds(t *testing.T) (pid int, end func()) {
	t.Helper()
	p := exec.Command("sleep", "300")
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	return p.Process.Pid, func() { once.Do(func() { p.Process.Kill(); p.Wait() }) }
}

// claude is a Claude Code session's environment; codex a Codex thread's.
func claude(id string, pid int) []string {
	return []string{"CLAUDE_CODE_SESSION_ID=" + id, "CLAUDE_PID=" + strconv.Itoa(pid)}
}

func codex(id string) []string { return []string{"CODEX_THREAD_ID=" + id} }

// hookIn is what a CLI writes to a hook's stdin; a SessionStart says its
// source, startup unless given.
func hookIn(session, event string, source ...string) string {
	in := map[string]string{"session_id": session, "hook_event_name": event}
	if event == "SessionStart" {
		in["source"] = append(source, "startup")[0]
	}
	b, _ := json.Marshal(in)
	return string(b)
}

// as runs handoff in a session and wants exit 0.
func (r *repo) as(t *testing.T, sess []string, args ...string) string {
	t.Helper()
	res := r.run(t, "", "", sess, args...)
	if res.code != 0 {
		t.Fatalf("handoff %v in %v: exit %d\nstdout: %s\nstderr: %s", args, sess, res.code, res.out, res.err)
	}
	return res.out
}

// refusedIn runs handoff in a session and wants exit 2 mentioning want.
func (r *repo) refusedIn(t *testing.T, sess []string, want string, args ...string) {
	t.Helper()
	res := r.run(t, "", "", sess, args...)
	if res.code != 2 || !strings.Contains(res.err, want) {
		t.Fatalf("handoff %v in %v: exit %d, stderr %q; want exit 2 mentioning %q", args, sess, res.code, res.err, want)
	}
}

// Two sessions of one CLI, on whatever models the operator started them with, take
// the roles the operator gives them and work a round with no --as.
func TestH12TwoClaudeSessionsTakeTheRolesTheOperatorGives(t *testing.T) {
	r := newRepo(t)
	opus, fable := claude("s-opus", cli(t)), claude("s-fable", cli(t))
	out := r.as(t, opus, "role", "implementer")
	if !strings.Contains(out, "You are now implementer in ") || !strings.Contains(out, "handoff brief: you are implementer") ||
		!strings.Contains(out, "Listening: if your session has handoff's hooks") {
		t.Fatalf("role implementer:\n%s", out)
	}
	r.as(t, fable, "role", "reviewer")

	req := idOf(t, r.as(t, opus, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1 work", "--body", r.body(t, "please review")))
	heard := r.run(t, "", hookIn("s-fable", "Stop"), fable, "hook", "listen", "--timeout", "10s")
	if heard.code != exitWake || !strings.Contains(heard.err, "new for reviewer") || !strings.Contains(heard.err, "request W1 implementer → reviewer") {
		t.Fatalf("fable's hook: exit %d\n%s", heard.code, heard.err)
	}
	r.as(t, fable, "claim", req)
	if out := r.as(t, fable, "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")); !strings.Contains(out, "reply W1 reviewer → implementer") {
		t.Fatalf("reply: %s", out)
	}
	if out := r.as(t, fable, "role"); !strings.Contains(out, "You are reviewer in ") {
		t.Fatalf("role: %s", out)
	}
	r.refusedIn(t, fable, "this session's role is reviewer", "post", "--as", "implementer", "--item", "W2", "--to", "reviewer", "--commit", r.commitB, "--title", "x", "--body", r.body(t, "x"))
}

// The operator gives a role to another session: the first has none now, and its
// listener stops within a second, so the new holder can listen.
func TestH12ARoleMovesAndTheOldListenerStops(t *testing.T) {
	r := newRepo(t)
	a, c := claude("s-a", cli(t)), claude("s-c", cli(t))
	r.as(t, a, "role", "reviewer")
	listen := exec.Command(bin, "hook", "listen", "--timeout", "60s")
	listen.Dir, listen.Env, listen.Stdin = r.dir, env(a...), strings.NewReader(hookIn("s-a", "Stop"))
	var stderr bytes.Buffer
	listen.Stderr = &stderr
	if err := listen.Start(); err != nil {
		t.Fatal(err)
	}
	presence := filepath.Join(r.dir, ".handoff", "presence", "reviewer.json")
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(presence); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if out := r.as(t, c, "role", "reviewer"); !strings.Contains(out, "Another session had reviewer; it has no role now.") {
		t.Fatalf("role reviewer, taken:\n%s", out)
	}
	done := make(chan error, 1)
	go func() { done <- listen.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the old listener: %v\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		listen.Process.Kill()
		t.Fatal("the old listener still listens after its role moved")
	}
	r.refusedIn(t, a, "no role", "inbox")
	heard := r.run(t, "", hookIn("s-c", "Stop"), c, "hook", "listen", "--timeout", "2s")
	if heard.code != exitWake || !strings.Contains(heard.err, "nothing new for reviewer") {
		t.Fatalf("the new holder's hook: exit %d\n%s", heard.code, heard.err)
	}
}

// After /clear a Claude Code session has a new id in the same process, with
// a SessionStart whose source is clear; a resumed one has its id in a new
// process. It keeps its role through both (probe-ids/, clear-probe/).
func TestH12ARoleOutlivesClearAndResume(t *testing.T) {
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-1", pid), "role", "reviewer")
	// Only a clear moves a role by process: a start of another kind has none.
	if out := r.run(t, "", hookIn("s-other", "SessionStart"), claude("s-other", pid), "hook", "brief").out; !strings.Contains(out, "has no role") {
		t.Fatalf("a startup in the same process:\n%s", out)
	}
	cleared := claude("s-2", pid)
	if out := r.run(t, "", hookIn("s-2", "SessionStart", "clear"), cleared, "hook", "brief").out; !strings.Contains(out, "you are reviewer") {
		t.Fatalf("brief after /clear:\n%s", out)
	}
	if out := r.as(t, cleared, "role"); !strings.Contains(out, "You are reviewer") {
		t.Fatalf("role after /clear: %s", out)
	}
	r.refusedIn(t, claude("s-1", pid), "no role", "inbox")
	resumed := claude("s-2", cli(t))
	if out := r.run(t, "", hookIn("s-2", "SessionStart", "resume"), resumed, "hook", "brief").out; !strings.Contains(out, "you are reviewer") {
		t.Fatalf("brief after resume:\n%s", out)
	}
	var b struct {
		Role string `json:"role"`
		PID  int    `json:"pid"`
	}
	data, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "sessions", "s-2.json"))
	if err == nil {
		err = json.Unmarshal(data, &b)
	}
	if err != nil || b.Role != "reviewer" || strconv.Itoa(b.PID) != strings.TrimPrefix(resumed[1], "CLAUDE_PID=") {
		t.Fatalf("binding after resume: %+v, %v", b, err)
	}
}

// Codex's hooks may share a parent, a server for many threads (GPT-6's
// probe, evidence/reviewer/H12-50a8c2d/shared-codex-parent.txt). So a thread
// keeps its role by its id alone: another thread under the same parent, at
// any kind of start, gets none.
func TestH12ACodexThreadKeepsItsRoleAlone(t *testing.T) {
	r := newRepo(t)
	r.as(t, codex("t-1"), "role", "implementer")
	dir := t.TempDir()
	in := func(name, body string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(body), 0o644)
		return p
	}
	// One shell stands in for a shared Codex: every hook is its child. The
	// last command keeps it from exec'ing the brief.
	sh := exec.Command("/bin/sh", "-c", `"$0" hook listen --timeout 1s <"$1" 2>/dev/null; for f in "$2" "$3"; do "$0" hook brief --prefer implementer <"$f"; done; true`, bin,
		in("stop", hookIn("t-1", "Stop")), in("startup", hookIn("t-2", "SessionStart")), in("clear", hookIn("t-3", "SessionStart", "clear")))
	sh.Dir, sh.Env = r.dir, env()
	out, err := sh.CombinedOutput()
	if err != nil || strings.Count(string(out), "this session has no role in it") != 2 || strings.Contains(string(out), "you are implementer") {
		t.Fatalf("other threads under the same parent: %v\n%s", err, out)
	}
	if out := r.as(t, codex("t-1"), "role"); !strings.Contains(out, "You are implementer") {
		t.Fatalf("the first thread: %s", out)
	}
	r.refusedIn(t, codex("t-2"), "no role", "inbox")
	// That parent has ended, and a new thread still gets none: --prefer, from
	// settings made before row 80, is ignored.
	if out := r.run(t, "", hookIn("t-4", "SessionStart"), codex("t-4"), "hook", "brief", "--prefer", "implementer").out; !strings.Contains(out, "this session has no role in it") {
		t.Fatalf("after the first thread's parent ended:\n%s", out)
	}
}

// A plain wait, as a session without hooks runs, stops when its session's
// role moves, so the new holder gets what arrives (GPT-6's probe,
// evidence/reviewer/H12-50a8c2d/moved-wait.txt).
func TestH12AWaitStopsWhenItsRoleMoves(t *testing.T) {
	r := newRepo(t)
	a, b := codex("t-a"), codex("t-b")
	r.as(t, a, "role", "reviewer")
	old := exec.Command(bin, "wait", "--timeout", "30s")
	old.Dir, old.Env = r.dir, env(a...)
	var oldOut bytes.Buffer
	old.Stdout, old.Stderr = &oldOut, &oldOut
	if err := old.Start(); err != nil {
		t.Fatal(err)
	}
	presence := filepath.Join(r.dir, ".handoff", "presence", "reviewer.json")
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(presence); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.as(t, b, "role", "reviewer")
	newWait := exec.Command(bin, "wait", "--timeout", "15s")
	newWait.Dir, newWait.Env = r.dir, env(b...)
	var newOut bytes.Buffer
	newWait.Stdout, newWait.Stderr = &newOut, &newOut
	if err := newWait.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- old.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		old.Process.Kill()
		t.Fatal("the old wait still waits after its role moved")
	}
	if code := old.ProcessState.ExitCode(); code != 2 || !strings.Contains(oldOut.String(), "no longer holds reviewer") {
		t.Fatalf("the old wait: exit %d\n%s", code, oldOut.String())
	}
	time.Sleep(time.Second) // the new wait has the lock by now
	r.ok(t, "implementer", "note", "--item", "W1", "--to", "reviewer", "--title", "for the reviewer", "--body", r.body(t, "a note"))
	if err := newWait.Wait(); err != nil || !strings.Contains(newOut.String(), "note W1 implementer → reviewer: for the reviewer") {
		t.Fatalf("the new holder's wait: %v\n%s", err, newOut.String())
	}
}

// Row 80: a session that starts with no role gets none, even where a role
// is named after its CLI and settings made before row 80 still pass
// --prefer. It is told how to get one, and has one once the operator gives it.
func TestH16AStartTakesNoRole(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init", "--roles", "claude,codex")
	first, second := claude("s-first", cli(t)), codex("t-second")
	for _, s := range []struct {
		id  string
		env []string
	}{{"s-first", first}, {"t-second", second}} {
		out := r.run(t, "", hookIn(s.id, "SessionStart"), s.env, "hook", "brief", "--prefer", "claude").out
		if !strings.Contains(out, "this session has no role in it") || !strings.Contains(out, " role <role>` (the roles are claude, codex)") {
			t.Fatalf("%s's brief at its start:\n%s", s.id, out)
		}
		if res := r.run(t, "", hookIn(s.id, "Stop"), s.env, "hook", "listen", "--prefer", "claude", "--timeout", "1s"); res.code != 0 || res.err != "" {
			t.Fatalf("%s: listen with no role: exit %d\n%s", s.id, res.code, res.err)
		}
		r.refusedIn(t, s.env, "no role", "inbox")
	}
	if entries, _ := os.ReadDir(filepath.Join(r.dir, ".handoff", sessionsDir)); slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !strings.HasPrefix(e.Name(), ".") }) {
		t.Fatalf("a session was given a role: %v", entries)
	}
	r.as(t, first, "role", "codex") // the operator gives the Claude Code session codex
	if out := r.run(t, "", hookIn("s-first", "SessionStart", "resume"), first, "hook", "brief", "--prefer", "claude").out; !strings.Contains(out, "you are codex") {
		t.Fatalf("after the operator gave it codex:\n%s", out)
	}
}

// An agent's session never gets the operator's question, with a role or without one,
// and never decides, whatever role it claims.
func TestH12AnAgentSessionNeverAnswersForTheOperator(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	out, err := r.atTerminalWith(t, codex("t-none"), nil)
	if err != nil || !strings.Contains(out, "this session has no role in it") || strings.Contains(out, "Approve, deny or skip?") {
		t.Fatalf("bare handoff in a session with no role: %v\n%s", err, out)
	}
	r.as(t, codex("t-rev"), "role", "reviewer")
	if out, err := r.atTerminalWith(t, codex("t-rev"), nil); err != nil || !strings.Contains(out, "inbox for reviewer") || strings.Contains(out, "Approve, deny or skip?") {
		t.Fatalf("bare handoff in a session with a role: %v\n%s", err, out)
	}
	for _, as := range [][]string{nil, {"--as", "operator"}} {
		out, _ := r.atTerminalWith(t, codex("t-none"), nil, append([]string{"decide", "W1", "--approve"}, as...)...)
		if !strings.Contains(out, "never in an agent's session") {
			t.Fatalf("decide %v in a session:\n%s", as, out)
		}
	}
	if d, _ := r.decision(t, req); d != "" {
		t.Fatalf("a decision was written: %s", d)
	}
	// The operator, in a terminal of the operator's own, still gets the question.
	if out, err := r.atTerminal(t, "", []step{{"Approve, deny or skip?", "s"}}); err != nil {
		t.Fatalf("The operator's terminal: %v\n%s", err, out)
	}
}

func TestH12RoleRefusals(t *testing.T) {
	r := newRepo(t)
	r.refused(t, "", "is for a Claude Code or Codex session", "role", "reviewer")
	s := codex("t-x")
	r.refusedIn(t, s, `unknown role "nobody"`, "role", "nobody")
	r.refusedIn(t, s, "operator is the person who decides", "role", "operator")
	r.refusedIn(t, append(s, "HANDOFF_ROLE=implementer"), "fix its role to implementer", "role", "reviewer")
	r.refusedIn(t, s, "no role: when the operator gives this session one, run `handoff role <role>`", "inbox")
}

// A role that moves while a listener starts, after it read its role and
// before it listens, stops it too, before it takes anything (GPT-6's probe,
// evidence/reviewer/H12-589c1cd/startup-race.txt). HANDOFF_TEST_STARTUP_DELAY
// widens that moment; a note posted after it reaches the new holder.
func TestH12ARoleMovedWhileAListenerStartsStopsIt(t *testing.T) {
	for _, kind := range []string{"hook", "wait"} {
		t.Run(kind, func(t *testing.T) {
			r := newRepo(t)
			a, c := codex("t-a"), codex("t-c")
			r.as(t, a, "role", "reviewer")
			args, stdin := []string{"wait", "--timeout", "20s"}, ""
			if kind == "hook" {
				args, stdin = []string{"hook", "listen", "--timeout", "20s"}, hookIn("t-a", "Stop")
			}
			old := exec.Command(bin, args...)
			old.Dir, old.Env, old.Stdin = r.dir, env(append(a, "HANDOFF_TEST_STARTUP_DELAY=1500ms")...), strings.NewReader(stdin)
			var oldOut bytes.Buffer
			old.Stdout, old.Stderr = &oldOut, &oldOut
			if err := old.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- old.Wait() }()
			time.Sleep(500 * time.Millisecond) // it has read its role and pauses
			r.as(t, c, "role", "reviewer")
			time.Sleep(2 * time.Second) // past the pause: a listener that kept its role would listen now
			r.ok(t, "implementer", "note", "--item", "W1", "--to", "reviewer", "--title", "for the reviewer", "--body", r.body(t, "a note"))
			got := r.run(t, "", "", c, "wait", "--timeout", "5s")
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				old.Process.Kill()
				t.Fatalf("the old %s still listens after its role moved:\n%s", kind, oldOut.String())
			}
			if got.code != 0 || !strings.Contains(got.out, "note W1 implementer → reviewer: for the reviewer") {
				t.Fatalf("the new holder's wait: exit %d\n%s%s\nthe old %s:\n%s", got.code, got.out, got.err, kind, oldOut.String())
			}
			want := map[string]int{"hook": 0, "wait": 2}[kind]
			if code := old.ProcessState.ExitCode(); code != want || strings.Contains(oldOut.String(), "for the reviewer") {
				t.Fatalf("the old %s: exit %d, want %d\n%s", kind, code, want, oldOut.String())
			}
		})
	}
}

// H23 (row 89): init makes reviewer-2, but not every project gives it a
// session. The implementer's brief names the reviewers with one, and post
// and note say when the role they went to has none.
func TestH23ReviewersWithASession(t *testing.T) {
	r := newRepo(t)
	asks := func(t *testing.T) string {
		t.Helper()
		b := r.ok(t, "implementer", "brief")
		for _, l := range strings.Split(b, "\n") {
			if strings.HasPrefix(l, "To ask for a review") {
				return l
			}
		}
		t.Fatalf("the brief tells no one how to ask:\n%s", b)
		return ""
	}
	pid, end := cliEnds(t)
	t.Cleanup(end)
	r.as(t, claude("s-rev", pid), "role", "reviewer")
	if l := asks(t); !strings.Contains(l, " --to reviewer --commit <sha> ") ||
		!strings.HasSuffix(l, "`. reviewer-2 has no session now: send to it only once the operator gives it one.") {
		t.Errorf("with reviewer alone: %s", l)
	}
	r.as(t, codex("t-rev-2"), "role", "reviewer-2")
	if l := asks(t); !strings.HasSuffix(l, "`, where <reviewer> is reviewer or reviewer-2.") {
		t.Errorf("with both: %s", l)
	}

	// Posts and notes to a role with no session say so; to one with a
	// session, or to the operator, they don't.
	sends := func(t *testing.T, to string, args ...string) bool {
		t.Helper()
		res := r.run(t, "implementer", "", nil, args...)
		if res.code != 0 {
			t.Fatalf("handoff %v: exit %d\n%s%s", args, res.code, res.out, res.err)
		}
		return strings.Contains(res.err, "handoff: no session has "+to+" now, so this waits until the operator gives one that role; tell the operator.\n")
	}
	if sends(t, "reviewer", "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "W1")) ||
		sends(t, "operator", "note", "--item", "W1", "--to", "operator", "--body", r.body(t, "a question")) {
		t.Error("told of no session for a role that has one, or for the operator")
	}
	end() // reviewer's session ends; its binding stays
	if l := asks(t); !strings.Contains(l, " --to reviewer-2 --commit <sha> ") || !strings.HasSuffix(l, "`. reviewer has no session now: send to it only once the operator gives it one.") {
		t.Errorf("once reviewer's session ended: %s", l)
	}
	if !sends(t, "reviewer", "note", "--item", "W1", "--to", "reviewer", "--body", r.body(t, "a note")) ||
		!sends(t, "reviewer", "post", "--item", "W2", "--to", "reviewer", "--commit", r.commitB, "--title", "W2", "--body", r.body(t, "W2")) {
		t.Error("not told that reviewer has no session")
	}

	// A role something listens for has a session, whoever gave it.
	lock, err := wait.Listen(filepath.Join(r.dir, ".handoff"), "reviewer", "wait")
	if err != nil {
		t.Fatal(err)
	}
	if sends(t, "reviewer", "note", "--item", "W2", "--to", "reviewer", "--body", r.body(t, "another note")) {
		t.Error("told of no session for a role something listens for")
	}
	if l := asks(t); !strings.HasSuffix(l, "`, where <reviewer> is reviewer or reviewer-2.") {
		t.Errorf("with reviewer listened for: %s", l)
	}
	lock.Close()
	if !sends(t, "reviewer", "note", "--item", "W2", "--to", "reviewer", "--body", r.body(t, "a third note")) {
		t.Error("not told once nothing listens for reviewer")
	}
}
