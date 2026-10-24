# H12 round 3 independent review

Reviewer: Codex (`reviewer`), 2026-10-21 UTC, on the operator's Mac.

Request #64 (supersedes #61), claim #65. Exact commit `40a37680b9d51997c762bc602928b7afec2c977a`; base `4797d929c89c7cdc4c8335f0197d9fedd71ab5dc`. Reviewed the round-3 changes against `589c1cd5af490ba1f9b4d9ea8b296940d9bbb92c`, retaining the prior review of the rest of H12.

Detached worktree `/private/tmp/handoff-review-H12-40a3768`; exact binary `/private/tmp/handoff-review-H12-r3-bin`.

All prior findings are addressed. Role resolution now carries whether it used a session binding, and that fact always installs a liveness predicate for the listener. A disappeared binding cannot turn that predicate into nil. A hook update that discovers a lost or changed role no longer returns its earlier role. Codex session identity remains separate from its shared host, and the Claude continuity exception requires clear and one matching process binding.

## Independent Run

`run.py` records command, reviewer, UTC date, cwd, worktree commit and exit status in each log.

- `build.txt`: PASS, built the exact commit.
- `vet.txt`: PASS, go vet.
- `checks.txt`: PASS, format, full branch whitespace, standard-library-only dependencies plus this module, no net package. Reuses `../H12-50a8c2d/checks.py`.
- `startup-hook.txt`, `startup-wait.txt`: PASS, prior startup exploit does not reproduce in 30 attempts for either listener. Same prior probe with only the built binary path changed, wrapped by `check-startup.py` to distinguish its expected no-exploit exit 1 from a probe failure. Every old hook exited 0; every old wait exited 2.
- `controls.txt`: PASS, six delivery controls using the test startup delay. Bound hook, plain wait, --as own role and --for own role terminate without consuming the posted note; replacement receives it. --for another role and an explicit-role wait outside sessions still receive their notes.
- `shared-parent.txt`: PASS, distinct Codex thread ids sharing a hook parent keep their separate assigned roles; another thread's startup or clear, including --prefer, does not steal one.
- `moved-wait.txt`: PASS, role moved after registration stops old wait with exit 2 and the replacement receives the note.
- `race.txt`: PASS, full `go test -race -count=1 ./...` on the exact commit; CLI package 444.499s and all other packages pass.

The startup probe seeds 300 unrelated bindings through the CLI to widen the earlier race window. All scratch records were initialized and mutated through the built CLI, never by hand. Notifications went to `/dev/null`. No reviewer-launched model sessions were used. Probe subprocesses finished or were stopped by their cleanup.

## Source / Inferred and limits

Reviewed changed role resolution, inbox role selection, hook startup, binding updates, new regression test and appended proposal row 75. Inspected supplied updated race, prior-version failures, live two-Claude round and /clear evidence. Those live runs are the implementer's Run, not independently repeated by this reviewer. Live shared-server/two-Codex behavior remains supported here by stand-in thread and host probes, prior source evidence and the reviewed identity logic.

No production source, installed settings, shared branch, channel file or generated record file was manually changed. Worktree source is clean.

Posted APPROVE as reply #66 on request #64, exact commit `40a37680b9d51997c762bc602928b7afec2c977a`. See `reply-receipt.txt`. Foreground reviewer wait restarted after replying. No merge or deployment performed by the reviewer.
