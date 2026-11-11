# H25 evidence

A README fix, and a correction to row 90 (b), with no change to handoff (§17 row 91).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-27, at the H25 commit named in each file's header, with `bash evidence/h25/collect.sh <patterns file>`. The patterns file is H14's, outside the repository. H25 is built on main after H24 (`7c5b98a`).

| File | What it shows |
|---|---|
| `readme.log` | The README's change. Every `handoff settings` command the README names, run in a scratch project with scratch settings folders: each prints its settings, exit 0. The one it named before, `handoff settings --global`: refused, exit 2. The brief's offer to a session with no hooks names the same two commands the README now gives |
| `h12a-under-load.log` | `TestH12ACodexThreadKeepsItsRoleAlone`, which runs a listener to its 1 s timeout in parallel: 8 copies at once, as up to 8 parallel tests run together on this Mac, each 5 times under the race detector, while the whole package runs under it beside them. All 40 pass, in 8.1 to 9.5 s each. The whole package beside them: all 94 tests pass, 397 s |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; no alert sent |
