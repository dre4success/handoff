package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/dre4success/handoff/internal/status"
)

// cmdSettings prints the settings that keep a CLI's sessions on duty in
// this project (proposal §7, §17 rows 67, 73). It prints; it never edits a
// settings file (§9). With --as, every session of the CLI here has that
// role. Without it, each session takes the role the operator gives it (handoff role),
// and one the operator has given none has none: no session takes a role by
// itself (row 80). With --global, the settings go in the CLI's user
// settings and keep its sessions on duty in every project with a record, so
// a new project needs only init (row 87).
func (a *app) cmdSettings(args []string) (int, error) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	as := fs.String("as", "", "")
	global := fs.Bool("global", false, "")
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	if len(pos) != 1 || (pos[0] != "claude" && pos[0] != "codex") {
		return 0, errors.New("settings needs a CLI: claude or codex")
	}
	if *global {
		// Roles are each project's own, so no role is fixed for them all.
		if *as != "" {
			return 0, errors.New("--as fixes a role in one project; with --global, each session takes the role you give it")
		}
		if pos[0] == "codex" {
			return exitOK, a.printCodexGlobal(selfCommand())
		}
		return exitOK, a.printClaudeGlobal(selfCommand())
	}
	if err := a.open(); err != nil {
		return 0, err
	}
	if _, ok := a.cfg.role(*as); *as != "" && (!ok || *as == status.Operator) {
		return 0, fmt.Errorf("unknown role %q; the roles are %s", *as, a.roleNames())
	}
	if pos[0] == "codex" {
		return exitOK, a.printCodexSettings(selfCommand(), *as)
	}
	record := a.dir
	if real, err := filepath.EvalSymlinks(record); err == nil {
		record = real
	}
	b, err := json.MarshalIndent(claudeSettings(selfCommand(), *as, record), "", "  ")
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(a.stdout, `Put this in .claude/settings.local.json in this project, merged with anything
already there. That file stays out of git. Then start Claude Code here again.
%s It gets the brief when it starts or is summarised, a hook listens for its
work whenever its turn ends, and it runs handoff without asking, but never
decides, nor edits .handoff/. To set up every project at once, use --global.

%s
`, whose("Claude Code", *as), b)
	return exitOK, nil
}

// everywhere says what user settings do in a project that doesn't use
// handoff: nothing, since a hook acts only for a session with a role in a
// project with a record (row 67 (g)).
const everywhere = `They set up every project at once, and do nothing in a project with no
.handoff/ or a session with no role.`

// printClaudeGlobal prints Claude Code's settings for its user settings.
// They are claudeSettings with no role, and the rule against edits to the
// record names every .handoff/ folder, since no one record is known.
func (a *app) printClaudeGlobal(h string) error {
	b, err := json.MarshalIndent(claudeSettings(h, "", ""), "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, `Put these settings in %s, merged with any already there.
%s Sessions already running get them
when they start again.
In a project with a record, every Claude Code session takes the role you give
it: tell it, and it runs handoff role <role>. Until then it has none. It gets
the brief when it starts or is summarised, a hook listens for its work
whenever its turn ends, and it runs handoff without asking, but never decides,
nor edits a .handoff/ folder.

%s
`, tilde(filepath.Join(userConfig("claude"), "settings.json")), everywhere, b)
	return nil
}

// whose says where a CLI's sessions get their role.
func whose(cli, role string) string {
	if role != "" {
		return fmt.Sprintf("Every %s session in this project is then %s.", cli, role)
	}
	return fmt.Sprintf("Every %s session in this project then takes the role you give it:\ntell it, and it runs handoff role <role>. Until then it has none.", cli)
}

// claudeSettings are Claude Code settings: the session's role when it is
// fixed, the three hooks, and permission rules. h is the handoff command as
// the brief names it, so the rules match what the session types. record is
// the record's real, absolute path: a session in a linked worktree has its
// own settings, but every worktree shares the one record, so the rules name
// it in full ("//" starts an absolute path in Claude Code's rules). The
// Edit rule covers every tool that writes a file. With no record, for user
// settings, the rule names every .handoff/ folder ("//**/" is any depth from
// the root).
func claudeSettings(h, role, record string) map[string]any {
	hook := func(name string) map[string]any {
		return map[string]any{"type": "command", "command": h, "args": []string{"hook", name}}
	}
	// listen waits up to 23 hours; the CLI's limit for it is a day.
	listen := hook("listen")
	listen["asyncRewake"] = true
	listen["timeout"] = 24 * 60 * 60
	protect := "Edit(/" + record + "/**)"
	if record == "" {
		protect = "Edit(//**/.handoff/**)"
	}
	out := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{map[string]any{"hooks": []any{hook("brief"), listen}}},
			"Stop":         []any{map[string]any{"hooks": []any{listen}}},
			"Notification": []any{map[string]any{"matcher": "permission_prompt", "hooks": []any{hook("notify")}}},
		},
		"permissions": map[string]any{
			"allow": []string{"Bash(" + shellQuote(h) + " *)"},
			"deny":  []string{"Bash(" + shellQuote(h) + " decide *)", protect},
		},
	}
	if role != "" {
		out["env"] = map[string]string{"HANDOFF_ROLE": role}
	}
	return out
}

// printCodexSettings prints .codex/hooks.json and .codex/rules/handoff.rules,
// and, for a fixed role, .codex/config.toml (§17 rows 68, 71, 73).
// config.toml gives every command the session runs its fixed role, as Claude
// Code's env does, so a bare handoff, which the rule allows, never takes the
// session for the operator; a session with a role the operator gave it, or none, is known by
// CODEX_THREAD_ID and never is either. Codex runs a hook's command through a
// shell, so each starts with exec: the hook's parent is then Codex itself,
// and listen notices when Codex dies. Codex passes hooks its own
// environment, which may have no HANDOFF_ROLE, so each names a fixed role.
func (a *app) printCodexSettings(h, role string) error {
	b, err := codexHooks(h, role)
	if err != nil {
		return err
	}
	files, config := "these two files", ""
	if role != "" {
		files = "these three files"
		config = fmt.Sprintf(".codex/config.toml:\n[shell_environment_policy]\nset = { HANDOFF_ROLE = %s }\n\n", strconv.Quote(role))
	}
	fixed := "Without a role, handoff on its own never shows it your decisions."
	if role != "" {
		fixed = fmt.Sprintf("Every command it runs has HANDOFF_ROLE=%s, so handoff on its own\nshows its inbox, never your decisions.", role)
	}
	fmt.Fprintf(a.stdout, `Put %s in this project's .codex/ folder, merged with any already
there, and keep it out of git (add .codex/ to .git/info/exclude). Codex reads
them only in a project it trusts. The next time Codex starts here, it asks you
to review the new hooks: choose Trust all. To set up every project at once,
use --global.

%s %s
It gets the brief when its first turn starts and after a summary. When its turn
would end, a hook listens for its work and continues it with what arrives. And
handoff runs outside Codex's sandbox, so its notifications and pop-ups reach
you; handoff decide is forbidden.

%s.codex/hooks.json:
%s

.codex/rules/handoff.rules:
%s`, files, whose("Codex", role), fixed, config, b, codexRules(h))
	return nil
}

// printCodexGlobal prints Codex's hooks and rule for its user settings, with
// no role. Codex loads user hooks in every project, trusted or not, once the
// operator has trusted them.
func (a *app) printCodexGlobal(h string) error {
	b, err := codexHooks(h, "")
	if err != nil {
		return err
	}
	home := userConfig("codex")
	fmt.Fprintf(a.stdout, `Put these two files in %s/, merged with any already there.
%s The next time Codex starts, it asks you
to review the new hooks: choose Trust all. Sessions already running get them
when they start again.
In a project with a record, every Codex session takes the role you give it:
tell it, and it runs handoff role <role>. Until then it has none, and handoff
on its own never shows it your decisions. It gets the brief when its first
turn starts and after a summary. When its turn would end, a hook listens for
its work and continues it with what arrives. And handoff runs outside Codex's
sandbox, so its notifications and pop-ups reach you; handoff decide is
forbidden.

%s:
%s

%s:
%s`, tilde(home), everywhere, tilde(filepath.Join(home, "hooks.json")), b, tilde(filepath.Join(home, "rules", "handoff.rules")), codexRules(h))
	return nil
}

// codexHooks is .codex/hooks.json: the brief when a turn starts, and listen
// when one would end, each naming role when it is fixed.
func codexHooks(h, role string) ([]byte, error) {
	cmd := func(hook string) string {
		c := "exec " + shellQuote(h) + " hook " + hook
		if role != "" {
			c += " --as " + shellQuote(role)
		}
		return c
	}
	hooks := map[string]any{"hooks": map[string]any{
		"SessionStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": cmd("brief"), "timeout": 30}}}},
		// listen waits up to 23 hours; Codex's limit for it is a day.
		"Stop": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": cmd("listen"), "timeout": 24 * 60 * 60}}}},
	}}
	return json.MarshalIndent(hooks, "", "  ")
}

// codexRules is .codex/rules/handoff.rules: handoff runs outside the
// sandbox, and decide never runs.
func codexRules(h string) string {
	q := strconv.Quote(h)
	return fmt.Sprintf(`# handoff writes only under .handoff/ and has no network code (proposal §9).
prefix_rule(pattern=[%s], decision="allow", justification="handoff runs outside the sandbox so that it can notify the operator")
prefix_rule(pattern=[%s, "decide"], decision="forbidden", justification="The operator decides, not an agent")
`, q, q)
}
