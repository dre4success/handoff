# Step 1 evidence

Run by Opus 5.5 (implementer) on 2026-10-09, in Claude Code 2.1.295 on the operator's Mac (darwin arm64, go1.25.3, git 2.55.0).

**Round 2.** The logs in this folder ran against `bd72805` on `h-step1-record`. Its code is identical to `a623f4b`; the three commits after `a623f4b` change only `.gitignore` and evidence. `collect.sh` regenerates every log. The round-1 logs, run against `b023ed5` and reviewed by GPT-6 in H5, are kept unchanged in `r1-b023ed5/`. A `*.log` rule kept them out of `7c1d0a2`. GPT-6's own evidence for that review is in `../reviewer/H5-7c1d0a2/`.

| File | What it holds |
|---|---|
| `go-test-race-v.log` | Every test, verbose, under the race detector. Under `-race` the CLI started by the process tests is race-built too |
| `repeat.log` | Checks 3, 8, 13, 19, row 51 and both H5 regressions, 5 runs each; check 1, 3 runs |
| `deps-vet-fmt.log` | `go vet`, `gofmt -l`, `go.mod`, every non-standard package, and the absence of `net` |
| `h5-regressions.log` | The tests for GPT-6's two findings: they fail on `7c1d0a2` and pass here |
| `demo.sh`, `demo.log` | A hand run of the slice in a scratch repository, with each command's real output and exit code |
| `collect.sh` | The script that produced all of the above |

## GPT-6's H5 CHANGES on `7c1d0a2`

| Point | Fix | Tests (Run: fail on `7c1d0a2`, pass here) |
|---|---|---|
| 1. Unverified entries were delivered, and new damage during a wait was silent | `a741240` | `TestDamageArrivingDuringAWaitStopsIt`, `TestBrokenRecordDeliversOnlyVerifiedEntries`, `TestNewAdvisoryDuringAWaitIsPrinted`, `TestH5ForgedLineDuringAnEmittingWait` (GPT-6's probe, as separate processes); record: `TestAppendedLineWithAWrongPrevIsNotVerified` |
| 2. A linked note to someone other than the reviewer cleared the response owed | `09a621e` | `TestOnlyANoteToTheReviewerAnswers` (to the operator and to `reviewer-2`), `TestH5LinkedEscalationIsNotAnAnswer` (GPT-6's probe, as separate processes) |
| Evidence logs missing from the commit | `77b9d92`, `32d4ae7` | `git ls-files evidence/step1` lists them |
| The subprocess CLI was not race-built | `a623f4b` | `race_test.go` sets the build flag under `-race` |

**The rule behind point 1.** An entry is verified when the line after it names its hash. The last line stands until something disagrees with it. At a break, the chain cannot tell whether the line before the break or the line at the break was changed. So the line before the break is withheld too, together with every line from the break on.

Withheld entries are kept apart, in `Snapshot.Unverified`:
- `status`, `inbox` and `wait` never see them;
- `show` prints them with `verified: NO`;
- posting stays refused until the operator decides;
- `wait` delivers the verified entries, prints the warning and exits 2, so its cursor never passes a withheld line.

The cost: when a bad line is appended after good ones, the last good entry is withheld as well, until the operator decides.

## Proposal §13 checks named for step 1

All **Run**. Tests in `cmd/handoff` run the built binary as separate processes.

| Check | Test | Result |
|---|---|---|
| 1. Two writers at once; 50 processes × 20 posts | `TestCheck1FiftyProcessesTwentyPostsEach`; in process: `TestConcurrentAppendsStaySequential` | 1,000 entries, ids 1 to 1000, chain intact, no advisory, race-built CLI included; 3 of 3 repeats |
| 2. Same post twice; a reused key with different content | `TestCheck2SamePostTwice` | the second plain post is refused as identical; a keyed retry returns `exists #2`; different content under the key is refused |
| 3. A writer killed leaving no newline | `TestCheck3KilledWriterLeavesATornTail` (a real SIGKILL mid-append, through `HANDOFF_TEST_FAULT=torn`); `TestTornTailRefusesUntilRepair` | the next post is refused; `repair` keeps the torn bytes unchanged as their own line, good lines are untouched, the chain continues; 5 of 5 |
| 4. A cut tail against `head.json`; an edited middle line by `prev` | `TestCutTailIsAnAdvisoryNotARefusal`, `TestEditedMiddleLineBreaksChain`, `TestCheck4EditedMiddleLineStopsPosting`, `TestEditedLastLineShowsAgainstHead` | cut tail: an advisory, and posting continues (row 51). Middle edit: the link at the next line fails, the edited line and everything after it are withheld, posting and `repair` are refused, and reads warn |
| 5. A reply to another commit, from a non-reviewing role or from a non-addressee; a reply to a superseded request | `TestCheck5ReplyRefusals` | all refused with exit 2; the reply to the superseded request is written and changes no state |
| 6. A second revision with `--supersedes` | `TestCheck6SecondRevision`; reducer: `TestSupersededRequestIgnoresLaterReplies` | refused without `--supersedes`; with it, the first shows "superseded by #2", and only the exact-commit APPROVE on the second moves it to "awaiting operator" |
| 7. `CHANNEL.md` hand-edited | none | **Not closed.** `render` is step 3. GPT-6 recommends keeping it there (H5) |
| 8. `wait` wakes within two seconds, by polling | `TestCheck8WaitWakesWithinTwoSeconds` | 516 to 520 ms in five normal runs; 1.55 s with the race-built CLI in `go-test-race-v.log`, inside the bound but closer to it; 0.49 s in the demo |
| 13. An entry posted while no `wait` runs; an entry between two arms | `TestCheck13NextWaitDeliversAtOnce`, `TestDeliversPendingEntriesAtOnce` (zero poll ticks), `TestEntryBetweenTwoArmsIsDelivered` | delivered at once; nothing delivered twice |
| 16. `decide` without a terminal; `HANDOFF_ROLE` and `--as` differ; no role | `TestCheck16Roles`; the positive path: `TestDecideWithATerminal`, through `script(1)` | all three refused; with a pseudo-terminal and the id typed, the decision lands |
| 18. A retry after a lost receipt, same key | `TestCheck18RetryAfterLostReceipt` | the retry of a superseding post returns the original id; one entry |
| 19. An interrupted `wait` | `TestCheck19InterruptedWait` | no child process while waiting; exits on SIGINT; the cursor is unchanged and readable; 5 of 5 |
| Row 51. A crash after the record sync, before `head.json` | `TestRow51KilledAfterSyncIsAnAdvisory` (a real SIGKILL through `HANDOFF_TEST_FAULT=after-sync`); `TestHeadBehindAfterCrashIsAnAdvisory` | posting continues with the advisory on stderr; `repair` clears it with a `repair` entry; 5 of 5 |

## Covered in part, closing nothing

- Check 9's arithmetic, with an injected clock: `TestDeadlineIsWallClockAndGapIsReported` and `TestEntryIsDeliveredBeforeAnExpiredDeadline`. A real sleep needs the operator.
- Check 14: `TestLinkedWorktreeUsesTheSameRecord`. A linked worktree writes to the main worktree's record, and a branch switch does not change it. The "while the reviewer waits" half is step 3.
- Check 20: no `net` package and no third-party module (`deps-vet-fmt.log`). The before-and-after tree comparison is step 4.

## Choices the spec leaves open (**Inferred** unless marked)

GPT-6 assessed the first ten in `../reviewer/H5-7c1d0a2/README.md` and found each acceptable, on conditions: 1 must be documented as a departure, 3 must stay observable to an active wait, and 6 must check the recipient. 3 and 6 are now met. 9 stays outstanding.

1. **The supersede rule is keyed on item, from and to**, not on item and from as §6 says. Otherwise a gate request to `reviewer-2` on the same item would have to supersede the reviewer's.
2. **Ids are never reused.** `head.json` also stores the last id, and the next id is one past the larger of that and the record's. Without this, a cut tail would hand out ids that a role's cursor has already passed.
3. **A `head.json` advisory stays until `repair`.** Each post carries it forward, and `wait` prints a new one when it appears. Posting never stops for it (row 51).
4. **Absent fields are omitted, not `null`.**
5. **Claims have no addressee**, so `wait` does not wake the requester for a claim.
6. **A note answers QUESTION or CHANGES only when `--re` names the standing reply and the note goes to the request's reviewer.** A linked note to anyone else is kept, and the answer stays owed (H5 CHANGES 2).
7. **`HANDOFF_TEST_FAULT`** kills the writer at one of two points. Only the tests set it.
8. **In step 1 although not in its list:** `init` (minimal), `repair` and `decide`.
9. **Not in step 1:** `--evidence` on `post`, `thread`, `board`, `brief`, `render`, presence and notifications. They remain outstanding v1 work.
10. **Limits:**
    - Every post reads and verifies the whole record, so its cost grows with the record's size.
    - It runs on macOS and Linux only.
    - If `head.json` is lost together with a cut tail, ids can repeat.
    - An interrupted `repair` becomes a middle break that needs the operator (GPT-6's simulated layout). `repair` writes the closing newline and its entry in one `write` call, so a killed process cannot split them. A power loss could. Left for the format gate.
11. **Withholding at a break** (H5 CHANGES 1): the line before the break is withheld as well, as described above.
