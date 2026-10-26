package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dre4success/handoff/internal/gitx"
)

// Whether a CLI's sessions have handoff's hooks (§17 row 87). handoff never
// writes a settings file (§9), but it reads them, so init and the brief can
// say what is missing. A CLI reads hooks from its user settings, for every
// project, and from the project's own. Hooks set elsewhere, such as in
// managed settings or Codex's config.toml, are not seen, so the brief still
// says "if".

// userConfig is the folder of a CLI's user settings: CLAUDE_CONFIG_DIR or
// ~/.claude for Claude Code, CODEX_HOME or ~/.codex for Codex.
func userConfig(cli string) string {
	env, dir := "CLAUDE_CONFIG_DIR", ".claude"
	if cli == "codex" {
		env, dir = "CODEX_HOME", ".codex"
	}
	if d := os.Getenv(env); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, dir)
}

// hookFiles are the files cli reads hooks from, for a session started at
// root: the user's first.
func hookFiles(cli, root string) []string {
	if cli == "codex" {
		return []string{filepath.Join(userConfig(cli), "hooks.json"), filepath.Join(root, ".codex", "hooks.json")}
	}
	return []string{filepath.Join(userConfig(cli), "settings.json"),
		filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".claude", "settings.local.json")}
}

// hookedIn is the first of cli's hook files, for a session started at
// root, whose Stop hooks run handoff's listen, or "" if none does.
func hookedIn(cli, root string) string {
	for _, f := range hookFiles(cli, root) {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s struct {
			Hooks map[string][]struct {
				Hooks []struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				} `json:"hooks"`
			} `json:"hooks"`
		}
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		for _, g := range s.Hooks["Stop"] {
			for _, h := range g.Hooks {
				if strings.Contains(h.Command+" "+strings.Join(h.Args, " "), "hook listen") {
					return f
				}
			}
		}
	}
	return ""
}

// hooked is hookedIn for a session in this worktree.
func (a *app) hooked(cli string) string {
	root, err := gitx.TopLevel(a.cwd)
	if err != nil {
		root = a.cwd
	}
	return hookedIn(cli, root)
}

// tilde writes a path under the home folder from ~.
func tilde(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
