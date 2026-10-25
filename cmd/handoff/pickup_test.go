package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// H11: any project picks handoff up (proposal §17 row 72). init needs no
// flags and keeps the record out of git by itself. Row 80 (H16) names the
// default roles by job, not after the agents; roles named after the agents,
// as --roles claude,codex makes, can each implement or review.

// plainRepo makes a repository on branch with one commit, and no record.
func plainRepo(t *testing.T, branch string) *repo {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", branch)
	return &repo{dir: dir, base: commit(t, dir, "base")}
}

func readConfig(t *testing.T, r *repo) config {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, ".handoff", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestH11InitWithNoFlags(t *testing.T) {
	r := plainRepo(t, "main")
	out := r.ok(t, "", "init")
	c := readConfig(t, r)
	if want := []roleCfg{{"implementer", false}, {"reviewer", true}, {"reviewer-2", true}}; !slices.Equal(c.Roles, want) || c.Main != "main" {
		t.Fatalf("roles %+v, main %q; want implementer, reviewer and reviewer-2, on main", c.Roles, c.Main)
	}
	for _, want := range []string{"Roles: implementer, reviewer and reviewer-2; reviewer and reviewer-2 review.", "Main branch: main.",
		" settings claude\n", " settings codex\n", "Then give each session its role: tell it, such as \"you review\", and it runs ", "handoff role <role>.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("init printed no %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--as") {
		t.Errorf("init tells you to fix a CLI's role:\n%s", out)
	}

	// Git ignores the record, with something in it, and the project's own
	// files are as they were.
	r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer", "--commit", "HEAD", "--title", "W1 work", "--body", r.body(t, "please review"))
	if st := mustGit(t, r.dir, "status", "--porcelain", "--untracked-files=all"); st != "" {
		t.Fatalf("git status after init and a post:\n%s", st)
	}
	if st := mustGit(t, r.dir, "status", "--porcelain", "--ignored"); st != "!! .handoff/" {
		t.Fatalf("git status --ignored: %q; want only .handoff/", st)
	}
	if _, err := os.Stat(filepath.Join(r.dir, ".gitignore")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("init touched the project's .gitignore: %v", err)
	}
}

func TestH11InitFindsTheMainBranch(t *testing.T) {
	master := plainRepo(t, "master")
	both := plainRepo(t, "master")
	mustGit(t, both.dir, "branch", "main")
	trunk := plainRepo(t, "trunk")
	unborn := &repo{dir: t.TempDir()}
	mustGit(t, unborn.dir, "init", "-q", "-b", "dev")
	named := plainRepo(t, "main")
	for _, tc := range []struct {
		name string
		r    *repo
		args []string
		want string
	}{
		{"only master", master, nil, "master"},
		{"main and master", both, nil, "main"},
		{"neither, on trunk", trunk, nil, "trunk"},
		{"no commit yet", unborn, nil, "dev"},
		{"named", named, []string{"--main", "release"}, "release"},
	} {
		tc.r.ok(t, "", append([]string{"init"}, tc.args...)...)
		if got := readConfig(t, tc.r).Main; got != tc.want {
			t.Errorf("%s: main %q, want %q", tc.name, got, tc.want)
		}
	}
	detached := plainRepo(t, "trunk")
	mustGit(t, detached.dir, "switch", "-q", "--detach")
	detached.refused(t, "", "--main", "init")
}

func TestH11InitReviewers(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []roleCfg
	}{
		// Every role reviews but one named implementer.
		{[]string{"--roles", "implementer,reviewer,reviewer-2"}, []roleCfg{{"implementer", false}, {"reviewer", true}, {"reviewer-2", true}}},
		{[]string{"--roles", "alice,bob"}, []roleCfg{{"alice", true}, {"bob", true}}},
		{[]string{"--roles", "alice,bob", "--reviewers", "bob"}, []roleCfg{{"alice", false}, {"bob", true}}},
	} {
		r := plainRepo(t, "main")
		r.ok(t, "", append([]string{"init"}, tc.args...)...)
		if got := readConfig(t, r).Roles; !slices.Equal(got, tc.want) {
			t.Errorf("init %v: roles %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

// Row 80 changes row 73: a session no longer takes the role named after its
// CLI when it starts. Even where the roles are named after the agents, the
// settings give no session a role; the operator does.
func TestH16SettingsNeverPickARole(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init", "--roles", "claude,codex")
	out := r.ok(t, "", "settings", "claude")
	if strings.Contains(out, "HANDOFF_ROLE") || strings.Contains(out, "--prefer") || !strings.Contains(out, "takes the role you give it:\ntell it, and it runs handoff role <role>. Until then it has none.") {
		t.Errorf("settings claude:\n%s", out)
	}
	out = r.ok(t, "", "settings", "codex")
	for _, want := range []string{"handoff hook listen\"", "handoff hook brief\""} {
		if !strings.Contains(out, want) || strings.Contains(out, "--prefer") || strings.Contains(out, "HANDOFF_ROLE = ") {
			t.Errorf("settings codex printed no %q, or a role:\n%s", want, out)
		}
	}
	// --as still fixes the role.
	if out := r.ok(t, "", "settings", "claude", "--as", "codex"); !strings.Contains(out, `"HANDOFF_ROLE": "codex"`) {
		t.Errorf("settings claude --as codex:\n%s", out)
	}
}

// Either agent can implement, so each agent's brief says how to ask the
// other for a review, and every command it gives runs as printed.
func TestH11BriefCommandsRunForClaudeAndCodex(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init", "--roles", "claude,codex")
	mustGit(t, r.dir, "switch", "-q", "-c", "w1")
	r.commitA = commit(t, r.dir, "A")
	r.commitB = commit(t, r.dir, "B")
	req := idOf(t, r.ok(t, "codex", "post", "--item", "W1", "--to", "claude", "--commit", r.commitA, "--title", "W1 work", "--body", r.body(t, "please review W1")))
	fill := strings.NewReplacer("<id>", req, "<that sha>", r.commitA, "<sha>", r.commitB, "<file>", r.body(t, "a question"),
		"<role>", "codex", "<item>", "W2", "<title>", "W2 work", "<question>", "A question", "APPROVE|CHANGES|QUESTION", "APPROVE")
	ran, last, brief := briefCommands(t, r, "claude", fill)
	if got := strings.Join(ran, " "); got != "show claim reply note note post note" {
		t.Fatalf("ran %q from the brief:\n%s", got, brief)
	}
	if !strings.Contains(brief, "Either of you can implement or review; the operator says who does which, per task.") {
		t.Errorf("the brief doesn't say either can implement:\n%s", brief)
	}
	if !strings.Contains(string(last), "note W2 claude → operator: A question") {
		t.Fatalf("the question to the operator: %s", last)
	}
	if inbox := r.ok(t, "codex", "inbox"); !strings.Contains(inbox, "request W2 claude → codex: W2 work") {
		t.Fatalf("codex's inbox has no request from claude:\n%s", inbox)
	}
}

// With an implementer and reviewers, only the implementer is told how to
// ask for a review, and of whom.
func TestH11BriefTellsTheImplementerHowToAsk(t *testing.T) {
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	fill := strings.NewReplacer("<id>", req, "<sha>", r.commitB, "<file>", r.body(t, "a question"), "<role>", "reviewer",
		"<reviewer>", "reviewer-2", "<item>", "W2", "<title>", "W2 work", "<question>", "A question")
	ran, _, brief := briefCommands(t, r, "implementer", fill)
	if got := strings.Join(ran, " "); got != "show note post note" {
		t.Fatalf("ran %q from the brief:\n%s", got, brief)
	}
	if !strings.Contains(brief, "`, where <reviewer> is reviewer or reviewer-2.") || strings.Contains(brief, "can implement or review") {
		t.Errorf("the implementer's brief:\n%s", brief)
	}
	if b := r.ok(t, "reviewer", "brief"); strings.Contains(b, "To ask for a review") {
		t.Errorf("the reviewer is told to ask for reviews:\n%s", b)
	}
}

// H18 (row 83): handoff works only inside a git repository, since every
// request names a commit. Outside one, a command says so and how to make one,
// and keeps git's own words. Then init works, with no commit yet.
func TestH18OutsideGitSaysRunGitInit(t *testing.T) {
	r := &repo{dir: t.TempDir()}
	// Git looks no higher than the folder, and speaks English.
	e := []string{"GIT_CEILING_DIRECTORIES=" + filepath.Dir(r.dir), "LC_ALL=C"}
	for _, cmd := range []string{"init", "inbox", "brief"} {
		res := r.run(t, "", "", e, cmd)
		if res.code != 2 || !strings.Contains(res.err, "not inside a git repository, which handoff needs; run `git init` first") ||
			!strings.Contains(res.err, "fatal: not a git repository") {
			t.Errorf("handoff %s outside git: exit %d, stderr %q", cmd, res.code, res.err)
		}
	}
	if _, err := os.Stat(filepath.Join(r.dir, ".handoff")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("init outside git made a record: %v", err)
	}
	mustGit(t, r.dir, "init", "-q", "-b", "main")
	r.ok(t, "", "init")
}

// H18 (row 83): init names the CLI each settings command is for, and says to
// run only those in use. The name comes first, so the rest of the line is the
// command, and it runs as printed.
func TestH18InitNamesEachCLI(t *testing.T) {
	r := plainRepo(t, "main")
	out := r.ok(t, "", "init")
	if want := "To keep sessions on duty here, run the command for each CLI you use, and do what it prints:\n"; !strings.Contains(out, want) {
		t.Errorf("init printed no %q:\n%s", want, out)
	}
	for _, c := range []struct{ name, cli string }{{"Claude Code:", "claude"}, {"Codex:", "codex"}} {
		var line string
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "  "+c.name+" ") {
				line = strings.TrimSpace(strings.TrimPrefix(l, "  "+c.name))
			}
		}
		if !strings.HasSuffix(line, " settings "+c.cli) {
			t.Errorf("init printed no line for %s:\n%s", c.name, out)
			continue
		}
		sh := exec.Command("/bin/sh", "-c", line)
		sh.Dir, sh.Env = r.dir, env()
		if b, err := sh.CombinedOutput(); err != nil || !strings.Contains(string(b), "hook listen") {
			t.Errorf("init's `%s`: %v\n%s", line, err, b)
		}
	}
}

// H18 (row 84): a pointer to a repository that isn't there gets git's words
// alone, with no git init hint, since git init does not fix it: a linked
// worktree whose repository is gone, and GIT_DIR naming a missing path
// (the reviewer's P5 and P6, evidence/reviewer/H18-1c392de/probes.log).
func TestH18AMissingRepositoryGetsNoHint(t *testing.T) {
	gone := &repo{dir: t.TempDir()}
	missing := filepath.Join(t.TempDir(), "main", ".git", "worktrees", "gone")
	if err := os.WriteFile(filepath.Join(gone.dir, ".git"), []byte("gitdir: "+missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nowhere := &repo{dir: t.TempDir()}
	for _, tc := range []struct {
		name string
		r    *repo
		env  []string
	}{
		{"a worktree whose repository is gone", gone, nil},
		{"GIT_DIR naming a missing path", nowhere, []string{"GIT_DIR=" + filepath.Join(nowhere.dir, "nowhere")}},
	} {
		e := append([]string{"GIT_CEILING_DIRECTORIES=" + filepath.Dir(tc.r.dir), "LC_ALL=C"}, tc.env...)
		res := tc.r.run(t, "", "", e, "init")
		if res.code != 2 || !strings.Contains(res.err, "handoff init: not inside a git repository: ") ||
			!strings.Contains(res.err, "fatal: not a git repository: ") || strings.Contains(res.err, "git init") {
			t.Errorf("%s: exit %d, stderr %q; want git's words alone", tc.name, res.code, res.err)
		}
	}
}

// H19 (row 85): the implementer commits on a branch of its own without
// asking first, and asks for a review of work a note has findings on. A
// reviewer asked to look at work with no request sends what it finds to the
// implementer as a note. The commands are run as printed by TestH10Brief-
// CommandsRun and TestH11BriefCommandsRunForClaudeAndCodex.
func TestH19BriefSaysWhoCommitsAndWhereFindingsGo(t *testing.T) {
	r := newRepo(t)
	impl := r.ok(t, "implementer", "brief")
	for _, want := range []string{
		"To ask for a review, commit your work on a branch of its own, not on main; you don't need to ask the operator first. Then `",
		" If it has findings on work you haven't asked about, fix them, then ask for a review.\n",
	} {
		if !strings.Contains(impl, want) {
			t.Errorf("the implementer's brief has no %q:\n%s", want, impl)
		}
	}
	rev := r.ok(t, "reviewer", "brief")
	if want := "If the operator asks you to review work with no request, such as uncommitted changes, send what you find to implementer as a note: `"; !strings.Contains(rev, want) {
		t.Errorf("the reviewer's brief has no %q:\n%s", want, rev)
	}
	if strings.Contains(rev, "commit your work") || strings.Contains(rev, "findings on work you haven't asked about") {
		t.Errorf("the reviewer is told to commit:\n%s", rev)
	}
}

// H19 (row 85): in a new project, main has no commit yet. The project's first
// commit has no parent, so it is asked about with no base, and the review
// covers all of it. Row 86: so do its later rounds, which supersede a request
// with no base. Any other commit still needs main or --base, so a misnamed
// main is still caught.
func TestH19FirstCommitNeedsNoBase(t *testing.T) {
	r := &repo{dir: t.TempDir()}
	mustGit(t, r.dir, "init", "-q", "-b", "main")
	r.ok(t, "", "init")
	mustGit(t, r.dir, "switch", "-q", "-c", "w1")
	first := commit(t, r.dir, "first")
	req := idOf(t, r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer", "--commit", "HEAD", "--title", "W1 first", "--body", r.body(t, "please review")))
	if e := r.snap(t).Entries[0]; e.Commit != first || e.Base != "" {
		t.Fatalf("the first request: commit %.12s, base %q; want %.12s and no base", e.Commit, e.Base, first)
	}
	r.ok(t, "reviewer", "claim", req)
	r.ok(t, "reviewer", "reply", req, "--verdict", "CHANGES", "--commit", first, "--body", r.body(t, "1. fix this"))

	// Round 2: the fix, on top, supersedes the first request, with no --base.
	fix := commit(t, r.dir, "fix")
	req2 := idOf(t, r.ok(t, "implementer", "post", "--item", "W1", "--to", "reviewer", "--commit", fix, "--supersedes", req, "--title", "W1 round 2", "--body", r.body(t, "fixed")))
	snap := r.snap(t)
	if e := snap.Entries[len(snap.Entries)-1]; e.Commit != fix || e.Base != "" {
		t.Fatalf("round 2: commit %.12s, base %q; want %.12s and no base", e.Commit, e.Base, fix)
	}
	r.ok(t, "reviewer", "claim", req2)
	r.ok(t, "reviewer", "reply", req2, "--verdict", "APPROVE", "--commit", fix, "--body", r.body(t, "fine"))
	if inbox := r.ok(t, "", "inbox", "--for", "operator"); !strings.Contains(inbox, "W1") {
		t.Errorf("the approved first item doesn't await the operator:\n%s", inbox)
	}

	// Another item still needs main or --base, and so does its next round.
	later := commit(t, r.dir, "later")
	r.refused(t, "implementer", "A project's first request, on a commit with no parent, needs none, nor do its later rounds",
		"post", "--item", "W2", "--to", "reviewer", "--commit", later, "--title", "W2", "--body", r.body(t, "later work"))
	w2 := idOf(t, r.ok(t, "implementer", "post", "--item", "W2", "--to", "reviewer", "--commit", later, "--base", fix, "--title", "W2", "--body", r.body(t, "later work")))
	again := commit(t, r.dir, "again")
	r.refused(t, "implementer", "Pass --base",
		"post", "--item", "W2", "--to", "reviewer", "--commit", again, "--supersedes", w2, "--title", "W2 round 2", "--body", r.body(t, "again"))
}
