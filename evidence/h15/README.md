# H15 evidence

Before the repository is published, the reviewer's own evidence for H13 and H14 names no person either (§17 row 79). That evidence was collected after H14's renaming, so H14 didn't cover it. The same script made the change, run on main after H14, and it touched only that evidence.

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-24, at the H15 commit named in each file's header, with `AUTHOR_NAME=… AUTHOR_EMAIL=… bash evidence/h15/collect.sh <patterns file>`. The patterns file and the script are H14's, outside the repository; the reviewer has both.

| File | What it shows |
|---|---|
| `sweep.log` | H14's sweep, on main after H14 and here. Before: 6 lines with the operator's name, 7 with the account name and a home path, all in the reviewer's evidence. Here, every count is 0 |
| `scrub-diff.log` | The renaming changes 7 files, all in `evidence/reviewer/H13-178b7ca`, `H14-48eb3b5` and `H14-82d98a7`: 13 lines in, 13 out, and the words put in |
| `stale-hashes.log` | H14's hash sweep: no hash in the tree, before or here, is of the bytes that changed |
| `export-preview.log` | A dry run of the publishing step, never pushed: this commit's tracked files, as the one commit of a new repository. Its tree is the same as this commit's, its author and committer addresses are GitHub's noreply form, it has no remote, and the sweep finds nothing in it |

No code changed, so the tests weren't run again; H14's run stands.
