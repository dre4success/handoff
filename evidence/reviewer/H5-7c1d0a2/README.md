# H5 reviewer evidence

Reviewer: GPT-6. Date: 2026-10-09. Target: `7c1d0a2fd6e660079cdfc8beacf2fe705a7c38ad`.

All source inspection and Go commands used a detached worktree of the target commit. The implementer's checkout was not switched or edited. These files are reviewer evidence; they are not implementation changes and are not committed by the reviewer.

## Run

- `checks.txt`: UTC time, reviewer, full commit, commands and output for `go build ./...`, `go vet ./...`, `go test ./... -race -count=1 -v`, dependency inspection and module inspection. All pass. The concurrency test produced 1,000 entries from 50 processes, with sequential IDs and an intact chain. The wait timing test woke in 516 ms.
- `deps.txt`: the complete `go list -deps ./...` output. No `net` or `net/*` package. Module inspection finds no third-party module.
- `probes.txt`: independent CLI probes, including a real SIGKILL at the existing `after-sync` fault point, followed by a same-key retry and repair. One request remains after retry; repair appends without changing the original bytes; the head catches up.
- `probes.py`: the reproducer for `probes.txt`. It creates and removes its own scratch repositories. To repeat, build the target's `cmd/handoff` into a temporary binary, then run `python3 evidence/reviewer/H5-7c1d0a2/probes.py /absolute/path/to/that/binary`.

The race flag covers the Go test binaries. The CLI integration tests' `TestMain` builds its subprocess binary with plain `go build`; their process-level locking assertions still run, but this is not a race-instrumented CLI subprocess.

## Findings

1. **Broken entries are delivered and new damage is silent in an active stream.** Probe case 2 consumes request #1 with `wait --emit --timeout 3s`, then appends a syntactically valid request #99 with `prev: broken` to the scratch record. The wait emits #99, writes cursor 99 and prints no warning. `internal/record/record.go:207` keeps a parsed entry even after its chain check fails; `internal/wait/wait.go:79` checks warnings only once, and line 83 consumes all parsed entries. A parsed entry is not necessarily a verified entry. Delivery and status must not treat the unverified portion as valid work, and new damage must be reported without consuming it. Add a regression that introduces damage after the wait has already loaded a clean record.
2. **A note routed elsewhere clears a review response obligation.** Probe case 4 posts a request, a QUESTION and then an implementer note with `--to operator --re <question-id>`. The request becomes `claimed` and vanishes from the implementer's needs-response list, although the reviewer receives no addressed answer. `internal/status/status.go:83` checks the sender and reply link but not the recipient. Count a note as the reviewer's answer only when it is addressed to that reviewer; a linked escalation can remain an operator note without clearing the original obligation. Cover another configured role as well as the operator.

## Additional observed limitation

Probe case 3 first kills a real writer with the `torn` fault. It then **simulates** a partial repair write by appending the initial newline and an incomplete repair JSON object. Another `repair` refuses because the original torn bytes are now a complete malformed line. This is Run evidence of the simulated byte layout, not a real kill inside `Repair`. The current policy requires the operator for middle breaks. The format gate should explicitly consider whether an interrupted repair is intended to require that intervention; this review does not demand automatic repair of arbitrary middle corruption.

## Evidence preservation

The four implementer `.log` files named by `evidence/step1/README.md` exist in the shared checkout and have headers naming code commit `b023ed5d32b9ae809586ff6522143238dedd4733`. They are absent from the H5 commit because `.gitignore` matches `*.log`. I inspected the main race, dependency and repeat logs as supplied evidence, while independently running the target snapshot. Preserve these artifacts in a reviewed commit or use an explicitly retained evidence location before claiming portable evidence; the README alone does not contain their output. My artifacts use `.txt` so they are visible to Git.

## The ten implementation choices

1. Per-item/from/to superseding is a reasonable way to keep a primary review and a gate request separate; document the departure from the item/from wording.
2. Keeping IDs above the remembered high-water mark after a cut is reasonable and avoids cursor reuse; normal appends remain sequential.
3. Retaining head advisories until repair preserves the observation. Passing the post-sync retry probe supports row 51. New advisories must also remain observable to an active wait.
4. Omitting optional JSON fields instead of writing null is acceptable for this schema.
5. Claims without an addressee are acceptable; their state remains available through inbox/show.
6. Linking an answer to the standing reply is precise. Its recipient must also match, as finding 2 requires.
7. The named fault environment variable provides real process-kill tests; its behavior is explicitly reported, not mistaken for normal operation.
8. Minimal init, repair and decide are justified by the requested checks.
9. Deferred evidence flags and views mean this is a prototype slice, not completed v1. They must remain explicitly outstanding. Check 7 is open; I recommend keeping render in step 3 rather than adding it merely to satisfy the step-1 range in the table.
10. Linear verification and macOS/Linux scope are disclosed limits. No larger record performance or real sleep behavior was established here.

Two-session delivery, long-session Codex behavior, real sleep and the website trial remain open. No merge, push, status-board edit or implementation commit was made.
