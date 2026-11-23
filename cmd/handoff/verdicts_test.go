package main

import (
	"strings"
	"testing"
)

// H37 (row 117): with two or more reviewers, the brief tells the role that
// asks for reviews to wait for every verdict on a commit before it posts the
// next round, so the operator needn't say it.
func TestH37TheBriefSaysToWaitForEveryVerdict(t *testing.T) {
	t.Parallel()
	rule := "With two or more reviewers on one commit, wait for every verdict on it before you post the next round, even after a CHANGES"
	r := newRepo(t) // implementer, reviewer and reviewer-2
	if out := r.ok(t, "implementer", "brief"); !strings.Contains(out, rule) {
		t.Fatalf("the implementer's brief, with two reviewers:\n%s", out)
	}
	if out := r.ok(t, "reviewer", "brief"); strings.Contains(out, rule) {
		t.Fatalf("a reviewer's brief has the rule:\n%s", out)
	}
	one := &repo{dir: t.TempDir()}
	mustGit(t, one.dir, "init", "-q", "-b", "main")
	commit(t, one.dir, "base")
	one.ok(t, "", "init", "--roles", "implementer,reviewer", "--reviewers", "reviewer", "--main", "main")
	if out := one.ok(t, "implementer", "brief"); strings.Contains(out, rule) {
		t.Fatalf("the implementer's brief, with one reviewer:\n%s", out)
	}
}
