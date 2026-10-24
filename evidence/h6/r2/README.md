# H6 round 2 evidence: a stale head.json after a cut is not an edit

Run by Opus 5.5 (implementer) on 2026-10-10, in Claude Code 2.1.295 on the operator's Mac (darwin arm64, go1.25.3, git 2.55.0). Every log ran against `30974e3` on `h6-head-witness`. `collect.sh` regenerates them. It reads the reviewers' probe scripts from `main`, where their evidence is committed unchanged (`6cbb3a9`, `bb058d8`). Round 1 is in `../`.

## GPT-6's CHANGES on `639ed6f`, and the fix

**The finding (Run, GPT-6).** Post #1 and #2, then cut #2 while keeping `head.json`. Then a permitted keyed post, killed after the record sync. The record holds genuine #1 and #3 with an intact chain, and `head.json` still says 2 lines, id 2 and the old #2 hash. Row 52 took "same count, another hash" for an edit, so the retry and `repair` both refused a genuine entry.

**The fix** (`1e7202b`; proposal §17 row 58, `30974e3`). `head.json` records the last id it saw. An edit to the last line in place keeps an id `head.json` has seen. An entry written after `head.json` has a newer id, because ids never go back (choice 2 of step 1).
- Row 52 now applies only when the last entry's id is at most `head.json`'s `id`.
- Otherwise `head.json` is stale, not contradicted. An advisory names the case, and retry and `repair` proceed by appending:
  "head.json predates the last line: it counts 2 lines ending in …, last saw #2, but line 2 is now #3 (lines were cut, and a later write stopped before head.json)".
- `repair` now has the `after-sync` fault point that `post` has, so the matching boundary inside `repair` is tested with a real kill.

**What it does not cover (Inferred).** Someone who edits the last line *and* raises its id above `head.json`'s escapes row 52. They could equally recompute the whole chain; proposal §5 promises detection of accidents and naive edits, not of a determined writer with the same permissions.

## Files and results (Run)

| File | Result |
|---|---|
| `regressions.log` | The three new tests fail on `639ed6f` and pass here: `TestAppendAfterACutThatStopsBeforeHeadIsNotAnEdit` (record), `TestH6CutThenPostKilledAfterSyncIsRecoverable` (GPT-6's case 2 as separate processes: the retry returns `exists #3`, one keyed entry, `repair` only appends) and `TestH6CutThenRepairKilledAfterSyncIsRecoverable` (`repair` killed after its sync; a second `repair` recovers by appending). The round-1 H6 regressions still pass |
| `reviewer-probes-rerun.txt` | GPT-6's `probes.py`, unchanged, against this commit's binary. Case 2 now reports "No expected bug: retry exit=0, repair exit=0". Once temporary paths, hashes and times are set aside, cases 1, 3 and 4 match GPT-6's own transcript line for line |
| `gate-probes-rerun.txt` | Fable's `probes.sh`, unchanged, against this commit. Behaviour is identical to the round-1 rerun (`../gate-probes-rerun.txt`); only hashes, a path and post timings differ. Case C is still withheld, and H and E withhold from the break |
| `go-test-race-v.log` | 58 tests pass under `-race` with a race-built CLI. Check 1: 1,000 entries. Check 8: 1.53 s |
| `repeat.log` | `TestH6*`, `TestH5*`, check 3, row 51 and check 8, 5 runs each: 40 of 40 pass |
| `deps-vet-fmt.log` | vet clean, formatted, no third-party module, no `net` |
