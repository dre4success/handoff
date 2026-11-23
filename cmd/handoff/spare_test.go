package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// H35 part 1 (row 115): the spare, a session that holds no work, given by
// `handoff role spare` in a session with no history in the project
// (docs/proposals/H35_DESIGN.md).

// genOf is the generation of session id's binding.
func (r *repo) genOf(t *testing.T, id string) int64 {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "sessions", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		Role string `json:"role"`
		Gen  int64  `json:"gen"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	return b.Gen
}

func TestH35p1TheSpareHoldsNoWork(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	sp := claude("s-spare", cli(t))
	out := r.as(t, sp, "role", "spare")
	for _, want := range []string{
		"You are now the spare in ",
		"handoff brief: you are the spare in ",
		"implementer, reviewer and reviewer-2 hand work to each other through handoff, and the operator decides. You hold no work, and nothing is sent to you.",
		"Listening: ",
		"you'll be woken and told which. Then take it, as the wake says, with `",
		"post, reply, note and claim refuse the spare.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("role spare: no %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "When something arrives") || strings.Contains(out, "Waiting for you now") {
		t.Fatalf("the spare's brief has a role's duties:\n%s", out)
	}
	if b := r.as(t, sp, "brief"); !strings.Contains(b, "handoff brief: you are the spare in ") {
		t.Fatalf("brief: %s", b)
	}
	if in := r.as(t, sp, "inbox"); !strings.Contains(in, "inbox for spare\n\nNothing is sent to the spare") {
		t.Fatalf("inbox: %s", in)
	}

	// It writes nothing as the spare.
	for name, args := range map[string][]string{
		"post":  {"post", "--item", "W1", "--to", "reviewer", "--commit", r.commitA, "--title", "W1", "--body", r.body(t, "b")},
		"note":  {"note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "n")},
		"claim": {"claim", "1"},
		"reply": {"reply", "1", "--verdict", "APPROVE", "--commit", r.commitA, "--body", r.body(t, "ok")},
	} {
		if res := r.run(t, "", "", sp, args...); res.code == 0 || !strings.Contains(res.err, "the spare holds no work, so it writes nothing") {
			t.Fatalf("%s as the spare: exit %d %q", name, res.code, res.err)
		}
	}
	// Nothing is sent to it.
	if res := r.run(t, "implementer", "", nil, "note", "--to", "spare", "--item", "W1", "--body", r.body(t, "n")); res.code == 0 || !strings.Contains(res.err, "nothing is sent to the spare") {
		t.Fatalf("a note to the spare: exit %d %q", res.code, res.err)
	}
	// It is named only in its session.
	if res := r.run(t, "spare", "", nil, "inbox"); res.code == 0 || !strings.Contains(res.err, "--as spare and HANDOFF_ROLE=spare are refused") {
		t.Fatalf("HANDOFF_ROLE=spare: exit %d %q", res.code, res.err)
	}
	if res := r.run(t, "", "", nil, "roles", "add", "spare"); res.code == 0 || !strings.Contains(res.err, "spare is built in") {
		t.Fatalf("roles add spare: exit %d %q", res.code, res.err)
	}
	// It listens like any session, and nothing comes.
	if res := r.run(t, "", "", sp, "wait", "--timeout", "1s"); res.code != 3 || res.out != "" {
		t.Fatalf("the spare's wait: exit %d %q %q", res.code, res.out, res.err)
	}
	// Its SessionStart hook gives the spare's brief.
	if res := r.run(t, "", hookIn("s-spare", "SessionStart"), sp, "hook", "brief"); !strings.Contains(res.out, "handoff brief: you are the spare in ") {
		t.Fatalf("the hook's brief: %q %q", res.out, res.err)
	}
}

// role spare in the spare's own session changes nothing: a new binding
// would end the cover the away entry names by its generation (the reviewer,
// #744). In another session it moves the spare, as every role moves.
func TestH35p1RoleSpareAgainChangesNothingAndElsewhereMovesIt(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	a, b := claude("s-a", cli(t)), claude("s-b", cli(t))
	r.as(t, a, "role", "spare")
	gen := r.genOf(t, "s-a")
	if out := r.as(t, a, "role", "spare"); !strings.Contains(out, "You are the spare already in ") || !strings.Contains(out, "handoff brief: you are the spare in ") {
		t.Fatalf("role spare again:\n%s", out)
	}
	if g := r.genOf(t, "s-a"); g != gen {
		t.Fatalf("role spare again wrote a new generation: %d, was %d", g, gen)
	}
	if out := r.as(t, b, "role", "spare"); !strings.Contains(out, "You are now the spare in ") || !strings.Contains(out, "Another session was the spare; it has no role now.") {
		t.Fatalf("role spare elsewhere:\n%s", out)
	}
	if res := r.run(t, "", "", a, "inbox"); res.code == 0 || !strings.Contains(res.err, "you no longer hold spare") {
		t.Fatalf("the old spare: exit %d %q", res.code, res.err)
	}
	// The old spare may be the spare again: spare is no other role.
	if out := r.as(t, a, "role", "spare"); !strings.Contains(out, "You are now the spare in ") {
		t.Fatalf("the old spare, the spare again:\n%s", out)
	}
}

// A session that has held another role here can't be the spare, nor can
// one it went on in, nor another conversation in its Claude Code process
// (the reviewer, #655).
func TestH35p1OnlyASessionWithNoHistoryIsTheSpare(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	imp := claude("s-imp", pid)
	r.as(t, imp, "role", "implementer")
	refused := func(name string, sess []string, held string) {
		t.Helper()
		if res := r.run(t, "", "", sess, "role", "spare"); res.code == 0 || !strings.Contains(res.err, "this session has been "+held+" in ") || !strings.Contains(res.err, "so it can't be the spare") {
			t.Fatalf("%s: exit %d %q %q", name, res.code, res.out, res.err)
		}
	}
	refused("the implementer", imp, "implementer")
	// Another conversation in the same process, as after /clear with no
	// hook to carry the role.
	refused("a conversation after /clear", claude("s-imp-2", pid), "implementer")
	// The role given to another session: the first is marked, and still
	// has held it.
	r.as(t, codex("t-imp"), "role", "implementer")
	refused("the implementer, its role given on", imp, "implementer")
	// A new Codex thread with no history may.
	if out := r.as(t, codex("t-new"), "role", "spare"); !strings.Contains(out, "You are now the spare in ") {
		t.Fatalf("a new session:\n%s", out)
	}
}

// reviewer-2's #803: a wait takes a role's lock and moves its cursor, so
// only the spare's own session waits for the spare, and the spare waits
// for no other role. Reading an inbox stays open.
func TestH35p1TheSparesListenerIsItsOwn(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	if res := r.run(t, "", "", nil, "wait", "--for", "spare", "--timeout", "1s"); res.code == 0 || !strings.Contains(res.err, "only the spare's own session waits for the spare") {
		t.Fatalf("an unbound wait for the spare: exit %d %q", res.code, res.err)
	}
	r.request(t, "W1", r.commitA)
	if res := r.run(t, "", "", sp, "wait", "--for", "reviewer", "--timeout", "1s"); res.code == 0 || !strings.Contains(res.err, "the spare listens only for the spare, not for reviewer") || res.out != "" {
		t.Fatalf("the spare waiting for reviewer: exit %d %q %q", res.code, res.out, res.err)
	}
	if res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "1s"); !strings.Contains(res.out, "#1 request") {
		t.Fatalf("the reviewer's request was taken: %q %q", res.out, res.err)
	}
	if out := r.as(t, sp, "inbox", "--for", "reviewer"); !strings.Contains(out, "inbox for reviewer") {
		t.Fatalf("the spare reading reviewer's inbox:\n%s", out)
	}
	if res := r.run(t, "", "", nil, "inbox", "--for", "spare"); res.code != 0 || !strings.Contains(res.out, "Nothing is sent to the spare") {
		t.Fatalf("reading the spare's inbox: exit %d %q", res.code, res.err)
	}
}

// reviewer-2's and reviewer-3's #803, #804: the history check is made
// under the locks that write the binding. role spare held before its write
// while the same session is given implementer: the spare is refused, and
// the implementer binding stands.
func TestH35p1TheSpareCheckIsUnderTheBindingsLock(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := claude("s-1", cli(t))
	p, pause := pauseAt(t, "role")
	done := make(chan result, 1)
	go func() { done <- r.run(t, "", "", append(s, pause), "role", "spare") }()
	p.reached(t)
	r.as(t, s, "role", "implementer")
	p.release(t)
	if res := <-done; res.code == 0 || !strings.Contains(res.err, "this session has been implementer in ") {
		t.Fatalf("role spare after implementer was given: exit %d %q %q", res.code, res.out, res.err)
	}
	if out := r.as(t, s, "role"); !strings.Contains(out, "You are implementer in ") {
		t.Fatalf("the implementer binding:\n%s", out)
	}
}

// reviewer-3's #804: a move of another role to the session by /clear, cut
// short before its second write, leaves only the old id's mark naming it;
// that is its history too.
func TestH35p1AMoveCutShortToTheSessionIsHistory(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid := cli(t)
	r.as(t, claude("s-1", pid), "role", "implementer")
	cleared := claude("s-2", pid)
	r.run(t, "", hookIn("s-2", "SessionStart", "clear"), append(cleared, "HANDOFF_TEST_FAULT=move-between"), "hook", "brief")
	if _, err := os.Stat(filepath.Join(r.dir, ".handoff", "sessions", "s-2.json")); err == nil {
		t.Fatal("the move wasn't cut short")
	}
	if res := r.run(t, "", "", cleared, "role", "spare"); res.code == 0 || !strings.Contains(res.err, "this session has been implementer in ") {
		t.Fatalf("role spare after a cut move: exit %d %q %q", res.code, res.out, res.err)
	}
}

// reviewer-3's #804: the spare resumed after its binding was marked gone
// is told it is the spare already, and counts as running again.
func TestH35p1TheSpareResumedRunsAgain(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	pid, end := cliEnds(t)
	r.as(t, claude("s-spare", pid), "role", "spare")
	end()
	r.as(t, claude("s-rev", cli(t)), "role", "reviewer") // marks the ended spare gone
	if !strings.Contains(r.binding(t, "s-spare"), `"gone":true`) {
		t.Fatalf("the ended spare wasn't marked gone: %s", r.binding(t, "s-spare"))
	}
	gen := r.genOf(t, "s-spare")
	resumed := claude("s-spare", cli(t))
	if out := r.as(t, resumed, "role", "spare"); !strings.Contains(out, "You are the spare already in ") {
		t.Fatalf("role spare, resumed:\n%s", out)
	}
	if b := r.binding(t, "s-spare"); strings.Contains(b, `"gone":true`) || !strings.Contains(b, `"pid":`) || r.genOf(t, "s-spare") != gen {
		t.Fatalf("the resumed spare: %s, gen %d, was %d", b, r.genOf(t, "s-spare"), gen)
	}
}

// reviewer-3's #804: HANDOFF_ROLE and --as never name the spare, even in
// its own session; role with no role named says the spare.
func TestH35p1HandoffRoleAndAsNeverNameTheSpare(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if res := r.run(t, "spare", "", claude("s-new", cli(t)), "role", "spare"); res.code == 0 || !strings.Contains(res.err, "HANDOFF_ROLE=spare is refused") {
		t.Fatalf("HANDOFF_ROLE=spare role spare: exit %d %q %q", res.code, res.out, res.err)
	}
	sp := claude("s-spare", cli(t))
	r.as(t, sp, "role", "spare")
	if res := r.run(t, "implementer", "", sp, "role", "spare"); res.code == 0 || !strings.Contains(res.err, "fix its role to implementer") {
		t.Fatalf("HANDOFF_ROLE=implementer role spare in the spare: exit %d %q %q", res.code, res.out, res.err)
	}
	for _, args := range [][]string{{"inbox", "--as", "spare"}, {"brief", "--as", "spare"}} {
		if res := r.run(t, "", "", sp, args...); res.code == 0 || !strings.Contains(res.err, "--as spare and HANDOFF_ROLE=spare are refused") {
			t.Fatalf("%v in the spare: exit %d %q", args, res.code, res.err)
		}
	}
	if res := r.run(t, "spare", "", sp, "inbox"); res.code == 0 || !strings.Contains(res.err, "--as spare and HANDOFF_ROLE=spare are refused") {
		t.Fatalf("HANDOFF_ROLE=spare inbox in the spare: exit %d %q", res.code, res.err)
	}
	if out := r.as(t, sp, "role"); !strings.Contains(out, "You are the spare in ") {
		t.Fatalf("role:\n%s", out)
	}
}

// binding is session id's binding file, as written.
func (r *repo) binding(t *testing.T, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "sessions", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// The reviewer's #810: cover rests on a spare that can be woken, so
// handoff on its own lists a silent or blocked spare as it lists a seat; and
// fyi says the spare isn't told, since it holds no work.
func TestH35p1HandoffShowsTheSpareAndFyiSaysItIsntTold(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.as(t, claude("s-spare", cli(t)), "role", "spare")
	r.as(t, claude("s-imp", cli(t)), "role", "implementer")
	if out, err := r.atTerminal(t, "", nil, "fyi", "Carry on."); err != nil || !strings.Contains(out, "told implementer") || !strings.Contains(out, "the spare isn't told: it holds no work") {
		t.Fatalf("fyi: %v\n%s", err, out)
	}
	r.setQuiet(t, "1s")
	time.Sleep(1100 * time.Millisecond)
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "\n  spare: not listening now (never listened)\n") {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
}

// The reviewer's #810: an Antigravity spare at its cap is listed as
// blocked, in words for the spare.
func TestH35p1ABlockedSpareIsListed(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	agy := antigravity("ag-spare")
	r.as(t, agy, "role", "spare")
	runAt(t, t.TempDir(), agyIn("ag-spare", "Stop", r.dir, agyContinuations), agy, "hook", "listen", "--timeout", "2s")
	if home := r.run(t, "", "", nil); !strings.Contains(home.out, "\n  blocked: the spare can't be woken until you type in its session (Antigravity's 10 wakes used, since ") || !strings.Contains(home.out, "so it can't cover a stuck seat\n") {
		t.Fatalf("handoff on its own:\n%s", home.out)
	}
}

// reviewer-2's #817: a wait that began with no binding, for reviewer, held
// before a delivery while its session is made the spare, delivers nothing
// and retires; the reviewer's request stays for the reviewer. And with
// --for, --as spare and HANDOFF_ROLE=spare are refused in the spare too;
// wait --for spare from its own binding still runs.
func TestH35p1AWaitStopsWhenItsSessionBecomesTheSpare(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	s := claude("s-1", cli(t))
	r.request(t, "W1", r.commitA)
	p, pause := pauseAt(t, "deliver")
	done := make(chan result, 1)
	go func() { done <- r.run(t, "", "", append(s, pause), "wait", "--for", "reviewer", "--timeout", "10s") }()
	p.reached(t)
	r.as(t, s, "role", "spare")
	p.release(t)
	if res := <-done; strings.Contains(res.out, "#1 request") {
		t.Fatalf("the wait delivered after its session became the spare: exit %d %q", res.code, res.out)
	}
	if res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "2s"); !strings.Contains(res.out, "#1 request") {
		t.Fatalf("the reviewer's request was taken: %q %q", res.out, res.err)
	}
	for _, c := range []struct {
		role string
		args []string
	}{{"", []string{"wait", "--for", "spare", "--as", "spare", "--timeout", "1s"}}, {"spare", []string{"wait", "--for", "spare", "--timeout", "1s"}}, {"", []string{"inbox", "--for", "reviewer", "--as", "spare"}}} {
		if res := r.run(t, c.role, "", s, c.args...); res.code == 0 || !strings.Contains(res.err, "--as spare and HANDOFF_ROLE=spare are refused") {
			t.Fatalf("%q %v in the spare: exit %d %q", c.role, c.args, res.code, res.err)
		}
	}
	if res := r.run(t, "", "", s, "wait", "--for", "spare", "--timeout", "1s"); res.code != 3 {
		t.Fatalf("wait --for spare in the spare: exit %d %q", res.code, res.err)
	}
}
