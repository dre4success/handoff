# H29 evidence: the public CLAUDE.md and README (§17 row 95)

Collected again by `collect.sh` at `902e36e`, for round 2, on the operator's Mac, 2026-11-11, by Opus 5.5 (implementer). Each log starts with its command, date, who ran it and the commit. `handoff version` reads `+dirty` because this folder was untracked while it ran.

| File | What it shows |
|---|---|
| `readme.log` | Every command in the README's table is in `handoff help`; the quick start as written, in a scratch project with a scratch home folder: `settings claude --global`, `settings codex --global`, `init`, a session given its role, `brief`, `fyi`, `roles add`, `handoff`, `version`, `init --roles a,b` |
| `docs.log` | How many lines CLAUDE.md and the README add and remove, and what CLAUDE.md drops, in row 95's words, not the dropped lines themselves (row 97); the README at 72 lines instead of 135; the files changed |
| `sweep.log` | The name sweep of every tracked file, before and after, by `sweep.py`; the patterns stay outside the repository. The other projects' names go from 3 lines to 0, and no added line matches anything |
| `go-test-race.log` | Every test under the race detector: 171 pass, 0 fail |
| `deps-vet-fmt.log` | None of the 16 lines CLAUDE.md removes is in any file here, word for word; no home path or email address added (the 3 added lines it counts are `collect.sh`'s own: its scratch home folder and its search pattern); no dependencies, no `net`; vet and gofmt clean; no alert sent |
