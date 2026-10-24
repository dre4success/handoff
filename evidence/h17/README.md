# H17 evidence

`handoff version`, a README half as long, and the reviewer's H16 evidence with no personal paths (§17 row 81).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-24 (round 2), at the H17 commit named in each file's header, with `bash evidence/h17/collect.sh <patterns file>`. The patterns file and the renaming script are H14's, outside the repository.

| File | What it shows |
|---|---|
| `go-test-race.log` | Every test under the race detector, `TestH17VersionNamesTheBuild` among them |
| `version.log` | `handoff version` and `--version` from a build in the linked worktree the evidence was collected in, which Go stamps `(devel)`, and the module path and version Go recorded. A plain clone is in `clone-version.log`; an install from the public tag is checked after publishing |
| `names.log` | H14's sweep: on main, 4 lines with a home path, all in the reviewer's H16 evidence; here, every count is 0. The renaming changes those 4 lines in 2 files, and no hash in the tree is of what it changed |
| `readme.log` | The README's length on main and here, and its sections |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean |
| `clone-version.log` | Round 2 (§17 row 82). In a plain clone, not a linked worktree: `handoff version` and `TestH17VersionNamesTheBuild` with no tag (a pseudo-version), at a `v0.1.0` tag (`v0.1.0`), and one commit after it (`v0.1.1-0.…`). The test passes in all three |
