# H5 round-2 reviewer evidence

Reviewer: GPT-6. Date: 2026-10-09. Exact target: `089a64fc201afcac6ebf70417f284b01ada41e7f`.

Source inspection, the build and all Go checks used detached worktree `/private/tmp/handoff-review-H5-r2.gxApJw` at that target. The implementer's checkout was not switched. These are uncommitted reviewer artifacts for the implementer to preserve under D10; no implementation or status-board edits were made.

## Run

- `checks.txt`: reviewer, UTC time, full target SHA and output of `go build ./...`, `go vet ./...`, `go test ./... -race -count=1 -v`. All succeed: 52 tests, including 50 processes posting 20 entries each (1,000 entries with sequential IDs and an intact chain), both H5 process regressions, torn-tail recovery, exact-commit/role refusals, superseding, terminal-only decisions, interruption and cursor tests. The race-built CLI wakes 1.529 seconds after the post returns, within check 8's two-second bound.
- `deps-evidence.txt`: `go list -deps ./...`, non-standard packages, `go.mod`, `gofmt -l .`, exact-code comparison with the saved evidence's `bd728051ade739c92724f75efaf21bc4b060b342`, and tracked evidence paths. No `net` or `net/*`, no third-party module, no formatting differences. `cmd/`, `internal/` and `go.mod` are identical to the saved logs' code; the round-1 and round-2 `.log` files are present in the reviewed commit.
- `probes.txt` and `probes.py`: independent built-CLI checks in disposable scratch repositories. The post-sync crash/retry check ran first, before the full test suite. A real SIGKILL after record sync leaves one keyed request; its retry returns `exists #2` without modifying the JSONL bytes; repair appends and catches up the head. New damage during an emitting wait produces exit 2, a warning and no forged delivery or cursor advance. A wait starting on a broken record delivers only the verified prefix, while show marks withheld entries and inbox omits them. Both QUESTION and CHANGES remain owed after linked notes to the operator and reviewer-2; an answer addressed to the reviewer transfers the work. Editing a middle QUESTION into APPROVE leaves the request claimed and creates no operator approval.

Repeat the independent probes by building this target with `go build -o /absolute/temporary/handoff ./cmd/handoff`, then running `python3 evidence/reviewer/H5-089a64f/probes.py /absolute/temporary/handoff`. The final saved run exits 0. An earlier harness attempt omitted the required `--item` on case 5's note; the CLI correctly refused it, the harness was corrected, and the full probe script was rerun.

The requested saved evidence closes checks 1–6, 8, 13, 16, 18, 19 and row 51 for this prototype slice. This review independently reruns their tests. The implementer's saved repeated runs and old-commit failures were inspected, not independently repeated here.

## Source and Inferred assessment

Both numbered findings on `7c1d0a2fd6e660079cdfc8beacf2fe705a7c38ad` are closed. Record parsing separates verified and forensic entries; wait observes changes to either the record or head, reports new warnings, and exits on a true break after delivering the verified prefix. Head disagreements remain advisory. Answer reduction now requires the requester, the standing reply and the addressed reviewer. The race build tag instruments the subprocess CLI during race tests.

The first ten choices retain the assessment from `../H5-7c1d0a2/README.md`, with these resolutions and limits:

| Choice | Assessment |
|---|---|
| 1. Supersede by item/from/to | Accepted for independent primary and gate requests; README now explicitly documents the departure from proposal §6. The drafter should reconcile the proposal wording. |
| 2. Remember the highest ID | Accepted; avoids reusing an ID already passed by a cursor after a cut while the head survives. |
| 3. Retain head advisories | Accepted; the active-wait warning condition is now met, including a head-only change in the unit regression. |
| 4. Omit absent fields | Accepted for this schema. |
| 5. Claims have no addressee | Accepted; inbox/show still expose their state. |
| 6. Link to the standing reply and address its reviewer | Accepted; both wrong-recipient and direct-answer paths pass. |
| 7. Named fault environment variable | Accepted as an explicit process-test mechanism. |
| 8. Minimal init, repair and decide | Accepted to exercise the slice's required checks. |
| 9. Deferred commands and evidence flag | Still outstanding v1 work. Check 7 remains open; keep render in step 3 and reconcile the step-1 check list. |
| 10. Disclosed limits | Accepted for this slice: whole-record verification, macOS/Linux scope, ID reuse if both head and tail are lost, and interrupted-repair recovery requiring the operator. No larger-record performance or real sleep result is claimed. |
| 11. Withhold the predecessor of a broken link | Accepted as a conservative rule: the link alone cannot identify which side changed. The last good entry can therefore be withheld after a bad append. My prefix and edited-verdict probes confirm that withheld entries cannot affect delivery or state. |

A later break can invalidate an entry already delivered by an earlier intact snapshot; the cursor is not rolled back and an agent's memory cannot be undone. The tested guarantee is that detection causes no new delivery or cursor advancement into the withheld region, and current state uses only the verified prefix.

The earlier interrupted-repair probe established a simulated byte layout, not a real kill inside `Repair`. The one-write implementation is Source evidence, not Run proof of every interruption boundary; its intended recovery remains for Fable's format gate.

## Verdict scope

APPROVE the exact target as the step-1 prototype slice. No remaining blocking findings. Fable's format gate still precedes channel migration; the real two-session trial, long-session CLI behavior, real sleep, generated-view check 7, remaining commands and website trial stay open. The temporary reviewer worktree is removed before posting the verdict. No commit, merge or push was made.
