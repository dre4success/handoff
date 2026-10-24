# H6 re-review evidence

Reviewer: GPT-6. Date: 2026-10-10. Exact target: `c5826e5ec05936906b36dff2c88a037d04beab7c`.

Reviewed and tested in detached worktree `/private/tmp/handoff-review-H6-r2.nEsbmm`, removed before posting. The shared checkout stayed on main. Only the channel and these reviewer artifacts were written in that checkout; no implementation, proposal or status-board edits, commit, merge or push. Evidence is uncommitted for the implementer under D10.

## Verdict

APPROVE the exact target. The single blocking finding from H6 at `639ed6fea63aa566ee693da1bc3996c8d70b7da8` is closed. No remaining blocking findings in this revision.

## Run

- `checks.txt`: reviewer, UTC time, target SHA, Go version, and output for `go build ./...`, `go vet ./...`, `go test ./... -race -count=1 -v`. All succeed: 58 tests with a race-built subprocess CLI, 1,000 sequential entries from 50 concurrent posting processes, an intact chain, the three new recovery tests and retained H6/H5 protections. Wait wakes in 1.535 seconds, within the two-second bound.
- `deps-evidence.txt`: complete `go list -deps ./...`, non-standard packages, module and formatting inspection, tracked evidence paths, and a source/proposal comparison against the implementer's evidence commit `30974e354702a4d7bd9bf4e3301770282a75080c`. No `net` or third-party module, no format differences, and exact code/proposal identity with the supplied logs.
- `probes.py`, `probes.txt`: independent built-CLI probes, creating and removing scratch repositories. The ordinary post-sync SIGKILL/retry check ran first, before the full Go suite. All six cases pass:
  1. Ordinary real post-sync SIGKILL: same-key retry returns the original entry without changing bytes; repair appends and catches up the head.
  2. Simulated tail cut, then real post-sync SIGKILL: retry returns `exists #3`, does not write another entry, and reports the stale-head advisory. Exactly one keyed request exists; repair preserves the prefix and catches up the head.
  3. Actual last-line body/verdict editing: withheld, no delivery or cursor, no manufactured operator approval, and posting/repair refuse without changing bytes.
  4. Forged append: the matching head preserves the genuine QUESTION; missing, unreadable, behind, ahead and wrong-hash heads keep the conservative partition. No forgery is delivered.
  5. Real post-sync SIGKILL inside repair after a cut: the repair entry is readable and advisory, and a second repair appends, catches up the head and clears the advisory.
  6. Real post-sync SIGKILL inside torn-tail repair: original damaged bytes and the repair proof remain; recovery only appends and updates the head. The damage hash and preceding-good-line hash are independently checked.

Repeat the probes by building this exact snapshot with `go build -o /absolute/temporary/handoff ./cmd/handoff`, then running `python3 evidence/reviewer/H6-c5826e5/probes.py /absolute/temporary/handoff`. The saved final run exits 0 and ends with all independent process probes passed.

## Source and Inferred

`Snapshot.witness` now applies the same-count/different-hash break only when the last parsed ID is at most the ID remembered by the head. Because ordinary writers choose `max(record ID, head ID)+1`, a permitted write after a cut receives a newer ID. The combined crash case therefore stays advisory, allowing the keyed receipt and append-only repair. The matching-head predecessor rule is unchanged. Both repair write paths now place the test kill after record sync and before the head replacement.

Proposal §17 row 58 correctly appends the correction to row 52; no earlier text changed. It preserves the intent to withhold actual last-line edits while recording the count collision after a cut. No disk-format change or additional recovery mechanism was introduced.

The implementer explicitly records the limit that changing both a tail entry's body and its ID above the remembered head ID can take the advisory path. That deliberate rewrite is within the same-user tampering limit of proposal §5; this format does not authenticate writers. The fix does not claim that ID comparison proves provenance.

I inspected the saved old-commit failures, 40 repeated passes, and both reviewers' probe reruns, rather than repeating those collections. The repair regression fails on the old commit because its repair path lacks the fault hook; the new real-kill run establishes the intended boundary. The unchanged historical reviewer script's final sentence still says case 2 reproduces a regression, but its actual case-2 output reports retry and repair exit 0. My current script asserts success explicitly. The gate script's growth and purported first timeout remain exploratory outputs, not new performance or timeout evidence.

## Scope

H6 is approved for the operator's merge and the planned repository trial. The trial itself (checks 10 and 11) has not run and requires the operator present. Deferred `repair --tail`, generated views/check 7, remaining v1 commands and documentation, real sleep and the website trial remain outstanding.
