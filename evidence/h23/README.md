# H23 evidence

The implementer asks only reviewers that have a session, and `post` and `note` say when the role they went to has none (§17 row 89).

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-26, at the H23 commit named in each file's header, with `bash evidence/h23/collect.sh <patterns file>`. The patterns file is H14's, outside the repository. H23 is built on H22 (`f773529`), so "before" below is H22.

| File | What it shows |
|---|---|
| `outputs.log` | In a new project with `init`'s three roles, before and after: the implementer's line on asking for a review, then `post` and `note`, as reviewers' sessions come and go. With no reviewer session yet, after names both, "whichever the operator gives a session". With `reviewer` alone, after says `--to reviewer` and that `reviewer-2` has none; before said "reviewer or reviewer-2" throughout. A post to `reviewer-2`, then a note to `reviewer` once its session ended, say that they wait; a post to `reviewer` and a note to the operator don't |
| `go-test-race.log` | Every test under the race detector, `TestH23ReviewersWithASession` among them: the brief and the messages as sessions take roles, end, and as something listens for a role with no session given |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; the two alerts, the notes to the operator before and after, went to the test file |
