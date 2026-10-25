# H19 evidence

A new project's loop starts by itself: the implementer commits on a branch without asking; a reviewer asked to look at work with no request sends its findings to the implementer as a note; a project's first commit can be asked about with no base (§17 rows 85 and 86).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-25 (round 2), at the H19 commit named in each file's header, with `bash evidence/h19/collect.sh <patterns file>`. The patterns file is H14's, outside the repository. H19 is built on H18 round 2 (`e832f00`), so "before" below is H18.

| File | What it shows |
|---|---|
| `go-test-race.log` | Every test under the race detector, `TestH19BriefSaysWhoCommitsAndWhereFindingsGo` and `TestH19FirstCommitNeedsNoBase` among them. `TestH10BriefCommandsRun` and `TestH11BriefCommandsRunForClaudeAndCodex` run every command the brief gives, the reviewer's new note included |
| `briefs.log` | The implementer's and the reviewer's brief, before and after, in a project made by `init` with no flags |
| `new-project.log` | A new project as the operator met it: no commit; `init`; the implementer commits on a branch and asks for a review. Before: refused, since the main branch has no commit. After: posted with no base, and `show` gives no base. A second commit with no `--base` is still refused, naming both causes; with `--base` it posts |
| `round-2.log` | Round 2 (§17 row 86): the reviewer asks for changes on that first request, and the fix goes on top with `--supersedes` and no `--base`. From round 1 (`63a9d7b`): refused. From here: posted with no base, and `show` gives no base |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; no alert sent |
