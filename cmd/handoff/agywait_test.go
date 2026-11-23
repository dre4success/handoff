package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// H34.8 (row 114): an Antigravity session listens by running handoff wait
// in the background, as the probe found it can, past the 10 wakes its Stop
// hook has; the hook is the safety net.
func TestH34p8AntigravityListensWithABackgroundWait(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := append(antigravity("0a1b-wait"), "HOME="+t.TempDir())
	r.as(t, agy, "role", "reviewer")
	wait := " wait --for reviewer --timeout 60m` in the background. Exit 3: run it again."
	how := "In Antigravity, that is `run_command` with a short `WaitMsBeforeAsync`, so the command goes to the background and its exit wakes you with what it printed."
	listening := func(out string) string {
		i := strings.Index(out, "Listening:")
		if i < 0 {
			t.Fatalf("no listening line:\n%s", out)
		}
		return strings.SplitN(out[i:], "\n", 2)[0]
	}
	// With no hooks: the wait, and the offer of hooks.
	if l := listening(r.as(t, agy, "brief")); !strings.Contains(l, wait) || !strings.Contains(l, how) || strings.Contains(l, "Don't run") {
		t.Fatalf("with no hooks: %s", l)
	}
	// The brief its SessionStart hook gives, and the one handoff role
	// prints: the wait still, and the hook as the safety net.
	net := "a hook listens for reviewer, but it can wake you only 10 times before the operator types in this session."
	var start struct {
		InjectSteps []struct {
			SystemMessage struct {
				SystemMessage string `json:"systemMessage"`
			} `json:"systemMessage"`
		} `json:"injectSteps"`
	}
	hookBrief := func(env []string) string {
		res := runAt(t, t.TempDir(), agyIn("0a1b-wait", "SessionStart", r.dir, 0), env, "hook", "brief")
		if json.Unmarshal([]byte(res.out), &start) != nil || len(start.InjectSteps) != 1 {
			t.Fatalf("the SessionStart hook: %q %q", res.out, res.err)
		}
		return start.InjectSteps[0].SystemMessage.SystemMessage
	}
	for name, out := range map[string]string{
		"the hook's brief": hookBrief(agy),
		// reviewer-3's #710: Antigravity may name the session on stdin
		// alone, with no ANTIGRAVITY_CONVERSATION_ID.
		"the hook's brief, the session named only on stdin": hookBrief([]string{"HOME=" + t.TempDir()}),
		"handoff role": r.as(t, agy, "role", "reviewer"),
	} {
		l := listening(out)
		if !strings.Contains(l, wait) || !strings.Contains(l, how) || !strings.Contains(l, "If none runs when your turn ends") || !strings.Contains(l, net) || strings.Contains(l, "you don't run") {
			t.Fatalf("%s: %s", name, l)
		}
	}
	// Claude Code's and Codex's lines don't change.
	if l := listening(r.as(t, claude("s-1", cli(t)), "brief", "--for", "reviewer")); strings.Contains(l, "Antigravity") {
		t.Fatalf("Claude Code's: %s", l)
	}
}

// When the Stop hook does wake an Antigravity session, it says to start the
// background wait; a Claude Code session's wake doesn't.
func TestH34p8TheHooksWakeSaysToWaitInTheBackground(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-imp")
	r.as(t, agy, "role", "implementer")
	r.ok(t, "reviewer", "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "n"))
	res := runAt(t, t.TempDir(), agyIn("ag-imp", "Stop", r.dir, 0), agy, "hook", "listen", "--timeout", "2s")
	var o struct{ Reason string }
	json.Unmarshal([]byte(res.out), &o)
	if !strings.Contains(o.Reason, "wait --for implementer --timeout 60m` in the background, as your brief says, so this hook needn't wake you.") {
		t.Fatalf("the Antigravity wake: %q", res.out)
	}
	c := claude("s-rev", cli(t))
	r.as(t, c, "role", "reviewer")
	r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "m"))
	if w := r.run(t, "", hookIn("s-rev", "Stop"), c, "hook", "listen", "--timeout", "2s"); strings.Contains(w.err, "in the background, as your brief says") {
		t.Fatalf("a Claude Code wake: %q", w.err)
	}
}
