# H6 evidence: head.json as a witness (gate findings 1 and 2)

Run by Opus 5.5 (implementer) on 2026-10-09, in Claude Code 2.1.295 on the operator's Mac (darwin arm64, go1.25.3, git 2.55.0). Every log ran against `afc42b4` on `h6-head-witness`, branched from `main` at `1b75b87`. `collect.sh` regenerates them.

The operator decided, in chat on 2026-10-09, to land Fable's gate findings 1 and 2 before this channel moves onto the tool. The decision is recorded as proposal §17 rows 52 and 53 (`afc42b4`).

## The rule

`head.json` is written only after the record is synced. A crash can leave it behind the record, but never at the same count with another hash.

- **Row 52, the edited last line.** When `head.json` counts the same lines as the record but names another last hash, either the last line or `head.json` was edited. The last line is withheld, as at a break: no delivery, no inbox, `show` marks it `verified: NO`. Posting and `repair` are refused until the operator decides. It is reported once, as a break, not again as an advisory.
- **Row 53, a vouched-for line before a break.** When `head.json` counts exactly the lines before a break and names the last of them as it is now, only the lines from the break on are withheld.
- **Unchanged:** without such a `head.json` (missing, unreadable, behind or ahead), the line before a break is still withheld (H5 CHANGES 1). A cut tail and a crash after the sync stay advisories (row 51).

Code: `Snapshot.witness` in `internal/record/record.go`, called from `parse`. `read` in `store.go` now reads `head.json` before parsing.

## Files

| File | What it holds |
|---|---|
| `h6-regressions.log` | The wait and process regressions fail on `1b75b87` and pass here. The record tests are not copied back: they use `Problem.ByHead`, which `1b75b87` lacks |
| `gate-probes-rerun.txt` | Fable's `evidence/gate/H5-089a64f/probes.sh`, unchanged, run by the implementer against this commit. The script's first output line names its author, Fable; the run is mine |
| `go-test-race-v.log` | Every test, verbose, under the race detector, with a race-built CLI |
| `repeat.log` | `TestH6*`, `TestH5*`, checks 8 and 13: 5 runs each |
| `deps-vet-fmt.log` | `go vet`, `gofmt`, `go.mod`, non-standard packages, no `net` |

## Results (Run)

| What | Test or log | Result |
|---|---|---|
| Fable's case C: the last line's body edited in place | `TestEditedLastLineIsABreak`, `TestEditedLastLineIsNotDelivered`, `TestH6EditedLastLineIsWithheld` (separate processes); `gate-probes-rerun.txt` case C | withheld; `wait` exits 2 with nothing delivered and no cursor written; posting and `repair` refused; `show` marks it. On `1b75b87` the edited QUESTION was delivered and the cursor moved |
| Fable's case H: a forged append with `head.json` intact | `TestForgedAppendIsWithheldFromTheBreakWhenHeadVouches`, `TestBrokenRecordDeliversOnlyVerifiedEntries`, `TestH5ForgedLineDuringAnEmittingWait`; `gate-probes-rerun.txt` case H | withheld from the break on; the implementer's "needs my response" keeps #3. On `1b75b87` #3 was withheld too |
| The conservative rule still holds without a vouching `head.json` | `TestForgedAppendWithoutAVouchingHeadWithholdsTheLineBefore` | `head.json` behind: the line before the break is withheld |
| Fable's case E: an interrupted repair | `gate-probes-rerun.txt` case E | still needs the operator (row 54, deferred). With `head.json` naming line 3, only line 4 on is withheld now, and #3 stays listed |
| Everything else | `go-test-race-v.log` | 55 tests pass. Check 1 with a race-built CLI: 1,000 entries. Check 8: 1.53 s under `-race` |
| Repeats | `repeat.log` | 25 of 25 pass |
| Dependencies | `deps-vet-fmt.log` | vet clean, formatted, no third-party module, no `net` |

Apart from cases C, E and H, the rerun of Fable's probes matches its original transcript once hashes, times and paths are set aside. The original has three more sections, G (redo), I (redo) and J, which Fable added by hand after its script ran, so the rerun lacks them.
