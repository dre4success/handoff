package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// H35 part 2 (row 116): away asks about cover when a spare is bound; while
// the operator is away with cover, a stuck seat wakes the spare once.

// lastAway is the record's latest away entry.
func (r *repo) lastAway(t *testing.T) record.Entry {
	t.Helper()
	var got record.Entry
	for _, e := range r.snap(t).Entries {
		if e.Kind == "away" {
			got = e
		}
	}
	if got.ID == 0 {
		t.Fatal("no away entry")
	}
	return got
}

func TestH35p2AwayAsksAboutCover(t *testing.T) {
	t.Parallel()
	t.Run("no spare", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		out, err := r.atTerminal(t, "", nil, "away", "Carry on.")
		if err != nil || !strings.Contains(out, "No spare: if a seat gets stuck, you'll be told, as now. To have one, start a session and give it the role spare.") || strings.Contains(out, "Turn cover on?") || r.lastAway(t).Cover != nil {
			t.Fatalf("away: %v\n%s", err, out)
		}
	})
	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-spare", cli(t)), "role", "spare")
		out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on.")
		c := r.lastAway(t).Cover
		if err != nil || !strings.Contains(out, "Cover: on. The spare can take a seat that gets stuck while you're away") || c == nil || c.Session != "s-spare" || c.Gen != r.genOf(t, "s-spare") {
			t.Fatalf("away: %v, cover %+v\n%s", err, c, out)
		}
	})
	t.Run("no", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-spare", cli(t)), "role", "spare")
		out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "n"}}, "away", "Carry on.")
		if err != nil || !strings.Contains(out, "Away without cover: if a seat gets stuck, you'll be told, as now.") || r.lastAway(t).Cover != nil {
			t.Fatalf("away: %v\n%s", err, out)
		}
	})
	// With work the go-ahead approves now, that question comes first, and
	// each yes means one thing.
	t.Run("after the go-ahead's question", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.approved(t, "W1")
		r.as(t, claude("s-spare", cli(t)), "role", "spare")
		out, err := r.atTerminal(t, "", []step{{"Go ahead? [y/N]", "y"}, {"Turn cover on? [y/N]", "y"}}, "away", "Carry on.")
		if err != nil || !strings.Contains(out, "approved under it now: W1") || !strings.Contains(out, "Cover: on.") || r.lastAway(t).Cover == nil {
			t.Fatalf("away: %v\n%s", err, out)
		}
		if i, j := strings.Index(out, "Go ahead? [y/N]"), strings.Index(out, "Turn cover on? [y/N]"); i < 0 || j < i {
			t.Fatalf("the questions' order:\n%s", out)
		}
	})
	// The spare moves while the operator is asked: nothing is written.
	t.Run("the spare changed while asked", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-spare", cli(t)), "role", "spare")
		p, pause := pauseAt(t, "cover")
		done := make(chan string, 1)
		go func() {
			out, _ := r.atTerminalWith(t, []string{pause}, []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on.")
			done <- out
		}()
		p.reached(t)
		r.as(t, claude("s-other", cli(t)), "role", "spare")
		p.release(t)
		if out := <-done; !strings.Contains(out, "the spare changed while you were asked; nothing written; run it again") {
			t.Fatalf("away:\n%s", out)
		}
		for _, e := range r.snap(t).Entries {
			if e.Kind == "away" {
				t.Fatalf("an away entry was written: #%d", e.ID)
			}
		}
	})
}

// coverRepo is a repo with a spare and the operator away with cover, and
// what's needed to make its quiet_after short once set up.
func coverRepo(t *testing.T) (*repo, []string) {
	t.Helper()
	r := newRepo(t)
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	return r, sp
}

// An exact sign, at quiet_after: the reviewer's process ended with a
// request waiting. The spare's wait wakes it once, naming the seat, why,
// the work and how to take it; then not again.
func TestH35p2TheSpareIsWokenOnceForAStuckSeat(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-rev", pid), "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	req := r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
	time.Sleep(2100 * time.Millisecond)
	res := r.run(t, "", "", sp, "wait", "--timeout", "5s")
	want := "reviewer's seat is stuck: its session's process has ended. #" + req + " waits for it. Take it with `"
	if res.code != 0 || !strings.Contains(res.out, want) || !strings.Contains(res.out, " role reviewer`: the operator's cover is their word for it.") {
		t.Fatalf("the spare's wait: exit %d %q %q", res.code, res.out, res.err)
	}
	if res := r.run(t, "", "", sp, "wait", "--timeout", "2s"); res.code != 3 || res.out != "" {
		t.Fatalf("a second wake: exit %d %q", res.code, res.out)
	}
	// Its Stop hook wakes it the same way, here with nothing new.
	if res := r.run(t, "", hookIn("s-spare", "Stop"), sp, "hook", "listen", "--timeout", "2s"); strings.Contains(res.err, "seat is stuck") {
		t.Fatalf("the hook woke it again: %q", res.err)
	}
}

// Silence takes a reviewer seat at twice quiet_after, never the implementer
// seat; and only the spare the away entry names is woken.
func TestH35p2SilenceTakesOnlyAReviewerSeat(t *testing.T) {
	t.Parallel()
	t.Run("the implementer, silent", func(t *testing.T) {
		t.Parallel()
		r, sp := coverRepo(t)
		r.as(t, claude("s-imp", cli(t)), "role", "implementer")
		r.ok(t, "reviewer", "note", "--to", "implementer", "--item", "W1", "--body", r.body(t, "n"))
		r.setQuiet(t, "1s")
		time.Sleep(3100 * time.Millisecond)
		if res := r.run(t, "", "", sp, "wait", "--timeout", "2s"); res.code != 3 || res.out != "" {
			t.Fatalf("the spare woken for the implementer: exit %d %q", res.code, res.out)
		}
	})
	t.Run("a reviewer, silent", func(t *testing.T) {
		t.Parallel()
		r, sp := coverRepo(t)
		r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
		req := r.request(t, "W1", r.commitA)
		r.setQuiet(t, "1s")
		time.Sleep(3100 * time.Millisecond)
		res := r.run(t, "", "", sp, "wait", "--timeout", "5s")
		if res.code != 0 || !strings.Contains(res.out, "reviewer's seat is stuck: nothing has been heard from it for ") || !strings.Contains(res.out, "#"+req+" waits for it.") {
			t.Fatalf("the spare's wait: exit %d %q %q", res.code, res.out, res.err)
		}
	})
	t.Run("before twice quiet_after", func(t *testing.T) {
		t.Parallel()
		r, sp := coverRepo(t)
		r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
		r.request(t, "W1", r.commitA)
		r.setQuiet(t, "30s")
		if res := r.run(t, "", "", sp, "wait", "--timeout", "2s"); res.code != 3 || res.out != "" {
			t.Fatalf("woken early: exit %d %q", res.code, res.out)
		}
	})
	t.Run("a spare given after away", func(t *testing.T) {
		t.Parallel()
		r, _ := coverRepo(t)
		pid, end := cliEnds(t)
		r.as(t, claude("s-rev", pid), "role", "reviewer")
		r.request(t, "W1", r.commitA)
		later := claude("s-later", cli(t))
		r.as(t, later, "role", "spare")
		end()
		r.setQuiet(t, "1s")
		time.Sleep(2100 * time.Millisecond)
		if res := r.run(t, "", "", later, "wait", "--timeout", "2s"); res.code != 3 || res.out != "" {
			t.Fatalf("a spare the away entry doesn't name was woken: exit %d %q", res.code, res.out)
		}
	})
}

// The spare named in the away entry is still the spare after /clear, by
// its generation, and is woken in the session it went on in (reviewer-2,
// #727).
func TestH35p2TheSpareAfterClearIsStillWoken(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	r.as(t, claude("s-rev", revPID), "role", "reviewer")
	pid := cli(t)
	r.as(t, claude("s-spare", pid), "role", "spare")
	req := r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	cleared := claude("s-spare-2", pid)
	if out := r.run(t, "", hookIn("s-spare-2", "SessionStart", "clear"), cleared, "hook", "brief").out; !strings.Contains(out, "you are the spare in ") {
		t.Fatalf("the brief after /clear:\n%s", out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	if res := r.run(t, "", "", cleared, "wait", "--timeout", "5s"); res.code != 0 || !strings.Contains(res.out, "reviewer's seat is stuck") || !strings.Contains(res.out, "#"+req) {
		t.Fatalf("the spare after /clear: exit %d %q %q", res.code, res.out, res.err)
	}
}

// The reviewer's #822: once the spare the away entry names is no longer the
// spare, handoff says no seat is covered.
func TestH35p2HandoffSaysWhenCoverEnds(t *testing.T) {
	t.Parallel()
	r, _ := coverRepo(t)
	if home := r.run(t, "", "", nil); strings.Contains(home.out, "no seat is covered") {
		t.Fatalf("handoff with cover standing:\n%s", home.out)
	}
	r.as(t, claude("s-other", cli(t)), "role", "spare")
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "cover: the spare your go-ahead #") || !strings.Contains(home.out, "names is no longer the spare, so no seat is covered") {
		t.Fatalf("handoff after the spare moved:\n%s", home.out)
	}
}

// reviewer-2's #853: the wake goes through the listener's fence. The old
// spare's wait, held at its delivery while the spare goes on by /clear,
// prints nothing and leaves the wake unsent; the session it went on in gets
// it.
func TestH35p2TheWakeIsTheCurrentSparesOnly(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	r.as(t, claude("s-rev", revPID), "role", "reviewer")
	pid := cli(t)
	old := claude("s-spare", pid)
	r.as(t, old, "role", "spare")
	req := r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	p, pause := pauseAt(t, "deliver")
	done := make(chan result, 1)
	go func() { done <- r.run(t, "", "", append(old, pause), "wait", "--timeout", "10s") }()
	p.reached(t)
	cleared := claude("s-spare-2", pid)
	r.run(t, "", hookIn("s-spare-2", "SessionStart", "clear"), cleared, "hook", "brief")
	p.release(t)
	if res := <-done; strings.Contains(res.out, "seat is stuck") {
		t.Fatalf("the old spare's wait printed the wake: exit %d %q", res.code, res.out)
	}
	if res := r.run(t, "", "", cleared, "wait", "--timeout", "5s"); res.code != 0 || !strings.Contains(res.out, "reviewer's seat is stuck") || !strings.Contains(res.out, "#"+req) {
		t.Fatalf("the spare after /clear: exit %d %q %q", res.code, res.out, res.err)
	}
}

// reviewer-2's #853: a review done with, answered and approved, isn't work,
// though it is past the cursor of a reviewer that never listened: its seat
// isn't stuck.
func TestH35p2AReviewDoneWithIsntWork(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	rev := claude("s-rev", revPID)
	r.as(t, rev, "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	req := r.request(t, "W1", r.commitA)
	r.as(t, rev, "reply", req, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	if out, err := r.atTerminal(t, "", []step{{"Go ahead? [y/N]", "y"}, {"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	if res := r.run(t, "", "", sp, "wait", "--timeout", "2s"); res.code != 3 || res.out != "" {
		t.Fatalf("the spare woken for a review done with: exit %d %q", res.code, res.out)
	}
}

// reviewer-2's #853: the exact sign is the session's own process, as Claude
// Code gives it; a host process alone, a hook's parent, isn't.
func TestH35p2OnlyTheSessionsOwnProcessEnds(t *testing.T) {
	t.Parallel()
	p := exec.Command("true")
	if err := p.Run(); err != nil {
		t.Fatal(err)
	}
	dead := p.Process.Pid
	for _, c := range []struct {
		b    binding
		want bool
	}{
		{binding{PID: dead, CLI: "claude"}, true},
		{binding{Gone: true, CLI: "claude"}, true},
		{binding{Host: dead, CLI: "codex"}, false},
		{binding{Gone: true, CLI: "codex"}, false},
		{binding{PID: os.Getpid(), CLI: "claude"}, false},
	} {
		if got := c.b.ended(); got != c.want {
			t.Fatalf("%+v: ended %v, want %v", c.b, got, c.want)
		}
	}
}

// The reviewer's #854: away says, before its question, that a blocked spare
// can't cover, and gives an Antigravity spare its line; in a project with a
// role of its own named spare, it says nothing of a spare.
func TestH35p2AwaySaysWhatTheSpareCanDo(t *testing.T) {
	t.Parallel()
	t.Run("an Antigravity spare at its cap", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		agy := antigravity("ag-spare")
		r.as(t, agy, "role", "spare")
		runAt(t, t.TempDir(), agyIn("ag-spare", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "2s")
		out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "n"}}, "away", "Carry on.")
		i, j := strings.Index(out, "Note: the spare can't be woken until you type in its session ("), strings.Index(out, "Turn cover on? [y/N]")
		if err != nil || i < 0 || j < i || !strings.Contains(out, "The spare is an Antigravity session: it listens with a wait it runs in the background.") {
			t.Fatalf("away: %v\n%s", err, out)
		}
	})
	t.Run("a project with a role named spare", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		path := filepath.Join(r.dir, ".handoff", "config.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		cfg["roles"] = append(cfg["roles"].([]any), map[string]any{"name": "spare", "reviews": true})
		raw, _ = json.Marshal(cfg)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := r.atTerminal(t, "", nil, "away", "Carry on."); err != nil || strings.Contains(out, "No spare") {
			t.Fatalf("away: %v\n%s", err, out)
		}
	})
}

// The reviewer's #854: the spare's Stop hook gives the wake as it is, with
// no line about ids; and a spare whose process ended "isn't running".
func TestH35p2TheHookGivesTheWakeAsItIs(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	r.as(t, claude("s-rev", revPID), "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	res := r.run(t, "", hookIn("s-spare", "Stop"), sp, "hook", "listen", "--timeout", "5s")
	if !strings.Contains(res.err, "handoff: reviewer's seat is stuck: its session's process has ended.") || strings.Contains(res.err, "Read each with") {
		t.Fatalf("the spare's hook: exit %d %q", res.code, res.err)
	}
}

func TestH35p2HandoffSaysWhenTheSpareIsntRunning(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-spare", pid), "role", "spare")
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "names isn't running, so no seat is covered") {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
}

// reviewer-3's #855: a permission prompt takes a reviewer seat only once the
// prompt itself has stood for twice quiet_after, not when old work meets a
// fresh prompt.
func TestH35p2AFreshPromptIsNoSign(t *testing.T) {
	t.Parallel()
	r, sp := coverRepo(t)
	rev := claude("s-rev", cli(t))
	r.as(t, rev, "role", "reviewer")
	req := r.request(t, "W1", r.commitA)
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	r.run(t, "", `{"session_id":"s-rev","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow rm"}`, rev, "hook", "notify")
	if res := r.run(t, "", "", sp, "wait", "--timeout", "1s"); res.code != 3 || res.out != "" {
		t.Fatalf("woken for a fresh prompt: exit %d %q", res.code, res.out)
	}
	time.Sleep(2100 * time.Millisecond)
	if res := r.run(t, "", "", sp, "wait", "--timeout", "5s"); res.code != 0 || !strings.Contains(res.out, "reviewer's seat is stuck: its session is at a permission prompt: allow rm") || !strings.Contains(res.out, "#"+req) {
		t.Fatalf("the prompt, stood: exit %d %q %q", res.code, res.out, res.err)
	}
}

// reviewer-3's #855: with cover declined, a spare that changed while the
// operator was asked doesn't matter, and away is written.
func TestH35p2CoverDeclinedNeedsNoSameSpare(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.as(t, claude("s-spare", cli(t)), "role", "spare")
	p, pause := pauseAt(t, "cover")
	done := make(chan string, 1)
	go func() {
		out, _ := r.atTerminalWith(t, []string{pause}, []step{{"Turn cover on? [y/N]", "n"}}, "away", "Carry on.")
		done <- out
	}()
	p.reached(t)
	r.as(t, claude("s-other", cli(t)), "role", "spare")
	p.release(t)
	if out := <-done; !strings.Contains(out, "Away without cover") || r.lastAway(t).Cover != nil {
		t.Fatalf("away:\n%s", out)
	}
}

// reviewer-2's #862: the operator comes back while the spare's wait, which
// chose a stuck seat, is held before its delivery: the wake is dropped,
// printed to no one and not marked sent.
func TestH35p2BackDropsAWakeUnderWay(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	r.as(t, claude("s-rev", revPID), "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	p, pause := pauseAt(t, "deliver")
	done := make(chan result, 1)
	go func() { done <- r.run(t, "", "", append(sp, pause), "wait", "--timeout", "4s") }()
	p.reached(t)
	if out, err := r.atTerminal(t, "", nil, "back"); err != nil {
		t.Fatalf("back: %v\n%s", err, out)
	}
	p.release(t)
	if res := <-done; strings.Contains(res.out, "seat is stuck") {
		t.Fatalf("the wake was given after back: exit %d %q", res.code, res.out)
	}
	if entries, _ := os.ReadDir(filepath.Join(r.dir, ".handoff", "notified")); entries != nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "spare-") {
				t.Fatalf("the wake was marked sent: %s", e.Name())
			}
		}
	}
}

// heldWake sets up a reviewer seat stuck by its process ending, with what
// work sets up before, and runs the spare's wait held at its delivery; it
// returns the repo, the reviewer's session, and what releases the wait and
// gives its result.
func heldWake(t *testing.T, work func(r *repo)) (*repo, []string, func() result) {
	t.Helper()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	rev := claude("s-rev", revPID)
	r.as(t, rev, "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	work(r)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	p, pause := pauseAt(t, "deliver")
	done := make(chan result, 1)
	go func() { done <- r.run(t, "", "", append(sp, pause), "wait", "--timeout", "4s") }()
	p.reached(t)
	return r, rev, func() result { p.release(t); return <-done }
}

// spareMarked says whether any spare wake was marked sent.
func (r *repo) spareMarked(t *testing.T) bool {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(r.dir, ".handoff", "notified"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "spare-") {
			return true
		}
	}
	return false
}

// reviewer-2's #871: a record broken while the wake is held at its delivery
// refuses take's write, so the wake is dropped, unspent, for after the
// repair.
func TestH35p2ABrokenRecordKeepsTheWake(t *testing.T) {
	t.Parallel()
	r, _, release := heldWake(t, func(r *repo) { r.request(t, "W1", r.commitA) })
	f, err := os.OpenFile(filepath.Join(r.dir, ".handoff", "channel.jsonl"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, `{"v":1,"id":99,"ts":"2026-11-23T03:00:00Z","kind":"note","item":"W1","from":"reviewer","to":"implementer","body":"forged","prev":"broken"}`+"\n")
	f.Close()
	if res := release(); strings.Contains(res.out, "seat is stuck") || r.spareMarked(t) {
		t.Fatalf("the wake was given on a broken record: exit %d %q", res.code, res.out)
	}
}

// reviewer-2's #871: the wake says what waits as it stands under the fence:
// a note answered while it was held isn't named.
func TestH35p2TheWakeSaysWhatWaitsNow(t *testing.T) {
	t.Parallel()
	var first, second string
	r, rev, release := heldWake(t, func(r *repo) {
		first = idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "one")))
		second = idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "two")))
	})
	r.as(t, rev, "note", "--to", "implementer", "--re", first, "--body", r.body(t, "done"))
	res := release()
	if !strings.Contains(res.out, "#"+second+" waits for it.") || strings.Contains(res.out, "#"+first+" ") {
		t.Fatalf("the wake: exit %d %q", res.code, res.out)
	}
}

// The operator's choice, dropping part 3 (chat, 2026-11-23): woken for a
// stuck seat, the spare takes it with handoff role, the move that marks the
// old session; the operator is told once.
func TestH35p2TheSpareTakesTheSeatWithRole(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	revPID, end := cliEnds(t)
	rev := claude("s-rev", revPID)
	r.as(t, rev, "role", "reviewer")
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	req := r.request(t, "W1", r.commitA)
	if out, err := r.atTerminal(t, "", []step{{"Turn cover on? [y/N]", "y"}}, "away", "Carry on."); err != nil || !strings.Contains(out, "Cover: on.") {
		t.Fatalf("away: %v\n%s", err, out)
	}
	end()
	r.setQuiet(t, "1s")
	time.Sleep(2100 * time.Millisecond)
	if res := r.run(t, "", "", sp, "wait", "--timeout", "5s"); !strings.Contains(res.out, " role reviewer`") {
		t.Fatalf("the wake: exit %d %q", res.code, res.out)
	}
	sent, notify := sink(t)
	res := r.run(t, "", "", append(sp, notify...), "role", "reviewer")
	if res.code != 0 || !strings.Contains(res.out, "You are now reviewer in ") {
		t.Fatalf("role reviewer in the spare: exit %d %q %q", res.code, res.out, res.err)
	}
	if l := lineWith(t, sent, "the spare took reviewer, which was stuck. Its old session can't act as reviewer now. No spare is left."); l == "" {
		t.Fatalf("the notification: %q", sentTo(t, sent))
	}
	if out := r.as(t, sp, "inbox"); !strings.Contains(out, "#"+req+" request") {
		t.Fatalf("the new reviewer's inbox:\n%s", out)
	}
	if res := r.run(t, "", "", rev, "inbox"); res.code == 0 || !strings.Contains(res.err, "you no longer hold reviewer") {
		t.Fatalf("the old session: exit %d %q", res.code, res.err)
	}
}
