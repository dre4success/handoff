# H16 evidence

No session takes a role by itself, `init` names the roles by job, and `roles add` gives a record more roles (§17 row 80).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-24, at the H16 commit named in each file's header, with `bash evidence/h16/collect.sh`. Alerts and sleep holds went to scratch files.

| File | What it shows |
|---|---|
| `go-test-race.log` | Every test under the race detector |
| `regressions.log` | The new and changed tests, run against main's code, then here. On main six of seven fail: a session takes the role named after its CLI, the settings pass `--prefer`, `init` makes `claude` and `codex`, and there is no `roles` command. The seventh, `TestH11BriefCommandsRunForClaudeAndCodex`, changed only to name its roles, so it passes on both. Here all pass |
| `demo.log` | A walk-through with this commit's `handoff` on `PATH`. A new project gets `implementer`, `reviewer` and `reviewer-2`, and settings with no role. Then a project made with `claude` and `codex`, as the operator's was, whose hooks still pass `--prefer`. A Claude Code session and a Codex session start there and get no role. `roles add` adds three roles, and the operator gives the three sessions (Opus, Fable, Codex) one each. A request then reaches `reviewer-2` |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean |

The sessions in `demo.log` are stood in for as the tests do: by the ids their CLI gives them, and a `sleep` for each Claude Code process.
