package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// H31: a session whose run started outside the project is warned when it
// takes a role or reads its brief (proposal §17 rows 101, 103). Its hooks
// run in the folder it is in, and each start, resume, /clear and /background
// puts it back where this run started (evidence/h31/), so they don't reach
// the project then.

// cliAt is cli for a Claude Code process started in dir: Claude Code's
// process stays in the folder its run started in.
func cliAt(t *testing.T, dir string) int {
	t.Helper()
	pid, end := cliAtEnds(t, dir)
	t.Cleanup(end)
	return pid
}

func cliAtEnds(t *testing.T, dir string) (int, func()) {
	t.Helper()
	p := exec.Command("sleep", "300")
	p.Dir = dir
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	return p.Process.Pid, func() { once.Do(func() { p.Process.Kill(); p.Wait() }) }
}

// elsewhere is a folder in no git repository.
func elsewhere(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// rollout writes Codex's rollout for id under the home folder home: its
// session_meta, naming where the conversation began, with long instructions,
// then a turn_context for each run's folder, each followed by a turn of
// lines. It returns the rollout's path.
func rollout(t *testing.T, home, id, began string, runs ...string) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", "2026", "11", "15")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-15T21-00-00-"+id+".jsonl")
	meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{
		"id": id, "cwd": began, "base_instructions": map[string]string{"text": strings.Repeat("instructions ", 2000)},
	}})
	lines := []string{string(meta)}
	for _, cwd := range runs {
		turn, _ := json.Marshal(map[string]any{"type": "turn_context", "payload": map[string]any{"cwd": cwd, "turn_id": "t"}})
		lines = append(lines, string(turn), `{"type":"response_item","payload":{"type":"message","content":"done"}}`)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// add appends lines to a file.
func add(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
}

func warned(out string) bool {
	return strings.Contains(out, "handoff: warning: this session started in")
}

// The warning names the folder the run started in and the project, says
// when its hooks miss the project and what it misses, and offers the
// operator a resume from the project or a new session there. The role is
// taken all the same.
func TestH31RoleWarnsASessionStartedOutside(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	start := elsewhere(t)
	e := claude("s-out", cliAt(t, start))
	res := r.run(t, "", "", e, "role", "implementer")
	project := filepath.Base(r.dir)
	for _, want := range []string{
		start + ", outside " + project + ".",
		"Its hooks run outside " + project + " when it starts or resumes, and after /clear or /background, so it can miss its brief and the work sent to it.",
		"They can close this session and resume it from " + project + " (`cd ",
		" && claude --resume s-out`), which keeps this conversation, or start a session there and give it the role.",
	} {
		if res.code != 0 || !warned(res.out) || !strings.Contains(res.out, want) {
			t.Fatalf("role from a session started outside: exit %d, want %q\n%s%s", res.code, want, res.out, res.err)
		}
	}
	if !strings.HasPrefix(res.out, "You are now implementer in ") || !strings.Contains(res.out, "you are implementer") {
		t.Fatalf("the role and its brief, after the warning:\n%s", res.out)
	}
	if b, ok := boundTo(t, r, "s-out"); !ok || b.Role != "implementer" {
		t.Fatalf("the role wasn't taken: %+v", b)
	}
	if out := r.run(t, "", "", e, "role").out; !warned(out) || !strings.Contains(out, "You are implementer") {
		t.Fatalf("role, asked again:\n%s", out)
	}
	if out := r.run(t, "", "", e, "brief").out; !warned(out) || !strings.Contains(out, "you are implementer") {
		t.Fatalf("brief by hand:\n%s", out)
	}
}

// Where a run may start without a warning: the project, a folder in it,
// another worktree of it. Where it is warned: no repository, another
// repository with or without a record of its own.
func TestH31OnlyAStartOutsideIsWarned(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	sub := filepath.Join(r.dir, "web", "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "tree")
	mustGit(t, r.dir, "worktree", "add", "-q", tree, "main")
	other := t.TempDir()
	mustGit(t, other, "init", "-q", "-b", "main")
	recorded := newRepo(t)
	for _, c := range []struct {
		name, start string
		warn        bool
	}{
		{"the project", r.dir, false},
		{"a folder in it", sub, false},
		{"another worktree of it", tree, false},
		{"no repository", elsewhere(t), true},
		{"another repository", other, true},
		{"another project with a record", recorded.dir, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := r.run(t, "", "", claude("s-"+strings.ReplaceAll(c.name, " ", "-"), cliAt(t, c.start)), "role", "reviewer").out
			if warned(out) != c.warn {
				t.Fatalf("started in %s: warned %v, want %v\n%s", c.name, warned(out), c.warn, out)
			}
		})
	}
}

// The run counts, not where the conversation began: a session begun
// outside and resumed in the project isn't warned, and one begun in the
// project and resumed outside is (the reviewer's R4 and R6). In a hook,
// CLAUDE_PROJECT_DIR says where the run started, as after a /compact.
func TestH31TheRunCounts(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	out := elsewhere(t)
	if res := r.run(t, "", "", claude("s-back", cliAt(t, r.dir)), "role", "implementer"); warned(res.out) {
		t.Fatalf("resumed in the project:\n%s", res.out)
	}
	if res := r.run(t, "", "", claude("s-away", cliAt(t, out)), "role", "reviewer"); !warned(res.out) {
		t.Fatalf("resumed outside:\n%s", res.out)
	}
	e := claude("s-compact", cliAt(t, r.dir))
	r.run(t, "", "", e, "role", "reviewer-2")
	compact := hookIn("s-compact", "SessionStart", "compact")
	if res := r.run(t, "", compact, append(e, "CLAUDE_PROJECT_DIR="+out), "hook", "brief"); !warned(res.out) || !strings.Contains(res.out, "you are reviewer-2") {
		t.Fatalf("the brief after /compact, the run started outside:\n%s%s", res.out, res.err)
	}
	if res := r.run(t, "", compact, append(e, "CLAUDE_PROJECT_DIR="+r.dir), "hook", "brief"); warned(res.out) {
		t.Fatalf("the brief after /compact, the run started in the project:\n%s", res.out)
	}
}

// Codex's latest turn says where its run is: the session_meta's folder is
// where the conversation began (the reviewer's X1). A turn's lines can be
// long; a line that holds a turn_context deeper doesn't count; a hook's own
// rollout is read when it names the session; CLAUDE_PROJECT_DIR is Claude
// Code's alone. The words name Codex's resume.
func TestH31CodexIsWarnedToo(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	out := elsewhere(t)
	home := t.TempDir()
	codexIn := func(id string, extra ...string) []string {
		return append(append(codex(id), "CODEX_HOME="+home), extra...)
	}
	rollout(t, home, "019a-away", r.dir, r.dir, out)
	res := r.run(t, "", "", codexIn("019a-away"), "role", "reviewer")
	if !warned(res.out) || !strings.Contains(res.out, out+", outside ") || !strings.Contains(res.out, "Its hooks run outside "+filepath.Base(r.dir)+" always,") || !strings.Contains(res.out, "&& codex resume 019a-away`)") {
		t.Fatalf("a Codex run outside:\n%s%s", res.out, res.err)
	}
	rollout(t, home, "019a-back", out, out, r.dir)
	if res := r.run(t, "", "", codexIn("019a-back"), "role", "reviewer-2"); warned(res.out) {
		t.Fatalf("a Codex run resumed in the project:\n%s", res.out)
	}
	long := rollout(t, home, "019a-long", r.dir, out)
	add(t, long, `{"type":"response_item","payload":{"output":"`+strings.Repeat("x", 3<<20)+`"}}`,
		`{"type":"response_item","payload":{"type":"turn_context","cwd":`+quote(r.dir)+`}}`)
	if res := r.run(t, "", "", codexIn("019a-long"), "role", "implementer"); !warned(res.out) {
		t.Fatalf("a long last turn, and a line that holds a turn_context deeper:\n%s", res.out)
	}
	if res := r.run(t, "", "", codexIn("019a-back", "CLAUDE_PROJECT_DIR="+out), "role", "reviewer-2"); warned(res.out) {
		t.Fatalf("CLAUDE_PROJECT_DIR in a Codex session:\n%s", res.out)
	}
	own := rollout(t, t.TempDir(), "019a-hook", r.dir, out)
	another := rollout(t, t.TempDir(), "019a-another", out, out)
	hook := func(transcript string) string {
		in, _ := json.Marshal(map[string]string{"session_id": "019a-hook", "hook_event_name": "SessionStart", "source": "startup", "transcript_path": transcript})
		return string(in)
	}
	r.run(t, "", "", codexIn("019a-hook"), "role", "reviewer")
	if res := r.run(t, "", hook(own), codexIn("019a-hook"), "hook", "brief"); !warned(res.out) {
		t.Fatalf("the start brief, from the hook's rollout:\n%s%s", res.out, res.err)
	}
	if res := r.run(t, "", hook(another), codexIn("019a-hook"), "hook", "brief"); warned(res.out) {
		t.Fatalf("warned from another session's rollout:\n%s", res.out)
	}
}

// A project whose path a shell would split is quoted in the words.
func TestH31APathIsQuoted(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "my project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, "base")
	r := &repo{dir: dir}
	r.ok(t, "", "init", "--roles", "implementer,reviewer", "--reviewers", "reviewer", "--main", "main")
	out := r.run(t, "", "", claude("s-q", cliAt(t, elsewhere(t))), "role", "reviewer").out
	if !strings.Contains(out, "(`cd '") || !strings.Contains(out, "my project' && claude --resume s-q`)") {
		t.Fatalf("the words for a path with a space:\n%s", out)
	}
}

// When the folder can't be told, nothing is said: a process that has ended,
// a relative or vanished CLAUDE_PROJECT_DIR, no rollout, a rollout that is a
// named pipe (read without waiting), a last turn_context beyond 16 MiB from
// the end, and the operator's brief with --for.
func TestH31NoWarningWhenTheFolderIsntKnown(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	ended, end := cliAtEnds(t, elsewhere(t))
	end()
	if res := r.run(t, "", "", claude("s-ended", ended), "role", "reviewer"); res.code != 0 || warned(res.out) {
		t.Fatalf("a process that has ended: exit %d\n%s%s", res.code, res.out, res.err)
	}
	e := claude("s-hook", cliAt(t, r.dir))
	r.run(t, "", "", e, "role", "implementer")
	for _, dir := range []string{"..", filepath.Join(t.TempDir(), "gone")} {
		if res := r.run(t, "", hookIn("s-hook", "SessionStart", "compact"), append(e, "CLAUDE_PROJECT_DIR="+dir), "hook", "brief"); warned(res.out) {
			t.Fatalf("CLAUDE_PROJECT_DIR %s:\n%s", dir, res.out)
		}
	}
	home := t.TempDir()
	pipe := filepath.Join(home, "sessions", "2026", "11", "15", "rollout-2026-11-15T21-00-00-019a-pipe.jsonl")
	os.MkdirAll(filepath.Dir(pipe), 0o755)
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	far := rollout(t, home, "019a-far", r.dir, elsewhere(t))
	add(t, far, `{"type":"response_item","payload":{"output":"`+strings.Repeat("x", 17<<20)+`"}}`)
	for _, id := range []string{"019a-none", "019a-pipe", "019a-far"} {
		start := time.Now()
		res := r.run(t, "", "", append(codex(id), "CODEX_HOME="+home), "role", "reviewer")
		if res.code != 0 || warned(res.out) || time.Since(start) > 10*time.Second {
			t.Fatalf("Codex %s: exit %d after %v\n%s%s", id, res.code, time.Since(start), res.out, res.err)
		}
	}
	if out := r.run(t, "", "", []string{"CLAUDE_PROJECT_DIR=" + elsewhere(t)}, "brief", "--for", "implementer").out; warned(out) {
		t.Fatalf("the operator's brief --for was warned:\n%s", out)
	}
}

// Of two rollouts for one session, the latest written counts.
func TestH31TheLatestRolloutCounts(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	older := rollout(t, home, "019a-two", elsewhere(t), elsewhere(t))
	newer := filepath.Join(home, "sessions", "2026", "11", "16", "rollout-2026-11-16T09-00-00-019a-two.jsonl")
	os.MkdirAll(filepath.Dir(newer), 0o755)
	b, _ := os.ReadFile(rollout(t, t.TempDir(), "019a-two", r.dir, r.dir))
	os.WriteFile(newer, b, 0o600)
	then := time.Now().Add(-time.Hour)
	os.Chtimes(older, then, then)
	if out := r.run(t, "", "", append(codex("019a-two"), "CODEX_HOME="+home), "role", "reviewer").out; warned(out) {
		t.Fatalf("warned from the older rollout:\n%s", out)
	}
}

// A session id that isn't plain is bound by a hash of it. It is never read
// as a pattern that finds another session's rollout.
func TestH31AnIdIsNotAPattern(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	rollout(t, home, "019a-someone", elsewhere(t), elsewhere(t))
	if res := r.run(t, "", "", append(codex("*"), "CODEX_HOME="+home), "role", "reviewer"); res.code != 0 || warned(res.out) || !strings.Contains(res.out, "You are now reviewer") {
		t.Fatalf("role with the id *: exit %d\n%s%s", res.code, res.out, res.err)
	}
}

// An Antigravity session isn't looked for in Codex's rollouts, even one
// named for its id (rows 102, 104).
func TestH31AntigravityIsntLookedFor(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	rollout(t, home, "0a1b-agy", elsewhere(t), elsewhere(t))
	res := r.run(t, "", "", append(antigravity("0a1b-agy"), "CODEX_HOME="+home), "role", "reviewer")
	if res.code != 0 || warned(res.out) || !strings.Contains(res.out, "You are now reviewer") {
		t.Fatalf("role in an Antigravity session: exit %d\n%s%s", res.code, res.out, res.err)
	}
}

// A turn_context written with spaces after its colons is found (row 104).
func TestH31ASpacedTurnIsFound(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	home := t.TempDir()
	path := rollout(t, home, "019a-spaced", r.dir, r.dir)
	add(t, path, `{"type": "turn_context", "payload": {"cwd": `+quote(elsewhere(t))+`, "turn_id": "t"}}`)
	if res := r.run(t, "", "", append(codex("019a-spaced"), "CODEX_HOME="+home), "role", "reviewer"); !warned(res.out) {
		t.Fatalf("a spaced turn_context outside:\n%s%s", res.out, res.err)
	}
}
