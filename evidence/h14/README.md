# H14 evidence

Before the repository is published, it names no person (§17 row 77). The operator's name became "the operator" everywhere, including what the tool prints. Paths under the operator's home became `~`, and the other project became "the website project".

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-24 (round 2), at the H14 code commit named in each file's header, with `bash evidence/h14/collect.sh <patterns file>`. The patterns file, and the script that did the renaming, stay outside the repository, since both name what was removed. The reviewer has both.

| File | What it shows |
|---|---|
| `sweep.log` | Every tracked file's contents and name, searched for the operator's name (as a word, and inside identifiers), the account name outside the module path, home paths, the other project's name, and email domains. Each count is 0 |
| `scrub-diff.log` | The renaming commit changes words only: as many lines in as out. It lists the renamed files git pairs up (two of the four; it shows the other two as a delete and an add) and every word or phrase put in, with counts. It doesn't repeat what was taken out |
| `brief.txt` | What an agent now reads at the start of a session: "The operator decides." |
| `go-test-race.log` | Every test under the race detector, with the renamed tests and messages |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean |
| `trial-record.log` | Round 2 (§17 row 78). The H7 trial's saved record: before the renaming, every link of its chain matches. In the redacted copy, lines 1 and 7 differ, and the links at lines 2 and 8 no longer match. `head.json` and `config.json` are unchanged. The tool, given the copy, reports the chain broken at line 2 and withholds every entry |
| `stale-hashes.log`, `stale-hashes.py` | Round 2. Every hash quoted in the tree, before the renaming and here, checked against the hashes of what it changed: each changed file, and each changed line. The only ones are the trial record's two links |

The evidence of earlier items was edited for names and paths only. Its commands and outputs are otherwise unchanged. So are its results, with one exception: the H7 trial's record no longer verifies, so it is labelled a redacted transcript, in `evidence/trial/README.md` and in a first line added to `evidence/trial/checks.txt` (see `trial-record.log`).
