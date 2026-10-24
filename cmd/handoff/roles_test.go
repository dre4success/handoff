package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// H16: roles add (proposal §17 row 80). A record made with roles named after
// the agents gets roles named by job, without changing an entry, and a
// session can then hold and work in a new role.
func TestH16RolesAddKeepsTheRecord(t *testing.T) {
	r := plainRepo(t, "main")
	r.ok(t, "", "init", "--roles", "claude,codex")
	r.ok(t, "codex", "post", "--item", "W1", "--to", "claude", "--commit", "HEAD", "--title", "W1 work", "--body", r.body(t, "please review"))
	channel := filepath.Join(r.dir, ".handoff", "channel.jsonl")
	before, err := os.ReadFile(channel)
	if err != nil {
		t.Fatal(err)
	}

	out := r.ok(t, "", "roles", "add", "implementer", "reviewer,reviewer-2")
	for _, want := range []string{"Added implementer, reviewer and reviewer-2.",
		"Roles: claude, codex, implementer, reviewer and reviewer-2; claude, codex, reviewer and reviewer-2 review.",
		"Give a session a role by telling it; it runs ", "handoff role <role>.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("roles add printed no %q:\n%s", want, out)
		}
	}
	want := []roleCfg{{"claude", true}, {"codex", true}, {"implementer", false}, {"reviewer", true}, {"reviewer-2", true}}
	if c := readConfig(t, r); !slices.Equal(c.Roles, want) || c.Main != "main" || c.QuietAfter != "30m" {
		t.Fatalf("config after roles add: %+v", c)
	}
	if after, _ := os.ReadFile(channel); !bytes.Equal(before, after) {
		t.Fatalf("roles add changed the record:\n%s\n%s", before, after)
	}

	// A session takes a new role, and work flows to and from it.
	rev := codex("t-rev")
	if out := r.as(t, rev, "role", "reviewer-2"); !strings.Contains(out, "You are now reviewer-2") {
		t.Fatalf("handoff role reviewer-2: %s", out)
	}
	req := idOf(t, r.ok(t, "implementer", "post", "--item", "W2", "--to", "reviewer-2", "--commit", "HEAD", "--title", "W2 work", "--body", r.body(t, "please review W2")))
	if inbox := r.as(t, rev, "inbox"); !strings.Contains(inbox, "request W2 implementer → reviewer-2: W2 work") {
		t.Fatalf("reviewer-2's inbox:\n%s", inbox)
	}
	r.as(t, rev, "claim", req)
	if inbox := r.ok(t, "", "inbox", "--for", "claude"); strings.Contains(inbox, "hash chain") || !strings.Contains(inbox, "request W1 codex → claude: W1 work") {
		t.Fatalf("the earlier entries after roles add:\n%s", inbox)
	}
}

func TestH16RolesAddRefuses(t *testing.T) {
	r := plainRepo(t, "main")
	r.refused(t, "", "handoff init", "roles", "add", "reviewer")
	r.ok(t, "", "init")
	config := filepath.Join(r.dir, ".handoff", "config.json")
	before, _ := os.ReadFile(config)
	for _, tc := range []struct {
		want string
		args []string
	}{
		{"roles needs add", nil},
		{"roles needs add", []string{"remove", "reviewer"}},
		{"needs a role", []string{"add"}},
		{"reviewer is already a role; the roles are implementer, reviewer, reviewer-2", []string{"add", "reviewer"}},
		{`role "reviewer-3" is listed twice`, []string{"add", "reviewer-3", "reviewer-3"}},
		{"operator is built in", []string{"add", "operator"}},
		{`role "Reviewer": use lower-case`, []string{"add", "Reviewer"}},
		{`role "": use lower-case`, []string{"add", "reviewer-3,"}},
	} {
		r.refused(t, "", tc.want, append([]string{"roles"}, tc.args...)...)
	}
	if after, _ := os.ReadFile(config); !bytes.Equal(before, after) {
		t.Fatalf("a refused roles add changed config.json:\n%s", after)
	}
}
