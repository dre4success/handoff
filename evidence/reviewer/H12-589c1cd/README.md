# H12 round 2 independent review

Reviewer: Codex (`reviewer`), 2026-10-21 UTC, on the operator's Mac.

Request #61 (supersedes #58), claim #62. Exact commit `589c1cd5af490ba1f9b4d9ea8b296940d9bbb92c`; base `4797d929c89c7cdc4c8335f0197d9fedd71ab5dc`. Reviewed round-2 production changes against `50a8c2d9aa372f47308c40b7d2cb78699102a6fe`, retaining the prior review of the rest.

Detached worktree `/private/tmp/handoff-review-H12-589c1cd`; exact binary `/private/tmp/handoff-review-H12-r2-bin`.

Verdict: CHANGES. Both original reproductions pass, but listener startup has an ownership race. See `reply.md`.

## Independent Run

Logs include command, reviewer, UTC date, cwd, worktree commit and exit status through `run.py`.

- `build.txt`: PASS, exact-commit build.
- `vet.txt`: PASS, go vet.
- `checks.txt`: PASS, format, full branch whitespace, standard-library-only dependencies plus this module and no net package. Reuses the prior review's `checks.py`.
- `moved-wait.txt`: PASS, displaced normal wait exits 2 and replacement receives the note.
- `shared-parent.txt`: PASS, shared Codex hook parent does not transfer roles on startup or clear, including --prefer; two distinct ids retain different assigned roles.
- `startup-race.txt`: REPRODUCED, transfer overlapping hook startup leaves a displaced hook listening without ownership checks; it consumes the replacement's note, and the replacement times out.
- `startup-wait-race.txt`: REPRODUCED, the same startup race in ordinary wait, with the same lost delivery.
- `race.txt`: PASS, full `go test -race -count=1 ./...` on the exact commit, CLI package 429.419s; all other packages pass.

`startup-race.py` uses 300 unrelated bindings to widen the binding-directory read window. Every binding and note was created through the exact built CLI, never by editing the record. Each race reproduced on attempt 0. These are real CLI subprocesses with stand-in session ids, not live model sessions. Scratch notifications went to `/dev/null`; subprocess listeners finished or were stopped by the probe.

`regressions.py` reuses the prior review's CLI fixture helper and supplies the round-2 binary. The prior artifacts are under `../H12-50a8c2d/` and are committed on main as supplied by the implementer.

## Source / Inferred

Reviewed `session.go`, `hook.go`, `cmdWait`, changed tests, README and appended proposal row 74. Inspected implementer evidence, including `round2.log`, the rerun live two-Claude round and the live Claude /clear probe. Those live runs are the implementer's evidence and were not repeated here. No other process-to-session identity fallback was found in bound; the remaining Claude clear path requires its explicit source and one matching process binding.

The new finding is an application-level interprocess ordering bug; a passing Go race detector does not close it. No production source, installed configuration, shared branch, channel file or generated record file was manually changed.

Posted CHANGES as reply #63 on request #61, exact commit `589c1cd5af490ba1f9b4d9ea8b296940d9bbb92c`. See `reply-receipt.txt`. Restarted the foreground reviewer wait after replying.
