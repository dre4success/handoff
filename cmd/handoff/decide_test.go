package main

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// H9: the operator decides in one step (proposal §17 row 64).

// step answers one prompt: once the terminal shows after, type send.
type step struct{ after, send string }

// atTerminal runs handoff under script(1), which gives it a pseudo-terminal,
// and answers each prompt as the operator would, once it shows. It returns what the
// terminal showed and how the command exited.
func (r *repo) atTerminal(t *testing.T, role string, steps []step, args ...string) (string, error) {
	t.Helper()
	var extra []string
	if role != "" {
		extra = append(extra, "HANDOFF_ROLE="+role)
	}
	return r.atTerminalWith(t, extra, steps, args...)
}

// atTerminalWith is atTerminal with extra environment.
func (r *repo) atTerminalWith(t *testing.T, extra []string, steps []step, args ...string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script(1) not available")
	}
	cmd := exec.Command("script", append([]string{"-q", "/dev/null", bin}, args...)...)
	if out, _ := exec.Command("uname").Output(); strings.TrimSpace(string(out)) == "Linux" {
		quoted := []string{bin}
		for _, a := range args {
			quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", `'\''`)+"'")
		}
		cmd = exec.Command("script", "-qec", strings.Join(quoted, " "), "/dev/null")
	}
	cmd.Dir, cmd.Env = r.dir, env(extra...)
	// script(1) sends end-of-input to the terminal as soon as its own stdin
	// closes, so stdin stays open until the command is done.
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	timer := time.AfterFunc(20*time.Second, func() { cmd.Process.Kill() })
	defer timer.Stop()
	var out bytes.Buffer
	buf := make([]byte, 4096)
	seen := 0
	for _, s := range steps {
		for !strings.Contains(out.String()[seen:], s.after) {
			n, err := stdout.Read(buf)
			out.Write(buf[:n])
			if err != nil {
				t.Fatalf("handoff %v: no %q on the terminal: %v\n%s", args, s.after, err, out.String())
			}
		}
		seen = out.Len()
		io.WriteString(stdin, s.send+"\n")
	}
	rest, _ := io.ReadAll(stdout)
	out.Write(rest)
	return out.String(), cmd.Wait()
}

// approved posts a request on item and has the reviewer approve it.
func (r *repo) approved(t *testing.T, item string) string {
	t.Helper()
	req := r.request(t, item, r.commitA)
	r.ok(t, "reviewer", "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	return req
}

func (r *repo) decision(t *testing.T, re string) (decision, body string) {
	t.Helper()
	for _, e := range r.snap(t).Entries {
		if e.Kind == "decision" && strconv.FormatInt(e.Re, 10) == re {
			return e.Decision, e.Body
		}
	}
	return "", ""
}

// The operator names the item, not an entry number, gives no role and no note, and
// answers y.
func TestH9DecideByItemWithNoRole(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "W1", "--approve")
	if err != nil {
		t.Fatalf("decide W1: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Approve W1 (#"+req+")?") || !strings.Contains(out, "APPROVE on #"+req) {
		t.Fatalf("the question should name the item and show the approval:\n%s", out)
	}
	if d, body := r.decision(t, req); d != "approve" || body != "Approved." {
		t.Fatalf("decision %q with body %q; want approve, \"Approved.\"", d, body)
	}
}

func TestH9DenyWithANote(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "yes"}}, "decide", "W1", "--deny", "--note", "fix the docs first"); err != nil {
		t.Fatalf("decide: %v\n%s", err, out)
	}
	if d, body := r.decision(t, req); d != "deny" || body != "fix the docs first" {
		t.Fatalf("decision %q with body %q", d, body)
	}
}

func TestH9NotConfirmedWritesNothing(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	out, err := r.atTerminal(t, "", []step{{"[y/N]", ""}}, "decide", "W1", "--approve")
	if err == nil || !strings.Contains(out, "nothing written") {
		t.Fatalf("Enter alone should write nothing; err %v\n%s", err, out)
	}
	if d, _ := r.decision(t, req); d != "" {
		t.Fatalf("decision %q written without a yes", d)
	}
}

// An item with two things awaiting the operator is ambiguous; the ids are listed, and
// either id still works, with or without #.
func TestH9ItemWithTwoEntriesNeedsTheId(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	note := idOf(t, r.ok(t, "implementer", "note", "--item", "W1", "--to", "operator", "--title", "May I deploy?", "--body", r.body(t, "after the merge")))
	out, err := r.atTerminal(t, "", nil, "decide", "W1", "--approve")
	if err == nil || !strings.Contains(out, "#"+req+", #"+note) || !strings.Contains(out, "give the id") {
		t.Fatalf("want a refusal listing #%s and #%s; err %v\n%s", req, note, err, out)
	}
	if out, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "#"+note, "--approve"); err != nil {
		t.Fatalf("decide #%s: %v\n%s", note, err, out)
	} else if !strings.Contains(out, "| after the merge") {
		t.Fatalf("a note's body should be shown before deciding:\n%s", out)
	}
	if _, err := r.atTerminal(t, "", []step{{"[y/N]", "y"}}, "decide", "W1", "--approve"); err != nil {
		t.Fatalf("with one entry left, the item is enough: %v", err)
	}
}

// handoff on its own, at a terminal: each decision in turn, approve, deny or
// skip, with an optional note.
func TestH9HandoffOnItsOwnAsksAboutEach(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	note := idOf(t, r.ok(t, "implementer", "note", "--item", "W2", "--to", "operator", "--title", "May I deploy?", "--body", r.body(t, "to staging")))
	out, err := r.atTerminal(t, "", []step{
		{"[a/d/s]", "a"}, {"Note (optional):", "merge it"},
		{"[a/d/s]", "s"},
	})
	if err != nil {
		t.Fatalf("handoff: %v\n%s", err, out)
	}
	if d, body := r.decision(t, req); d != "approve" || body != "merge it" {
		t.Fatalf("W1: decision %q with body %q", d, body)
	}
	if d, _ := r.decision(t, note); d != "" {
		t.Fatalf("W2 was skipped, but decision %q was written", d)
	}
	if !strings.Contains(out, "May I deploy?") || !strings.Contains(out, "| to staging") {
		t.Fatalf("the note and its body should be shown:\n%s", out)
	}
	// The skipped note is still there next time; deny it, with no note.
	if out, err := r.atTerminal(t, "", []step{{"[a/d/s]", "d"}, {"Note (optional):", ""}}); err != nil {
		t.Fatalf("handoff: %v\n%s", err, out)
	}
	if d, body := r.decision(t, note); d != "deny" || body != "Denied." {
		t.Fatalf("W2: decision %q with body %q", d, body)
	}
	if out, _ := r.atTerminal(t, "", nil); !strings.Contains(out, "Nothing needs your decision.") {
		t.Fatalf("after both:\n%s", out)
	}
}

// Without a terminal it lists and writes nothing, so an agent's shell can
// read it but not decide.
func TestH9HandoffOnItsOwnWithoutATerminal(t *testing.T) {
	r := newRepo(t)
	req := r.approved(t, "W1")
	res := r.run(t, "", "a\n\n", nil)
	if res.code != 0 || !strings.Contains(res.out, "APPROVE on #"+req) || !strings.Contains(res.out, "in a terminal to decide") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", res.code, res.out, res.err)
	}
	if d, _ := r.decision(t, req); d != "" {
		t.Fatalf("decision %q written without a terminal", d)
	}
}

// In an agent's session, handoff on its own is that role's inbox.
func TestH9HandoffOnItsOwnInAnAgentSession(t *testing.T) {
	r := newRepo(t)
	r.request(t, "W1", r.commitA)
	res := r.h(t, "reviewer")
	if res.code != 0 || !strings.Contains(res.out, "inbox for reviewer") || !strings.Contains(res.out, "W1 work") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", res.code, res.out, res.err)
	}
}

func TestH9HandoffOnItsOwnOutsideAProject(t *testing.T) {
	cmd := exec.Command(bin)
	cmd.Dir, cmd.Env = t.TempDir(), env()
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !strings.Contains(string(out), "usage: handoff") || !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("want the usage and exit 2; err %v\n%s", err, out)
	}
}

// An agent's role is still refused, terminal or not; with no role and no
// terminal, the terminal is what's missing.
func TestH9DecideRefusals(t *testing.T) {
	r := newRepo(t)
	r.approved(t, "W1")
	r.refused(t, "", "terminal", "decide", "W1", "--approve")
	r.refused(t, "implementer", "operator", "decide", "W1", "--approve")
	r.refused(t, "", "operator", "decide", "W1", "--approve", "--as", "reviewer")
	if out, err := r.atTerminal(t, "reviewer", nil, "decide", "W1", "--approve"); err == nil || !strings.Contains(out, "for the operator") {
		t.Fatalf("an agent's role at a terminal: err %v\n%s", err, out)
	}
	if out, err := r.atTerminal(t, "", nil, "decide", "W1", "--approve", "--note", "x", "--body", r.body(t, "y")); err == nil || !strings.Contains(out, "not both") {
		t.Fatalf("--note with --body: err %v\n%s", err, out)
	}
	if out, err := r.atTerminal(t, "", nil, "decide", "W9", "--approve"); err == nil || !strings.Contains(out, "nothing on W9") {
		t.Fatalf("an item with nothing waiting: err %v\n%s", err, out)
	}
}
