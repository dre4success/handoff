# H6 reviewer evidence

Reviewer: GPT-6. Date: 2026-10-10. Exact target: `639ed6fea63aa566ee693da1bc3996c8d70b7da8`.

Reviewed the target in detached worktree `/private/tmp/handoff-review-H6.bDzgGr`. A second detached worktree at base `1b75b877390178520922a52e976e6c30052d13b3` established that the new failure is a regression. Both are removed before posting. The implementer's checkout was not switched or edited. These artifacts are uncommitted for the implementer under D10.

## Verdict: CHANGES

1. **P2 — A cut tail followed by a post-sync crash is misclassified as an edited last line.** `internal/record/record.go:261` treats equal counts and unequal hashes as proof of an edit. But permitted writing after a cut can restore the old line count before the head update. Run case 2 posts #1 and #2, simulates a cut by deleting complete line #2 while retaining its head, then performs a real `after-sync` SIGKILL on a permitted keyed post. The record now contains genuine #1 and #3 with an independently checked intact chain. The old head still has `lines: 2`, `id: 2` and the old #2 hash. Both the same-key retry and `repair` exit 2, classifying genuine #3 as edited. No line body or head was edited. On the pre-H6 base, the identical sequence returns `exists #3` and repair succeeds without changing the original prefix.

Fix this classification while preserving actual last-line edit detection and row 53's witness rule. Account for the existing ID high-water mark or otherwise preserve enough recovery evidence when a permitted append catches up after a cut. Add the combined cut/post-sync-crash regression: same-key retry returns the original receipt, exactly one new keyed request exists, and repair preserves bytes and catches up the head. Also cover the corresponding repair/head-write interruption boundary. Proposal §17 row 52 and the witness/README explanation need an appended correction: a crash is not necessarily behind in **line count** after a cut. This case is separate from the deferred `repair --tail` work: all entries here are complete and the chain verifies.

## Run

- `checks.txt`: reviewer, UTC time, full SHA, `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1 -v`. All pass: 55 tests, race-built subprocess CLI, 1,000 sequential posts from 50 processes, intact chain, and a 1.533-second wake within the two-second bound. The existing suite misses the combined failure above.
- `deps-evidence.txt`: full dependencies and non-standard packages, module and format inspection, tracked H6 evidence, and exact source/proposal comparison to the logs' `afc42b49cbf484f9843a4f35e318705cf212acab`. No `net` or third-party module; no formatting differences. Saved implementer evidence matches the reviewed code and proposal.
- `probes.py`, `probes.txt`: independent CLI probes. The ordinary post-sync SIGKILL/retry check ran first, before the full Go suite, and passes with one keyed request and append-only repair. Case 2 reproduces the blocking regression; the cut is simulated and the SIGKILL is real. Case 3 confirms actual last-line body/verdict editing is withheld, cannot manufacture APPROVE, writes refuse, and no cursor is created. Case 4 confirms a matching head preserves the QUESTION before a forged append, while missing, unreadable, behind, ahead and wrong-hash heads keep the conservative partition; no forgery is delivered.
- `base-probes.py`, `base-probes.txt`: cases 1 and 2 against the pre-H6 base. Both retry and repair succeed in case 2; the new synced entry's receipt is #3 and the repaired record preserves its prefix.

To reproduce, build the indicated snapshot with `go build -o /absolute/temporary/handoff ./cmd/handoff`, then run `python3 evidence/reviewer/H6-639ed6f/probes.py /absolute/temporary/handoff`. Use `base-probes.py` with a binary of the base SHA for comparison. The scripts exit 0 when their observations and assertions complete; a `BUG:` line records the observed product failure, not a passing product expectation.

## Source and Inferred

The witness runs before partitioning, and shared/exclusive locking keeps the head and record reads consistent with ordinary writers. `Store.Append` syncs the record before replacing the head, and checks `Broken` before returning any same-key receipt (`store.go:119`); `Repair` also refuses the inferred break (`store.go:168`). That ordering explains the stranded receipt. The existing `Head.ID` already distinguishes the observed old head #2 from new entry #3; the current witness does not use it.

Rows 52 and 53 record the gate recommendations and the the operator decision attributed in the H6 request. Row 53 behaves as intended. Row 52's underlying count-only premise is contradicted by the Run result. Rows 54–57 match the channel discussion: tail repair deferred, item/from/to superseding and check-7 deferral recorded as superseding the earlier wording, and third-party format documentation remains open for the drafter. The proposal diff only appends those six rows.

I inspected the implementer's saved regression failures, 25 repeated passes and gate-script rerun; I did not repeat those collections. The gate script's growth section yields one line because unkeyed duplicate notes are refused, and its first purported timeout delivers pending work; neither establishes a new performance/timeout result here. The actual suite supplies the timing and cursor results claimed above.

No implementation, status-board or proposal edits; no commit, merge or push. The two-session trial and migration wait for the operator after a corrected H6 review.
