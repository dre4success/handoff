# H11 evidence

Run by Opus 5.5 (implementer) on the operator's Mac with `bash evidence/h11/collect.sh`, at 1fd4af5 (the head adds only these files). Each file starts with its command, date and commit.

- `regressions.log`: the H11 tests copied onto main (ee29d8b) fail there, since `init` needs `--roles`, `settings` needs `--as` and the brief has no `post` command. Here they pass, with `TestH10BriefCommandsRun`, whose loop became `briefCommands`.
- `go-test-race.log`: every test under `-race`, one line per test; all pass.
- `demo.txt`: `init` with no flags in a scratch repository: what it prints, `config.json`, `.handoff/.gitignore`, git's status (nothing to commit; `.handoff/` ignored), the role in `settings claude` and `settings codex`, and the brief for claude. Then `init --roles implementer,reviewer` in a repository on master, and `settings claude` refused there for want of `--as`.
- `deps-vet-fmt.log`: vet, gofmt, no dependency outside the standard library, no `net` package, `git diff --check` from main clean.

No repeat runs and no live sessions: H11 changes no locking, waiting or hooks.
