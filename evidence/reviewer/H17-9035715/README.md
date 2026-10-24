# H17 round 2: the reviewer's evidence for `9035715`

Collected by Fable 5.1 (reviewer), Claude Code, on the operator's Mac, 2026-10-24, in a detached worktree at `90357151cc9da1b6159f0b60d718df38379e5af8`, with `OUT=<this directory> bash collect.sh <h14-scrub.py> <h14-patterns.txt>`; the script and the patterns file are H14's, outside the repository. The race suite was run separately in a plain clone of the same commit. Alerts and sleep holds went to scratch files; none were sent. Verdict: APPROVE, `handoff` reply to request #90.

| File | What it shows |
|---|---|
| `version.log` | `handoff version` and `TestH17VersionNamesTheBuild` in this worktree (`(devel)`) and in a plain clone in every state: no tag, no tag with a dirty tree, at a `v0.1.0` tag, at the tag with a dirty tree, one commit after the tag. The test passes in all six. `go install …@v0.1.0` through a `file://` module proxy made from the tag prints `handoff v0.1.0`; `version` from `/` |
| `readme.log` | The README's two changed lines and its length; the hooks `settings claude` and `settings codex` print, which the permission-prompt sentence now matches |
| `scrub.log` | The renaming `d9dd08e` still reproduces from H14's script on the base, same tree id; the stale-hash sweep finds nothing at the base or here |
| `sweep-and-merge.log` | `git merge-tree --write-tree` of main (`f2c73e9`) and `9035715`: clean, objects to a temporary directory, repository count unchanged. H14's sweep over this commit (0), over main (4 lines: the H16 evidence lines the branch renames and main does not have yet), and over the merged tree (0, with the reviewer's round-1 evidence in it) |
| `build-vet-deps.log` | `go build`, `go vet`, `gofmt` clean; no `net`; no dependencies; the four round-2 commits on round 1's; the evidence commit adds only `evidence/h17/`; the test's regex as it is now |
| `go-test-race.log` | Every test under the race detector in a plain clone: 153 pass, none fail. The wrapper's own exit line is missing, as its last line explains |
