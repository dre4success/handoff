package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// H34 part 5 (row 113): the quiet alert says why work waits.

// lineWith is the one sent line that contains each of want, or "".
func lineWith(t *testing.T, sent string, want ...string) string {
	t.Helper()
	for _, l := range sentTo(t, sent) {
		all := true
		for _, w := range want {
			all = all && strings.Contains(l, w)
		}
		if all {
			return l
		}
	}
	return ""
}

// The quiet alert ends with why its move waits: the session can't be
// woken, nothing is listening for it, or no session has the role; and with
// nothing when a listener is there. inbox and handoff on its own show the
// same lines, for when no listener runs to send them.
func TestH34p5TheQuietAlertSaysWhy(t *testing.T) {
	t.Parallel()
	t.Run("blocked, deaf and no session", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		agy := antigravity("ag-rev")
		r.as(t, agy, "role", "reviewer")
		runAt(t, t.TempDir(), agyIn("ag-rev", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "2s") // the cap
		r.as(t, claude("s-rev2", cli(t)), "role", "reviewer-2")                                                           // never listens
		w1 := r.request(t, "W1", r.commitA)
		w2 := idOf(t, r.ok(t, "implementer", "post", "--item", "W2", "--to", "reviewer-2", "--commit", r.commitA, "--title", "W2", "--body", r.body(t, "b")))
		w3 := r.request(t, "W3", r.commitA)
		r.ok(t, "reviewer", "reply", w3, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix")) // implementer owes, with no session
		r.setQuiet(t, "1s")                                                                                         // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		sent, notify := sink(t)
		r.run(t, "implementer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside")) // an alert pass
		for _, want := range [][]string{
			{"W1: reviewer hasn't picked up #" + w1, ", and its session can't be woken (Antigravity's 10 wakes used, since "},
			{"W2: reviewer-2 hasn't picked up #" + w2, ", and nothing is listening for it (never listened)"},
			{"W3: implementer hasn't answered the CHANGES on #" + w3, ", and no session has the role"},
		} {
			l := lineWith(t, sent, want...)
			if l == "" {
				t.Fatalf("no alert with %q: %q", want, sentTo(t, sent))
			}
			// The same line, its age aside, which may have grown since.
			has := func(out string) bool {
				for _, o := range strings.Split(out, "\n") {
					if strings.Contains(o, want[0]+" (") && strings.HasSuffix(o, want[1]+l[strings.LastIndex(l, want[1])+len(want[1]):]) {
						return true
					}
				}
				return false
			}
			if inbox := r.ok(t, "", "inbox", "--for", "operator"); !has(inbox) {
				t.Fatalf("inbox lacks %q:\n%s", l, inbox)
			}
			if home := r.run(t, "", "", nil); !has(home.out) {
				t.Fatalf("handoff on its own lacks %q:\n%s", l, home.out)
			}
		}
	})
	t.Run("listening", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		rev := claude("s-rev", cli(t))
		r.as(t, rev, "role", "reviewer")
		sent, notify := sink(t)
		// The listener's own pass sends the alert, so it shares the sink.
		r.start(t, "", "", append(rev, notify...), "wait", "--emit", "--timeout", "60s")
		r.listenedFor(t, "reviewer")
		w1 := r.request(t, "W1", r.commitA)
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		r.run(t, "implementer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
		l := lineWith(t, sent, "W1: reviewer hasn't picked up #"+w1)
		if l == "" || strings.Contains(l, ", and ") {
			t.Fatalf("the alert for a listening role: %q", sentTo(t, sent))
		}
	})
}

// A note undelivered past quiet_after alerts once, saying why; once its
// role's wait delivers it, nothing more.
func TestH34p5AnUndeliveredNoteAlerts(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	rev := claude("s-rev", cli(t))
	r.as(t, rev, "role", "reviewer")
	note := idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "look")))
	r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
	time.Sleep(2100 * time.Millisecond)
	sent, notify := sink(t)
	pass := func() {
		r.run(t, "implementer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
	}
	pass()
	want := "W1: reviewer hasn't been given the note #" + note + " from implementer ("
	if l := lineWith(t, sent, want, ", and nothing is listening for it (never listened)"); l == "" {
		t.Fatalf("no alert for the note: %q", sentTo(t, sent))
	}
	if res := r.run(t, "", "", rev, "wait", "--timeout", "5s"); !strings.Contains(res.out, "#"+note+" note") {
		t.Fatalf("the reviewer's wait: %s%s", res.out, res.err)
	}
	before := len(sentTo(t, sent))
	time.Sleep(1100 * time.Millisecond)
	pass()
	for _, l := range sentTo(t, sent)[before:] {
		if strings.Contains(l, "reviewer hasn't been given") {
			t.Fatalf("an alert after delivery: %q", l)
		}
	}
}

// No undelivered alert for a held APPROVE, an fyi naming a session the
// role's isn't, a note to a blocked role, a note its role's listener is
// delivering, or a note from before tracking began.
func TestH34p5NoUndeliveredAlertForAHeldApproveAForeignFyiOrAnOldNote(t *testing.T) {
	t.Parallel()
	t.Run("a held APPROVE", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-imp", cli(t)), "role", "implementer")
		out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
		first := idOf(t, strings.Split(strings.TrimSpace(out), "\n")[0])
		r.ok(t, "reviewer", "reply", first, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		sent, notify := sink(t)
		r.run(t, "reviewer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
		if l := lineWith(t, sent, "implementer hasn't been given"); l != "" {
			t.Fatalf("an alert for a held APPROVE: %q", l)
		}
		if l := lineWith(t, sent, "reviewer-2 hasn't picked up"); l == "" {
			t.Fatalf("the round's open request gave no alert: %q", sentTo(t, sent))
		}
	})
	t.Run("an fyi for a session gone", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-old", cli(t)), "role", "implementer")
		if _, err := r.atTerminal(t, "", nil, "fyi", "Carry on."); err != nil {
			t.Fatal(err)
		}
		r.as(t, claude("s-new", cli(t)), "role", "implementer")
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		sent, notify := sink(t)
		r.run(t, "reviewer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
		if l := lineWith(t, sent, "implementer hasn't been given your fyi"); l != "" {
			t.Fatalf("an alert for an fyi the role's session can't get: %q", l)
		}
	})
	t.Run("a note to a blocked role", func(t *testing.T) {
		// The block's alert named it as it arrived (the reviewer, #645).
		t.Parallel()
		r := newRepo(t)
		agy := antigravity("ag-rev")
		r.as(t, agy, "role", "reviewer")
		runAt(t, t.TempDir(), agyIn("ag-rev", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "2s") // the cap
		sent, notify := sink(t)
		note := idOf(t, r.run(t, "implementer", "", notify, "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "look")).out)
		if l := lineWith(t, sent, "work waits for it (#"+note+")"); l == "" {
			t.Fatalf("no block alert for the note: %q", sentTo(t, sent))
		}
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		r.run(t, "implementer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
		if l := lineWith(t, sent, "reviewer hasn't been given the note #"+note); l != "" {
			t.Fatalf("a second alert for a blocked role's note: %q", l)
		}
	})
	t.Run("a note its role's own listener is delivering", func(t *testing.T) {
		// The listener sends alerts as it starts, and gives the note in the
		// same second (the reviewer, #673).
		t.Parallel()
		r := newRepo(t)
		rev := claude("s-rev", cli(t))
		r.as(t, rev, "role", "reviewer")
		note := idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "look")))
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		sent, notify := sink(t)
		if res := r.run(t, "", "", append(rev, notify...), "wait", "--timeout", "5s"); !strings.Contains(res.out, "#"+note+" note") {
			t.Fatalf("the reviewer's wait: %s%s", res.out, res.err)
		}
		if l := lineWith(t, sent, "reviewer hasn't been given the note #"+note); l != "" {
			t.Fatalf("an alert from the listener that delivers it: %q", l)
		}
	})
	t.Run("a note from before tracking", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		r.as(t, claude("s-rev", cli(t)), "role", "reviewer")
		note := idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "old")))
		tracking := fmt.Sprintf(`{"v":1,"since":%s}`, note) // as if this shipped after the note
		if err := os.WriteFile(filepath.Join(r.dir, ".handoff", "tracking.json"), []byte(tracking), 0o644); err != nil {
			t.Fatal(err)
		}
		r.setQuiet(t, "1s") // only now, so no setup command finds an alert due
		time.Sleep(2100 * time.Millisecond)
		sent, notify := sink(t)
		r.run(t, "implementer", "", notify, "note", "--to", "reviewer-2", "--item", "W9", "--body", r.body(t, "aside"))
		if l := lineWith(t, sent, "reviewer hasn't been given the note #"+note); l != "" {
			t.Fatalf("an alert for a note from before tracking: %q", l)
		}
	})
}

// The operator's decision on one APPROVE says where the item's other
// requests stand, so approving it alone isn't taken for the whole item.
func TestH34p5TheDecideAlertNamesTheOthers(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	first, second := idOf(t, lines[0]), idOf(t, lines[1])
	r.ok(t, "reviewer-2", "reply", second, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix"))
	sent, notify := sink(t)
	r.run(t, "reviewer", "", notify, "reply", first, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	want := "W1: reviewer approved #" + first + "; reviewer-2 asked for changes on #" + second + ". Your decision."
	if l := lineWith(t, sent, want); l == "" {
		t.Fatalf("the decide alert: %q, want %q", sentTo(t, sent), want)
	}
}

// reviewer-2's #693: the pop-up the operator clicks in shows the dialog's own
// text, not the alert's, so it names the others too, as does the decision at
// a terminal. The title, the buttons and Later stay as they were.
func TestH34p5ThePopupAndTheTerminalNameTheOthers(t *testing.T) {
	t.Parallel()
	r := newPanelRepo(t)
	out := r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer,reviewer-2,reviewer-3", "--commit", r.commitA, "--title", "W1 work", "--body", r.body(t, "b"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	first, second, third := idOf(t, lines[0]), idOf(t, lines[1]), idOf(t, lines[2])
	r.ok(t, "reviewer-2", "reply", second, "--verdict", "CHANGES", "--commit", r.commitA, "--body", r.body(t, "fix"))
	r.ok(t, "reviewer", "reply", first, "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok"))
	others := "reviewer-2 asked for changes on #" + second + ", reviewer-3 hasn't picked up #" + third

	al := r.decideAlert(t)
	var shown []string
	if err := r.app(t).popup(al.Key, al.Target, clicks(t, &shown, click{button: "Later"})); err != nil {
		t.Fatal(err)
	}
	want := "handoff: " + filepath.Base(r.dir) + " | W1: reviewer approved #" + first + "; " + others + ".\n\nW1 work\n\nLater keeps it waiting: run handoff in the project to decide there. | Later,Deny,Approve"
	if len(shown) != 1 || shown[0] != want {
		t.Fatalf("the dialog:\n%q\nwant\n%q", shown, want)
	}
	if d, _ := r.decision(t, first); d != "" {
		t.Fatalf("Later wrote a decision %q", d)
	}

	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "\n  on W1: "+others+"\n") {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
	got, _ := r.atTerminal(t, "", []step{{"[y/N]", "n"}}, "decide", first, "--approve")
	if !strings.Contains(got, "\n  on W1: "+others+"\r\n") { // a terminal ends its lines with \r\n
		t.Fatalf("decide at a terminal:\n%s", got)
	}
	if d, _ := r.decision(t, first); d != "" {
		t.Fatalf("n wrote a decision %q", d)
	}
}
