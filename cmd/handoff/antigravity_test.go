package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// H32: Antigravity sessions (proposal §17 row 102). Antigravity gives the
// commands its agent runs, and its hooks, ANTIGRAVITY_CONVERSATION_ID, and
// runs those commands with a terminal on stdin (evidence/h32/).

func antigravity(id string) []string { return []string{"ANTIGRAVITY_CONVERSATION_ID=" + id} }

// An Antigravity session is an agent's session: the operator's commands
// refuse it, with a role or without, before they look for a terminal.
func TestH32TheOperatorsCommandsRefuseAnAntigravitySession(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "t", "--body", r.body(t, "b"))
	agy := antigravity("0a1b2c3d-agy")
	for _, c := range [][]string{{"decide", "W1", "--approve"}, {"fyi", "Carry on."}, {"away", "Merge when approved."}, {"back"}} {
		where := "in a terminal of the operator's own"
		if c[0] == "decide" {
			where += " or the pop-up"
		}
		r.refusedIn(t, agy, c[0]+" is the operator's, "+where+", never in an agent's session", c...)
	}
	r.as(t, agy, "role", "reviewer")
	r.refusedIn(t, agy, "decide is the operator's", "decide", "W1", "--approve")
	if out := r.as(t, agy, "role"); !strings.Contains(out, "You are reviewer in ") {
		t.Fatalf("role in an Antigravity session: %s", out)
	}
	if res := r.run(t, "", "", agy, "inbox"); res.code != 0 || !strings.Contains(res.out, "needs my review") {
		t.Fatalf("inbox in an Antigravity session with a role: exit %d\n%s%s", res.code, res.out, res.err)
	}
}

// agyIn is what Antigravity writes to a hook's stdin: the conversation, the
// event, the folder the agent is in, and for a Stop, how many times in a
// row a hook has continued the turn.
func agyIn(conversation, event, cwd string, continued int) string {
	in := map[string]any{"conversationId": conversation, "hookEventName": event, "cwd": cwd,
		"workspacePaths": []string{cwd}, "transcriptPath": "/nowhere/transcript_full.jsonl"}
	if event == "Stop" {
		in["executionNum"], in["fullyIdle"], in["terminationReason"] = continued, true, "NO_TOOL_CALL"
	}
	b, _ := json.Marshal(in)
	return string(b)
}

// runAt runs handoff in dir, as Antigravity runs a hook in the folder of
// its hooks.json, which need not be the project.
func runAt(t *testing.T, dir, stdin string, extraEnv []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env(extraEnv...), strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return result{out.String(), errb.String(), code}
}

// The brief at a start is a system message, from a hook run outside the
// project, for the folder the agent is in. With no role, it says how to get
// one; outside any project, it says nothing.
func TestH32TheStartBriefIsASystemMessage(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("0a1b-brief")
	r.as(t, agy, "role", "reviewer")
	elsewhere := t.TempDir()
	message := func(res result) string {
		t.Helper()
		var o struct {
			InjectSteps []struct {
				SystemMessage struct {
					SystemMessage string `json:"systemMessage"`
				} `json:"systemMessage"`
			} `json:"injectSteps"`
		}
		if res.code != 0 || res.err != "" || json.Unmarshal([]byte(res.out), &o) != nil || len(o.InjectSteps) != 1 {
			t.Fatalf("brief hook: exit %d, stdout %q, stderr %q", res.code, res.out, res.err)
		}
		return o.InjectSteps[0].SystemMessage.SystemMessage
	}
	if m := message(runAt(t, elsewhere, agyIn("0a1b-brief", "SessionStart", r.dir, 0), agy, "hook", "brief")); !strings.Contains(m, "you are reviewer in ") || !strings.Contains(m, "a hook listens for reviewer") {
		t.Fatalf("the brief at a start:\n%s", m)
	}
	// The stdin alone names the session, as Antigravity documents.
	if m := message(runAt(t, elsewhere, agyIn("0a1b-brief", "SessionStart", r.dir, 0), nil, "hook", "brief")); !strings.Contains(m, "you are reviewer in ") {
		t.Fatalf("the brief at a start, with the session named only on stdin:\n%s", m)
	}
	if m := message(runAt(t, elsewhere, agyIn("0a1b-new", "SessionStart", r.dir, 0), antigravity("0a1b-new"), "hook", "brief")); !strings.Contains(m, "this session has no role in it") {
		t.Fatalf("the start of a session with no role:\n%s", m)
	}
	// A folder in no project, and a relative one, which would name the
	// project from the folder above it, but isn't taken.
	for _, c := range []struct{ at, cwd string }{{elsewhere, elsewhere}, {filepath.Dir(r.dir), filepath.Base(r.dir)}} {
		if res := runAt(t, c.at, agyIn("0a1b-brief", "SessionStart", c.cwd, 0), agy, "hook", "brief"); res.code != 0 || strings.TrimSpace(res.out) != "{}" {
			t.Fatalf("a start in %s with cwd %s: exit %d, %q %q", c.at, c.cwd, res.code, res.out, res.err)
		}
	}
}

// The listener continues the turn with what arrives, or, after its time,
// with word that nothing came; up to Antigravity's limit, and no further.
func TestH32ListenContinuesTheTurn(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("0a1b-listen")
	r.as(t, agy, "role", "reviewer")
	told := filepath.Join(t.TempDir(), "told")
	e := append(agy, "HANDOFF_TEST_NOTIFY="+told)
	reason := func(res result) string {
		t.Helper()
		var o struct{ Decision, Reason string }
		if res.code != 0 || res.err != "" || json.Unmarshal([]byte(res.out), &o) != nil || o.Decision != "continue" {
			t.Fatalf("listen: exit %d, stdout %q, stderr %q", res.code, res.out, res.err)
		}
		return o.Reason
	}
	stop := func(continued int) string { return agyIn("0a1b-listen", "Stop", r.dir, continued) }
	if m := reason(runAt(t, t.TempDir(), stop(0), e, "hook", "listen", "--timeout", "2s")); !strings.Contains(m, "nothing new for reviewer in 2s") {
		t.Fatalf("after its time:\n%s", m)
	}
	r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "t", "--body", r.body(t, "b"))
	if m := reason(runAt(t, t.TempDir(), stop(agyContinuations-1), e, "hook", "listen", "--timeout", "20s")); !strings.Contains(m, "handoff: new for reviewer:") || !strings.Contains(m, "#1 request W1") {
		t.Fatalf("with a request waiting, at the last continuation allowed:\n%s", m)
	}
	if b, _ := os.ReadFile(told); len(b) != 0 {
		t.Fatalf("the operator was told before the limit: %s", b)
	}
	start := time.Now()
	res := runAt(t, t.TempDir(), stop(agyContinuations), e, "hook", "listen", "--timeout", "20s")
	if res.code != 0 || strings.TrimSpace(res.out) != "{}" || time.Since(start) > 10*time.Second {
		t.Fatalf("at the limit: exit %d after %v, %q %q", res.code, time.Since(start), res.out, res.err)
	}
	if b, _ := os.ReadFile(told); !strings.Contains(string(b), "handoff can't wake reviewer's Antigravity session for new work until you type in it") {
		t.Fatalf("the operator wasn't told at the limit: %q", b)
	}
}

// settings antigravity prints the hooks for .agents/hooks.json, or with
// --global for ~/.gemini/config/hooks.json, and fixes no role.
func TestH32Settings(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	for _, global := range []bool{false, true} {
		args := []string{"settings", "antigravity"}
		where := "Put this in .agents/hooks.json in this project"
		if global {
			args, where = append(args, "--global"), "Put this in ~/.gemini/config/hooks.json, merged"
		}
		res := r.run(t, "", "", []string{"HOME=" + home}, args...)
		if res.code != 0 || !strings.HasPrefix(res.out, where) || !strings.Contains(res.out, "press Esc before you type to it") {
			t.Fatalf("settings %v: exit %d\n%s%s", args, res.code, res.out, res.err)
		}
		var hooks map[string]map[string][]struct {
			Type, Command string
			Timeout       int
		}
		if err := json.Unmarshal([]byte(res.out[strings.Index(res.out, "\n{"):]), &hooks); err != nil {
			t.Fatalf("settings %v: %v\n%s", args, err, res.out)
		}
		start, stop := hooks["handoff"]["SessionStart"], hooks["handoff"]["Stop"]
		if len(start) != 1 || !strings.HasSuffix(start[0].Command, " hook brief") || !strings.HasPrefix(start[0].Command, "exec ") ||
			len(stop) != 1 || !strings.HasSuffix(stop[0].Command, " hook listen") || stop[0].Timeout != 86400 || stop[0].Type != "command" {
			t.Fatalf("settings %v: %+v", args, hooks)
		}
	}
	r.refused(t, "", "an Antigravity session takes the role you give it", "settings", "antigravity", "--as", "reviewer")
}

// The brief an Antigravity session reads by hand offers Antigravity's hooks
// when neither its user nor its project hooks.json listens, and not when one
// does; a hook that is turned off doesn't count.
func TestH32TheBriefOffersAntigravitysHooks(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	agy := append(antigravity("0a1b-offer"), "HOME="+home)
	r.as(t, agy, "role", "reviewer")
	offer := "settings antigravity --global` prints them"
	if out := r.as(t, agy, "brief"); !strings.Contains(out, offer) || !strings.Contains(out, "Listening: Don't run `") {
		t.Fatalf("the brief with no hooks:\n%s", out)
	}
	listens := func(enabled bool) string {
		return `{"other": {"Stop": [{"command": "true"}]}, "handoff": {"enabled": ` + strconv.FormatBool(enabled) + `, "Stop": [{"type": "command", "command": "exec handoff hook listen", "timeout": 86400}]}}`
	}
	for _, c := range []struct {
		path, hooks string
		offered     bool
	}{
		{filepath.Join(r.dir, ".agents", "hooks.json"), listens(false), true},
		{filepath.Join(r.dir, ".agents", "hooks.json"), listens(true), false},
		{filepath.Join(home, ".gemini", "config", "hooks.json"), listens(true), false},
	} {
		os.MkdirAll(filepath.Dir(c.path), 0o755)
		if err := os.WriteFile(c.path, []byte(c.hooks), 0o644); err != nil {
			t.Fatal(err)
		}
		if out := r.as(t, agy, "brief"); strings.Contains(out, offer) != c.offered {
			t.Fatalf("with %s: offered %v, want %v\n%s", c.hooks, !c.offered, c.offered, out)
		}
		os.Remove(c.path)
	}
	// The operator's brief for a role names no CLI, and offers Claude Code's
	// and Codex's hooks only.
	if out := r.ok(t, "", "brief", "--for", "reviewer"); strings.Contains(out, "settings antigravity") || !strings.Contains(out, "settings codex --global") {
		t.Fatalf("the operator's brief for the reviewer:\n%s", out)
	}
}
