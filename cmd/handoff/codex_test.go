package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// H10b: Codex sessions keep themselves going (proposal §17 row 68). A real
// Codex session driving these hooks is in evidence/h10b/.

// codexSettings returns what `settings codex` printed: hooks.json, parsed,
// the rules file's lines, and config.toml's lines.
func codexSettings(t *testing.T, r *repo, role string) (map[string][]struct {
	Hooks []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	} `json:"hooks"`
}, []string, []string) {
	t.Helper()
	res := r.run(t, "", "", nil, "settings", "codex", "--as", role)
	_, rest, ok1 := strings.Cut(res.out, ".codex/config.toml:\n")
	config, rest, ok2 := strings.Cut(rest, "\n\n.codex/hooks.json:\n")
	hooksJSON, rules, ok3 := strings.Cut(rest, "\n\n.codex/rules/handoff.rules:\n")
	if res.code != 0 || !ok1 || !ok2 || !ok3 {
		t.Fatalf("exit %d, stdout %q, stderr %q", res.code, res.out, res.err)
	}
	var file struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(hooksJSON), &file); err != nil {
		t.Fatalf("%v in\n%s", err, hooksJSON)
	}
	return file.Hooks, strings.Split(strings.TrimSpace(rules), "\n"), strings.Split(config, "\n")
}

// settings codex prints hooks.json and a rules file. Each hook names the
// role, since Codex passes hooks its own environment, and starts with exec,
// so the hook's parent is Codex. The rules let handoff run outside the
// sandbox and forbid decide.
func TestH10bSettingsCodex(t *testing.T) {
	r := newRepo(t)
	hooks, rules, config := codexSettings(t, r, "reviewer")
	h, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatal(err)
	}
	start, stop := hooks["SessionStart"], hooks["Stop"]
	if len(hooks) != 2 || len(start) != 1 || len(start[0].Hooks) != 1 || len(stop) != 1 || len(stop[0].Hooks) != 1 {
		t.Fatalf("hooks %+v", hooks)
	}
	if b := start[0].Hooks[0]; b.Type != "command" || b.Command != "exec "+h+" hook brief --as reviewer" || b.Timeout != 30 {
		t.Fatalf("SessionStart hook %+v", b)
	}
	if l := stop[0].Hooks[0]; l.Type != "command" || l.Command != "exec "+h+" hook listen --as reviewer" || l.Timeout != 86400 {
		t.Fatalf("Stop hook %+v", l)
	}
	q := strconv.Quote(h)
	want := []string{
		"# handoff writes only under .handoff/ and has no network code (proposal §9).",
		`prefix_rule(pattern=[` + q + `], decision="allow", justification="handoff runs outside the sandbox so that it can notify the operator")`,
		`prefix_rule(pattern=[` + q + `, "decide"], decision="forbidden", justification="The operator decides, not an agent")`,
	}
	if strings.Join(rules, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rules:\n%s", strings.Join(rules, "\n"))
	}
	if got := strings.Join(config, "\n"); got != "[shell_environment_policy]\nset = { HANDOFF_ROLE = \"reviewer\" }" {
		t.Fatalf("config.toml:\n%s", got)
	}
	r.refused(t, "", "settings needs a CLI: claude or codex", "settings", "cursor", "--as", "reviewer")
	// Without --as, no role is fixed: no config.toml, and hooks with no role
	// (row 73).
	out := r.ok(t, "", "settings", "codex")
	if strings.Contains(out, "config.toml:") || strings.Contains(out, "--as") || !strings.Contains(out, `hook listen",`) ||
		!strings.Contains(out, "Put these two files") || !strings.Contains(out, "takes the role you give it") {
		t.Fatalf("settings codex, no --as:\n%s", out)
	}
}

// Codex's own checker, where it is installed, reads the printed rules as
// meant: handoff is allowed, and handoff decide is forbidden.
func TestH10bCodexReadsTheRules(t *testing.T) {
	codex, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("codex is not installed")
	}
	if os.Getenv("CODEX_SANDBOX") != "" {
		t.Skip("inside Codex's sandbox")
	}
	r := newRepo(t)
	_, rules, _ := codexSettings(t, r, "reviewer")
	path := filepath.Join(t.TempDir(), "handoff.rules")
	if err := os.WriteFile(path, []byte(strings.Join(rules, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := filepath.EvalSymlinks(bin)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{h, "show", "1"}, "allow"},
		{[]string{h, "reply", "--as", "reviewer", "1", "--verdict", "APPROVE"}, "allow"},
		{[]string{h, "decide", "W1", "--approve"}, "forbidden"},
	} {
		out, err := exec.Command(codex, append([]string{"execpolicy", "check", "--pretty", "--rules", path, "--"}, c.args...)...).Output()
		if err != nil {
			t.Fatalf("codex execpolicy check %v: %v", c.args[1:], err)
		}
		var got struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(out, &got); err != nil || got.Decision != c.want {
			t.Fatalf("%v: decision %q (%v), want %q\n%s", c.args[1:], got.Decision, err, c.want, out)
		}
	}
}

// The brief names the role in every command that needs one, since a Codex
// session's commands may not see HANDOFF_ROLE.
func TestH10bBriefNamesTheRole(t *testing.T) {
	r := newRepo(t)
	res := r.run(t, "", "", nil, "hook", "brief", "--as", "reviewer")
	for _, want := range []string{"claim --as reviewer <id>", "reply --as reviewer <id>", "post --as reviewer ", "note --as reviewer --to <role>", "note --as reviewer --to operator"} {
		if res.code != 0 || !strings.Contains(res.out, want) {
			t.Fatalf("missing %q in\n%s", want, res.out)
		}
	}
}

func TestH10bShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/local/bin/handoff":  "/usr/local/bin/handoff",
		"/Users/a b/bin/handoff":  `'/Users/a b/bin/handoff'`,
		"/Users/it's/bin/handoff": `'/Users/it'\''s/bin/handoff'`,
		"":                        "''",
		"reviewer-2":              "reviewer-2",
		"/x/$HOME/handoff":        `'/x/$HOME/handoff'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

// With the role config.toml gives every command a Codex session runs, a bare
// handoff at a terminal shows the role's inbox and never asks the operator's question
// (GPT-6, H10b CHANGES #45; evidence/reviewer/H10b-418a0c4/bare.txt). Without
// it, the same command asks.
func TestH10bTheRoleKeepsBareHandoffFromTheOperatorsQuestion(t *testing.T) {
	r := newRepo(t)
	r.approved(t, "W1")
	_, _, config := codexSettings(t, r, "reviewer")
	role, ok := strings.CutPrefix(strings.Join(config, "\n"), "[shell_environment_policy]\nset = { HANDOFF_ROLE = ")
	if role, err := strconv.Unquote(strings.TrimSuffix(role, " }")); !ok || err != nil || role != "reviewer" {
		t.Fatalf("config.toml: %q", config)
	}
	out, err := r.atTerminal(t, "reviewer", nil)
	if err != nil || !strings.Contains(out, "inbox for reviewer") || strings.Contains(out, "Approve, deny or skip?") {
		t.Fatalf("with the role: %v\n%s", err, out)
	}
	if out, err := r.atTerminal(t, "", []step{{"Approve, deny or skip?", "s"}}); err != nil {
		t.Fatalf("without a role: %v\n%s", err, out)
	}
}
