# H18 evidence

Outside a git repository, handoff says to run `git init`; `init` names the CLI each settings command is for; the README says a project must be a git repository (§17 row 83).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-25 (round 2), at the H18 commit named in each file's header, with `bash evidence/h18/collect.sh <patterns file>`. The patterns file is H14's, outside the repository.

| File | What it shows |
|---|---|
| `go-test-race.log` | Every test under the race detector, the H18 tests among them: `TestH18OutsideGitSaysRunGitInit`, `TestH18InitNamesEachCLI`, `TestH18AMissingRepositoryGetsNoHint` and `TestH18NoRepositoryMeansGitInitFixesIt` |
| `outputs.log` | What a user sees, built from main and from here: `init`, `inbox` and `brief` outside any repository, then `git init` and `init`. Round 2 (§17 row 84): `init` in a worktree whose repository is gone, and with `GIT_DIR` naming a missing path: git's words alone, from main and from here alike. Round 1 (`1c392de`) gave the `git init` hint there, as the reviewer's P5 and P6 show. Then each command `init` now prints, with the CLI's name taken off, run through `sh` as printed: both exit 0 |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; no alert sent |
