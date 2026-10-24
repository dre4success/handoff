package main

// H8: the operator hears when the work goes quiet or needs a decision.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setQuiet rewrites quiet_after in config.json, as the operator may.
func (r *repo) setQuiet(t *testing.T, value string) {
	t.Helper()
	path := filepath.Join(r.dir, ".handoff", "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if value == "" {
		delete(c, "quiet_after")
	} else {
		c["quiet_after"] = value
	}
	b, _ = json.Marshal(c)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// sink returns a file that stands in for the operator's notifications, and the env
// that sends them there.
func sink(t *testing.T) (string, []string) {
	path := filepath.Join(t.TempDir(), "sent.txt")
	return path, []string{"HANDOFF_TEST_NOTIFY=" + path}
}

func sentTo(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestH8InitSetsQuietAfter(t *testing.T) {
	r := newRepo(t)
	b, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		QuietAfter string `json:"quiet_after"`
	}
	if err := json.Unmarshal(b, &c); err != nil || c.QuietAfter != "30m" {
		t.Fatalf("config.json = %s; want quiet_after 30m", b)
	}
}

// A record made before H8 has no quiet_after; it means 30 minutes. A value
// that is not a duration is refused, loudly, by every command.
func TestH8QuietAfterSetting(t *testing.T) {
	r := newRepo(t)
	r.setQuiet(t, "")
	r.ok(t, "", "inbox", "--for", "operator")
	r.setQuiet(t, "soon")
	r.refused(t, "", "quiet_after", "inbox", "--for", "operator")
	r.setQuiet(t, "-5m")
	r.refused(t, "", "quiet_after", "inbox", "--for", "operator")
}

// The trial's stall: a claim, then nothing. A running wait, here the
// implementer's, tells the operator once, naming the project.
func TestH8QuietClaimTellsTheOperatorOnce(t *testing.T) {
	r := newRepo(t)
	r.setQuiet(t, "1s")
	path, sinkEnv := sink(t)
	req := r.request(t, "W1", r.commitA)
	r.run(t, "reviewer", "", sinkEnv, "claim", req)
	res := r.run(t, "", "", sinkEnv, "wait", "--for", "implementer", "--timeout", "4s")
	if res.code != 3 {
		t.Fatalf("wait: exit %d, stderr %q", res.code, res.err)
	}
	got := sentTo(t, path)
	project := filepath.Base(r.dir)
	if len(got) != 1 || !strings.HasPrefix(got[0], project+": W1: reviewer claimed #1 and has been quiet for") {
		t.Fatalf("sent %q; want one alert about the claim, titled %q", got, project)
	}
	if !strings.Contains(res.err, "told the operator: W1: reviewer claimed #1") {
		t.Fatalf("wait's stderr %q; want the session told too", res.err)
	}
	// A second wait sees the same silence and stays quiet about it.
	r.run(t, "", "", sinkEnv, "wait", "--for", "implementer", "--timeout", "2s")
	if got := sentTo(t, path); len(got) != 1 {
		t.Fatalf("after a second wait, sent %q; want still one", got)
	}
}

// An APPROVE tells the operator at once, from the reply itself: after an APPROVE no
// agent may be waiting, as in the trial.
func TestH8ApproveTellsTheOperatorAtOnce(t *testing.T) {
	r := newRepo(t)
	path, sinkEnv := sink(t)
	req := r.request(t, "W1", r.commitA)
	res := r.run(t, "reviewer", "", sinkEnv, "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	if res.code != 0 {
		t.Fatalf("reply: exit %d, stderr %q", res.code, res.err)
	}
	got := sentTo(t, path)
	if len(got) != 1 || !strings.Contains(got[0], "W1: reviewer approved #1. Your decision") {
		t.Fatalf("sent %q; want one decide alert", got)
	}
	if !strings.Contains(res.err, "told the operator: W1: reviewer approved #1") {
		t.Fatalf("reply's stderr %q; want the reviewer told that the operator was told", res.err)
	}
	r.run(t, "implementer", "", sinkEnv, "note", "--item", "W1", "--to", "reviewer", "--body", r.body(t, "thanks"))
	if got := sentTo(t, path); len(got) != 1 {
		t.Fatalf("after another write, sent %q; want still one", got)
	}
}

// If the operator misses the notification, the operator's inbox shows what went quiet.
func TestH8InboxShowsWhatWentQuiet(t *testing.T) {
	r := newRepo(t)
	r.setQuiet(t, "1s")
	r.request(t, "W1", r.commitA)
	time.Sleep(1500 * time.Millisecond)
	out := r.ok(t, "", "inbox", "--for", "operator")
	if !strings.Contains(out, "gone quiet:\n  W1: reviewer hasn't picked up #1") {
		t.Fatalf("inbox:\n%s", out)
	}
}

// H8 CHANGES 2, the reviewer's probe through the real command: claim is
// idempotent, so claiming again after a restart writes nothing and does not
// restart the clock. The silence since the claim is real in the record; the operator
// is told once, and claiming again does not tell again.
func TestH8ClaimingAgainWritesNothingAndTellsOnce(t *testing.T) {
	r := newRepo(t)
	r.setQuiet(t, "1s")
	path, sinkEnv := sink(t)
	req := r.request(t, "W1", r.commitA)
	r.run(t, "reviewer", "", sinkEnv, "claim", req)
	time.Sleep(1500 * time.Millisecond)
	again := r.run(t, "reviewer", "", sinkEnv, "claim", req)
	if again.code != 0 || !strings.HasPrefix(again.out, "exists #2") {
		t.Fatalf("claim again: exit %d, out %q; want the first claim returned", again.code, again.out)
	}
	if n := len(r.snap(t).Entries); n != 2 {
		t.Fatalf("%d entries after claiming again; want 2", n)
	}
	r.run(t, "reviewer", "", sinkEnv, "claim", req)
	got := sentTo(t, path)
	if len(got) != 1 || !strings.Contains(got[0], "W1: reviewer claimed #1 and has been quiet for") {
		t.Fatalf("sent %q; want one alert about the silence since the claim", got)
	}
}
