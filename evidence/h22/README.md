# H22 evidence

`init` asks only for roles when both CLIs have handoff's hooks (§17 row 88).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-26, at the H22 commit named in each file's header, with `bash evidence/h22/collect.sh <patterns file>`. The patterns file is H14's, outside the repository. H22 is built on main after H21 (`b6d164e`), so "before" below is H21.

| File | What it shows |
|---|---|
| `outputs.log` | `init`, before and after, in a new project: with nothing set up, with Codex alone set up, and with both set up as `settings --global` says. Only the both-set-up output changes: it says "Both CLIs have them:" and "Give each session its role", not "Set them once, for every project:" and "Then give" |
| `go-test-race.log` | Every test under the race detector, with the CLIs' user settings in empty scratch folders. `TestH21SettingsGlobal` checks the both-set-up wording and the Codex-alone wording; `TestH11InitWithNoFlags` and `TestH18InitNamesEachCLI` check the nothing-set-up wording |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; no alert sent |
