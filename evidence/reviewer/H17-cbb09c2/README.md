# H17: the reviewer's evidence for `cbb09c2`

Collected by Fable 5.1 (reviewer), Claude Code, on the operator's Mac, 2026-10-24, in a detached worktree at `cbb09c235ffb661bf0c73f8726800fda9ed309d3`, with `OUT=<this directory> bash collect.sh <h14-scrub.py> <h14-patterns.txt>`; the script and the patterns file are H14's, outside the repository. The race suite was run separately at the same commit. Alerts and sleep holds went to scratch files; none were sent. Verdict: CHANGES, `handoff` reply to request #87.

| File | What it shows |
|---|---|
| `version.log` | `handoff version` built in this worktree: `(devel)`. Built in a plain clone of this repository at this commit: `v0.0.0-<date>-<hash>` untagged, `v0.1.0` at a `v0.1.0` tag, `v0.1.0+dirty` with a dirty tree, `v0.1.1-0.<date>-<hash>` one commit after the tag. `go install …@v0.1.0` through a `file://` module proxy made from the tag: `handoff v0.1.0`. `version` from `/`. Each output against `TestH17VersionNamesTheBuild`'s regex: the two pseudo-versions do not match, and the test fails in the clone and passes in the worktree (point 1) |
| `readme.log` | The README's length on the base and here; what `handoff help` still says of what the README dropped; that nothing the tool prints says every worktree shares one record; which hooks `settings claude` and `settings codex` print, and that only Claude Code's mention a permission prompt (point 2) |
| `scrub.log` | H14's script on a worktree at the base makes `d9dd08e`'s diff exactly, and the same tree id |
| `sweep.log` | H14's sweep: four lines on the base, none here; the stale-hash sweep finds no hash of the changed bytes |
| `merge-dry-run.log` | `git merge-tree --write-tree` of main (`1c7c779`) and `cbb09c2`: clean, objects to a temporary directory, repository count unchanged |
| `build-vet-deps.log` | `go build`, `go vet`, `gofmt` clean; no `net`; no dependencies; the five commits; the evidence commit adds only `evidence/h17/` |
| `go-test-race.log` | Every test under the race detector, in the worktree: 153 pass, none fail |
