package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// H11: any project picks handoff up (proposal §17 row 72). init needs no
// flags and keeps the record out of git by itself. The roles are named after
// the agents, so either can implement or review, and settings and the brief
// follow from that.

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
	if want := []roleCfg{{"claude", true}, {"codex", true}}; !slices.Equal(c.Roles, want) || c.Main != "main" {
		t.Fatalf("roles %+v, main %q; want claude and codex, both reviewing, on main", c.Roles, c.Main)
	}
	for _, want := range []string{"Roles: claude and codex. Either can implement or review.", "Main branch: main.", " settings claude\n", " settings codex\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("init printed no %q:\n%s", want, out)
		}
	}

	// Git ignores the record, with something in it, and the project's own
	// files are as they were.
	r.ok(t, "codex", "post", "--item", "W1", "--to", "claude", "--commit", "HEAD", "--title", "W1 work", "--body", r.body(t, "please review"))
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

// Row 73 changes row 72 (d): a session no longer has its CLI's role fixed;
// it takes the role named after its CLI when it starts, if that is free.
func TestH11SettingsTakeTheCLIsOwnRole(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init")
	out := r.ok(t, "", "settings", "claude")
	if strings.Contains(out, "HANDOFF_ROLE") || strings.Count(out, `"--prefer",`+"\n"+`              "claude"`) != 3 || !strings.Contains(out, "One that starts with no role takes\nclaude") {
		t.Errorf("settings claude:\n%s", out)
	}
	out = r.ok(t, "", "settings", "codex")
	for _, want := range []string{"hook listen --prefer codex", "hook brief --prefer codex"} {
		if !strings.Contains(out, want) || strings.Contains(out, "HANDOFF_ROLE = ") {
			t.Errorf("settings codex printed no %q, or a fixed role:\n%s", want, out)
		}
	}
	// --as still fixes the role.
	if out := r.ok(t, "", "settings", "claude", "--as", "codex"); !strings.Contains(out, `"HANDOFF_ROLE": "codex"`) || strings.Contains(out, "--prefer") {
		t.Errorf("settings claude --as codex:\n%s", out)
	}
}

// Either agent can implement, so each agent's brief says how to ask the
// other for a review, and every command it gives runs as printed.
func TestH11BriefCommandsRunForClaudeAndCodex(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init")
	mustGit(t, r.dir, "switch", "-q", "-c", "w1")
	r.commitA = commit(t, r.dir, "A")
	r.commitB = commit(t, r.dir, "B")
	req := idOf(t, r.ok(t, "codex", "post", "--item", "W1", "--to", "claude", "--commit", r.commitA, "--title", "W1 work", "--body", r.body(t, "please review W1")))
	fill := strings.NewReplacer("<id>", req, "<that sha>", r.commitA, "<sha>", r.commitB, "<file>", r.body(t, "a question"),
		"<role>", "codex", "<item>", "W2", "<title>", "W2 work", "<question>", "A question", "APPROVE|CHANGES|QUESTION", "APPROVE")
	ran, last, brief := briefCommands(t, r, "claude", fill)
	if got := strings.Join(ran, " "); got != "show claim reply note post note" {
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
