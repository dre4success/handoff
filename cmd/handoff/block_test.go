package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// H34 part 2 (row 110): when handoff knows a session can't take its work,
// it keeps a block in the session's own binding, fenced so a stale hook
// can't write one, cleared when the session listens again; senders, the
// inbox, the brief and the operator see it.

// agyStopIdle is Antigravity's Stop input, with fullyIdle as given.
func agyStopIdle(conversation, cwd string, continued int, idle bool) string {
	var in map[string]any
	json.Unmarshal([]byte(agyIn(conversation, "Stop", cwd, continued)), &in)
	in["fullyIdle"] = idle
	b, _ := json.Marshal(in)
	return string(b)
}

// blockJSON is a binding's block, as its file has it.
type blockJSON struct{ Reason, Detail, At string }

// blockOf reads a session's block from its binding's file.
func (r *repo) blockOf(t *testing.T, id string) *blockJSON {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "sessions", id+".json"))
	if err != nil {
		t.Fatalf("%s has no binding: %v", id, err)
	}
	var b struct {
		Block *blockJSON `json:"block"`
	}
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	return b.Block
}

// At Antigravity's cap the block is written; senders, the inbox, the brief
// and handoff on its own say so; the operator hears once for the work that
// waits, and again when more arrives; and the block clears when the session
// listens again, after the operator types.
func TestH34p2ACapBlocksTheSessionUntilItListensAgain(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-1")
	r.as(t, agy, "role", "reviewer")
	sent, notify := sink(t)
	at10 := runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, agyContinuations), append(agy, notify...), "hook", "listen", "--timeout", "2s")
	if strings.TrimSpace(at10.out) != "{}" {
		t.Fatalf("at the cap: %q %q", at10.out, at10.err)
	}
	if b := r.blockOf(t, "ag-1"); b == nil || b.Reason != "cap" {
		t.Fatalf("the block: %+v", b)
	}
	// No work waited: the operator is told of the cap, as before.
	if got := sentTo(t, sent); len(got) != 1 || !strings.Contains(got[0], "handoff can't wake reviewer's Antigravity session for new work until you type in it") {
		t.Fatalf("told at the cap: %q", got)
	}
	const blocked = "reviewer can't be woken until the operator types in its session (Antigravity's 10 wakes used, since "
	post := r.run(t, "implementer", "", notify, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	if post.code != 0 || !strings.Contains(post.err, blocked) {
		t.Fatalf("the sender: exit %d\n%s", post.code, post.err)
	}
	if out := r.ok(t, "reviewer", "inbox"); !strings.Contains(out, "blocked: "+blocked) {
		t.Fatalf("the inbox:\n%s", out)
	}
	if out := r.ok(t, "reviewer", "brief"); !strings.Contains(out, "Blocked: "+blocked) {
		t.Fatalf("the brief:\n%s", out)
	}
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "sessions:\n  blocked: "+blocked) {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
	alerts := func() []string {
		var out []string
		for _, l := range sentTo(t, sent) {
			if strings.Contains(l, "work waits for it") {
				out = append(out, l)
			}
		}
		return out
	}
	if got := alerts(); len(got) != 1 || !strings.Contains(got[0], "reviewer can't be woken until you type in its session") || !strings.Contains(got[0], "(#1)") {
		t.Fatalf("the block's alert: %q", got)
	}
	r.run(t, "implementer", "", notify, "inbox") // nothing new: not sent again
	r.run(t, "implementer", "", notify, "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "more"))
	if got := alerts(); len(got) != 2 || !strings.Contains(got[1], "(#2)") {
		t.Fatalf("the alert for new work: %q", got)
	}
	// The operator types; the turn ends and its Stop listens.
	heard := runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, 0), agy, "hook", "listen", "--timeout", "2s")
	if !strings.Contains(heard.out, "#1 request W1") {
		t.Fatalf("listening again: %q %q", heard.out, heard.err)
	}
	if b := r.blockOf(t, "ag-1"); b != nil {
		t.Fatalf("still blocked: %+v", b)
	}
}

// A stale hook can't block: one that resolved its role before the session
// took it again, one from before the session listened since, or one while a
// listener holds the role's lock.
func TestH34p2AStaleHookCantBlock(t *testing.T) {
	t.Parallel()
	var notify []string
	cap := func(r *repo, pause string) *exec.Cmd {
		cmd := exec.Command(bin, "hook", "listen", "--as", "reviewer", "--timeout", "2s")
		cmd.Dir, cmd.Env = t.TempDir(), env(append(append(antigravity("ag-1"), notify...), pause)...)
		cmd.Stdin = strings.NewReader(agyIn("ag-1", "Stop", r.dir, agyContinuations))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	// A stale hook says nothing to the operator either (#494-#496).
	quiet := func(t *testing.T, sent string) {
		t.Helper()
		if got := sentTo(t, sent); len(got) != 0 {
			t.Fatalf("a stale hook told the operator: %q", got)
		}
	}
	t.Run("the role taken again", func(t *testing.T) {
		r := newRepo(t)
		sent, e := sink(t)
		notify = e
		r.as(t, antigravity("ag-1"), "role", "reviewer")
		p, pause := pauseAt(t, "write")
		cmd := cap(r, pause)
		p.reached(t) // resolved at the first generation
		r.as(t, antigravity("ag-1"), "role", "reviewer")
		p.release(t)
		cmd.Wait()
		if b := r.blockOf(t, "ag-1"); b != nil {
			t.Fatalf("blocked by a hook of the generation before: %+v", b)
		}
		quiet(t, sent)
	})
	t.Run("listened since", func(t *testing.T) {
		r := newRepo(t)
		sent, e := sink(t)
		notify = e
		r.as(t, antigravity("ag-1"), "role", "reviewer")
		p, pause := pauseAt(t, "write")
		cmd := cap(r, pause)
		p.reached(t) // began before the listener below
		runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, 0), antigravity("ag-1"), "hook", "listen", "--timeout", "1s")
		p.release(t)
		cmd.Wait()
		if b := r.blockOf(t, "ag-1"); b != nil {
			t.Fatalf("blocked by a hook from before the session listened: %+v", b)
		}
		quiet(t, sent)
	})
	t.Run("a listener holds the lock", func(t *testing.T) {
		r := newRepo(t)
		r.as(t, antigravity("ag-1"), "role", "reviewer")
		w := exec.Command(bin, "wait", "--for", "reviewer", "--timeout", "10s")
		w.Dir, w.Env = r.dir, env()
		if err := w.Start(); err != nil {
			t.Fatal(err)
		}
		defer w.Process.Kill()
		r.listenedFor(t, "reviewer")
		sent, e := sink(t)
		runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, agyContinuations), append(antigravity("ag-1"), e...), "hook", "listen", "--timeout", "1s")
		if b := r.blockOf(t, "ag-1"); b != nil {
			t.Fatalf("blocked while a listener held the lock: %+v", b)
		}
		quiet(t, sent)
	})
}

// A session that leaves the role and takes it back has no block; and a
// session with no binding gets none, nor a binding.
func TestH34p2ABlockIsTheBindingsAndNeverMakesOne(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-1")
	r.as(t, agy, "role", "reviewer")
	runAt(t, t.TempDir(), agyIn("ag-1", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "1s")
	if r.blockOf(t, "ag-1") == nil {
		t.Fatal("no block at the cap")
	}
	r.as(t, agy, "role", "reviewer-2")
	r.as(t, agy, "role", "reviewer")
	if b := r.blockOf(t, "ag-1"); b != nil {
		t.Fatalf("the block came back with the role: %+v", b)
	}
	runAt(t, t.TempDir(), agyIn("ag-2", "Stop", r.dir, agyContinuations), antigravity("ag-2"), "hook", "listen", "--as", "reviewer-2", "--timeout", "1s")
	if _, err := os.Stat(filepath.Join(r.dir, ".handoff", "sessions", "ag-2.json")); err == nil {
		t.Fatal("a block made a binding")
	}
}

// A permission prompt blocks the session; senders, reply too, see it; and
// it clears when the session runs a handoff command.
func TestH34p2APermissionPromptClearsOnACommand(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	req := r.request(t, "W1", r.commitA)
	prompt := `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}`
	r.run(t, "", prompt, imp, "hook", "notify")
	if b := r.blockOf(t, "s-imp"); b == nil || b.Reason != "permission" || b.Detail != "Claude needs your permission to use Bash" {
		t.Fatalf("the block: %+v", b)
	}
	reply := r.run(t, "reviewer", "", nil, "reply", req, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix"))
	if reply.code != 0 || !strings.Contains(reply.err, "implementer's session is at a permission prompt: Claude needs your permission to use Bash, since ") {
		t.Fatalf("the reply's sender: exit %d\n%s", reply.code, reply.err)
	}
	// A hook is no command from the session: one with --as leaves the block
	// (reviewer-3, #496).
	r.run(t, "", `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"idle_prompt","message":"waiting"}`, imp, "hook", "notify", "--as", "implementer")
	r.run(t, "", hookIn("s-imp", "SessionStart"), imp, "hook", "brief", "--as", "implementer")
	if b := r.blockOf(t, "s-imp"); b == nil {
		t.Fatal("a hook with --as cleared the block")
	}
	r.as(t, imp, "inbox")
	if b := r.blockOf(t, "s-imp"); b != nil {
		t.Fatalf("still blocked after a command: %+v", b)
	}
}

// While the agent's own task runs, Antigravity's Stop says fullyIdle false:
// the hook stands aside, neither listening nor blocking.
func TestH34p2AnAgentsOwnTaskKeepsTheHookAside(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-1")
	r.as(t, agy, "role", "reviewer")
	began := time.Now()
	aside := runAt(t, t.TempDir(), agyStopIdle("ag-1", r.dir, 0, false), agy, "hook", "listen", "--timeout", "20s")
	if strings.TrimSpace(aside.out) != "{}" || time.Since(began) > 10*time.Second {
		t.Fatalf("with its own task running: %q %q after %v", aside.out, aside.err, time.Since(began))
	}
	if b := r.blockOf(t, "ag-1"); b != nil {
		t.Fatalf("blocked while its own task ran: %+v", b)
	}
	idle := runAt(t, t.TempDir(), agyStopIdle("ag-1", r.dir, 0, true), agy, "hook", "listen", "--timeout", "1s")
	if !strings.Contains(idle.out, "nothing new for reviewer") {
		t.Fatalf("fully idle: %q %q", idle.out, idle.err)
	}
}

// handoff on its own shows when a session last listened.
func TestH34p2LastListenedShowsTheTime(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	rev := claude("s-rev", cli(t))
	r.as(t, rev, "role", "reviewer")
	r.run(t, "", hookIn("s-rev", "Stop"), rev, "hook", "listen", "--timeout", "1s")
	// Shorter than quiet_after, a gap is a turn at work: not shown (#495).
	if home := r.run(t, "", "", nil); strings.Contains(home.out, "sessions:") {
		t.Fatalf("handoff on its own, a moment after:\n%s", home.out)
	}
	r.setQuiet(t, "1s")
	time.Sleep(2 * time.Second) // presence rounds to the second
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "sessions:\n  reviewer: not listening now (last listened just now)") {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
}

// reviewer-2's and the reviewer's #494, #495: what waits for a blocked role
// is any entry addressed to it past its cursor, a note alone as much as a
// request, or a cut round its inbox says to post again; a note delivered
// before the block waits for nothing.
func TestH34p2WhatWaitsForABlockedRole(t *testing.T) {
	t.Parallel()
	blockAt := func(t *testing.T, r *repo, role string, e []string) {
		t.Helper()
		agy := antigravity("ag-" + role)
		r.as(t, agy, "role", role)
		runAt(t, t.TempDir(), agyIn("ag-"+role, "Stop", r.dir, agyContinuations), append(agy, e...), "hook", "listen", "--timeout", "1s")
		if r.blockOf(t, "ag-"+role) == nil {
			t.Fatalf("%s not blocked", role)
		}
	}
	waits := func(t *testing.T, sent string) []string {
		var out []string
		for _, l := range sentTo(t, sent) {
			if strings.Contains(l, "work waits for it") {
				out = append(out, l)
			}
		}
		return out
	}
	t.Run("a note alone", func(t *testing.T) {
		r := newRepo(t)
		sent, e := sink(t)
		blockAt(t, r, "reviewer", e)
		note := idOf(t, r.run(t, "implementer", "", e, "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "hold the merge")).out)
		_ = r.run(t, "implementer", "", e, "note", "--to", "reviewer-2", "--item", "W1", "--body", r.body(t, "another role"))
		if got := waits(t, sent); len(got) != 1 || !strings.Contains(got[0], "(#"+note+")") {
			t.Fatalf("the alert for a note alone: %q", got)
		}
	})
	t.Run("a note delivered before the block", func(t *testing.T) {
		r := newRepo(t)
		sent, e := sink(t)
		r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "read"))
		r.run(t, "reviewer", "", nil, "wait", "--timeout", "2s") // delivered
		blockAt(t, r, "reviewer", e)
		r.run(t, "implementer", "", e, "note", "--to", "reviewer-2", "--item", "W1", "--body", r.body(t, "another role"))
		if got := waits(t, sent); len(got) != 0 {
			t.Fatalf("an alert for work already delivered: %q", got)
		}
	})
	t.Run("a cut round", func(t *testing.T) {
		r := newRepo(t)
		sent, e := sink(t)
		r.run(t, "implementer", "", []string{"HANDOFF_TEST_FAULT=first-line"}, "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
		blockAt(t, r, "implementer", e)
		if got := waits(t, sent); len(got) != 1 || !strings.Contains(got[0], "implementer can't be woken") {
			t.Fatalf("the alert for a cut round: %q", got)
		}
	})
}

// reviewer-2's and reviewer-3's #494, #496: a permission prompt's
// notification marks its own block's alert sent, and leaves another role's
// to be sent.
func TestH34p2APromptMarksOnlyItsOwnAlert(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	sent, e := sink(t)
	// Its id begins with the prompt's session's, which a prefix would take
	// for the same (reviewer-2, #521).
	agy := antigravity("s-imp-review")
	r.as(t, agy, "role", "reviewer")
	runAt(t, t.TempDir(), agyIn("s-imp-review", "Stop", r.dir, agyContinuations), append(agy, e...), "hook", "listen", "--timeout", "1s")
	// Work for the blocked reviewer, its alert not sent: the notification
	// went nowhere.
	nowhere := []string{"HANDOFF_TEST_NOTIFY=" + filepath.Join(t.TempDir(), "no", "such", "file")}
	req := idOf(t, r.run(t, "implementer", "", nowhere, "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b")).out)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	prompt := `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow rm"}`
	r.run(t, "", prompt, append(imp, e...), "hook", "notify")
	r.run(t, "reviewer-2", "", e, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "c"))
	found := false
	for _, l := range sentTo(t, sent) {
		found = found || strings.Contains(l, "reviewer can't be woken until you type in its session") && strings.Contains(l, "(#"+req+")")
	}
	if !found {
		t.Fatalf("the reviewer's alert was dropped: %q", sentTo(t, sent))
	}
}

// reviewer-2's #521, point 2: a command clears the prompt it saw, and not a
// newer one recorded while it ran.
func TestH34p2ACommandClearsOnlyThePromptItSaw(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	prompt := func(msg string) {
		r.run(t, "", `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"`+msg+`"}`, imp, "hook", "notify")
	}
	prompt("allow A")
	p, pause := pauseAt(t, "write")
	cmd := exec.Command(bin, "inbox")
	cmd.Dir, cmd.Env = r.dir, env(append(imp, pause)...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // it has read prompt A's block
	time.Sleep(10 * time.Millisecond)
	prompt("allow B")
	p.release(t)
	cmd.Wait()
	if b := r.blockOf(t, "s-imp"); b == nil || b.Detail != "allow B" {
		t.Fatalf("prompt B's block: %+v", b)
	}
}

// reviewer-2's #590, point 1: a prompt's hook marks sent the alert of the
// block it wrote, and never a newer prompt's, which another hook wrote while
// it was held and failed to notify.
func TestH34p2APromptsNoticeMarksOnlyItsOwnBlock(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	sent, notify := sink(t)
	r.run(t, "reviewer", "", nil, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "work"))
	in := func(msg string) string {
		return `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"` + msg + `"}`
	}
	p, pause := pauseAt(t, "notified")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(append(imp, notify...), pause)...), strings.NewReader(in("allow A"))
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // A wrote its block
	time.Sleep(10 * time.Millisecond)
	broken := []string{"HANDOFF_TEST_NOTIFY=" + filepath.Join(t.TempDir(), "no", "such", "sent.txt")}
	r.run(t, "", in("allow B"), append(imp, broken...), "hook", "notify") // B's notice fails
	p.release(t)
	a.Wait()
	if b := r.blockOf(t, "s-imp"); b == nil || b.Detail != "allow B" {
		t.Fatalf("the block: %+v", b)
	}
	// An alert pass, by a write that adds no work for the implementer.
	r.run(t, "reviewer", "", notify, "note", "--to", "reviewer-2", "--item", "W1", "--body", r.body(t, "aside"))
	found := false
	for _, l := range sentTo(t, sent) {
		found = found || strings.Contains(l, "allow B") && strings.Contains(l, "work waits for it")
	}
	if !found {
		t.Fatalf("prompt B's alert was marked sent by A's notice: %q", sentTo(t, sent))
	}
}

// reviewer-2's #590, point 2: a hook that saw an older prompt, held before
// its write, can't replace a newer prompt's block. The role is fixed by the
// settings, as reviewer-2 ran it, so the hook resolves it where the write
// pause is.
func TestH34p2AnOlderPromptCantReplaceANewer(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := append(claude("s-imp", cli(t)), "HANDOFF_ROLE=implementer")
	r.as(t, imp, "role", "implementer")
	in := func(msg string) string {
		return `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"` + msg + `"}`
	}
	p, pause := pauseAt(t, "write")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(imp, pause)...), strings.NewReader(in("allow A"))
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // A has its role, not yet its block
	time.Sleep(10 * time.Millisecond)
	r.run(t, "", in("allow B"), imp, "hook", "notify")
	p.release(t)
	a.Wait()
	if b := r.blockOf(t, "s-imp"); b == nil || b.Detail != "allow B" {
		t.Fatalf("the block: %+v", b)
	}
}

// reviewer-2's #590, point 3: any command from the session clears its
// prompt, show among them; a hook of it doesn't.
func TestH34p2AnyCommandFromTheSessionClearsItsPrompt(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	id := idOf(t, r.run(t, "reviewer", "", nil, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "work")).out)
	r.run(t, "", `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow A"}`, imp, "hook", "notify")
	r.run(t, "", hookIn("s-imp", "UserPromptSubmit"), imp, "hook", "start")
	if b := r.blockOf(t, "s-imp"); b == nil {
		t.Fatal("a hook cleared the prompt")
	}
	if res := r.run(t, "", "", imp, "show", id); res.code != 0 {
		t.Fatalf("show: exit %d\n%s", res.code, res.err)
	}
	if b := r.blockOf(t, "s-imp"); b != nil {
		t.Fatalf("show left the prompt: %+v", b)
	}
}

// reviewer-2's #596, point 1: work that arrives while a prompt's hook is
// held after its write is marked sent only by a notice that names it.
func TestH34p2APromptsNoticeNamesTheWorkItMarks(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	sent, notify := sink(t)
	p, pause := pauseAt(t, "notified")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(append(imp, notify...), pause)...), strings.NewReader(`{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow A"}`)
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // A wrote its block, with no work waiting
	broken := []string{"HANDOFF_TEST_NOTIFY=" + filepath.Join(t.TempDir(), "no", "such", "sent.txt")}
	note := idOf(t, r.run(t, "reviewer", "", broken, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "work")).out) // its alert fails
	p.release(t)
	a.Wait()
	got := sentTo(t, sent)
	if len(got) != 1 || !strings.Contains(got[0], "allow A") || !strings.Contains(got[0], "work waits for it (#"+note+")") {
		t.Fatalf("A's notice should name the work it marks: %q", got)
	}
	r.run(t, "reviewer", "", notify, "note", "--to", "reviewer-2", "--item", "W1", "--body", r.body(t, "aside")) // an alert pass
	if again := sentTo(t, sent); len(again) != 1 {
		t.Fatalf("sent again: %q", again)
	}
}

// reviewer-2's #596, point 2: once a command from the session clears a
// prompt, a hook that saw an older one, held before its write, can't block
// the session again.
func TestH34p2AClearedPromptStillFencesOlderHooks(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := append(claude("s-imp", cli(t)), "HANDOFF_ROLE=implementer")
	r.as(t, imp, "role", "implementer")
	in := func(msg string) string {
		return `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"` + msg + `"}`
	}
	p, pause := pauseAt(t, "write")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(imp, pause)...), strings.NewReader(in("allow A"))
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // A has its role, not yet its block
	time.Sleep(10 * time.Millisecond)
	r.run(t, "", in("allow B"), imp, "hook", "notify")
	r.run(t, "", "", imp, "inbox") // the session answers B
	if b := r.blockOf(t, "s-imp"); b != nil {
		t.Fatalf("B not cleared: %+v", b)
	}
	p.release(t)
	a.Wait()
	if b := r.blockOf(t, "s-imp"); b != nil {
		t.Fatalf("an older prompt blocked the session again: %+v", b)
	}
}

// reviewer-2's #610, point 1: a prompt's keyed alert, sent meanwhile by
// another process's alert pass, isn't sent again by the prompt's hook.
func TestH34p2APromptsAlertIsSentOnce(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := claude("s-imp", cli(t))
	r.as(t, imp, "role", "implementer")
	sent, notify := sink(t)
	p, pause := pauseAt(t, "notified")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(append(imp, notify...), pause)...), strings.NewReader(`{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow A"}`)
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t)                                                                                                                      // A wrote its block, with no work waiting
	note := idOf(t, r.run(t, "reviewer", "", notify, "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "work")).out) // its alert goes
	p.release(t)
	a.Wait()
	var named []string
	for _, l := range sentTo(t, sent) {
		if strings.Contains(l, "work waits for it (#"+note+")") {
			named = append(named, l)
		}
	}
	if len(named) != 1 {
		t.Fatalf("the alert for #%s, sent %d times: %q", note, len(named), sentTo(t, sent))
	}
}

// reviewer-2's #610, point 2: a prompt's report, held before its write,
// can't block the session once a command of it ran, though the command
// saw no block; a report from after the command still does.
func TestH34p2ACommandFencesAReportItNeverSaw(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	imp := append(claude("s-imp", cli(t)), "HANDOFF_ROLE=implementer")
	r.as(t, imp, "role", "implementer")
	in := func(msg string) string {
		return `{"session_id":"s-imp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"` + msg + `"}`
	}
	p, pause := pauseAt(t, "write")
	a := exec.Command(bin, "hook", "notify")
	a.Dir, a.Env, a.Stdin = r.dir, env(append(imp, pause)...), strings.NewReader(in("allow A"))
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	p.reached(t) // A has its role, not yet its block
	time.Sleep(10 * time.Millisecond)
	r.run(t, "", "", imp, "inbox") // the operator answered; the session works, and saw no block
	p.release(t)
	a.Wait()
	if b := r.blockOf(t, "s-imp"); b != nil {
		t.Fatalf("a report from before the command blocked the session: %+v", b)
	}
	r.run(t, "", in("allow B"), imp, "hook", "notify") // after it
	if b := r.blockOf(t, "s-imp"); b == nil || b.Detail != "allow B" {
		t.Fatalf("a report from after the command: %+v", b)
	}
}
