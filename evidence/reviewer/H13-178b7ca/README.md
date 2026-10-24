# H13 review

Reviewer: Codex. Date: 2026-10-24 UTC. Request #68; claim #70.
Exact revision: `178b7ca4e29637bd95e7d9010dbf78d2d26bc723`.
Base: `ae65b6ce30b5c5eb6a03d81f99d8cffdc25581b4`.
Detached worktree: `/private/tmp/handoff-review-H13-178b7ca`.

Verdict: APPROVE. No blocking findings.

Run evidence (each log includes command, date, reviewer, revision, cwd and exit):

- `race.txt`: `go test -race -count=1 ./...`, all packages pass (CLI 460.088 s).
- `build.txt`: requested revision builds to `/private/tmp/handoff-review-H13-bin`.
- `vet.txt`: `go vet ./...` passes.
- `live-awake-unrestricted.txt`: reran the submitted live probe against that binary. Observed real PreventUserIdleSystemSleep assertions for a plain wait and hook listener, descriptors 0–2 at `/dev/null`, release after APPROVE and hook wake, re-hold on a new request, and no caffeinate after SIGKILL of its owner. Fixture record written only through CLI commands.
- `live-awake.txt`: initial sandbox run could not inspect processes (`pgrep` denied). Its exit 0 is NOT evidence that assertions existed or released; the unrestricted rerun supplies that evidence.
- `checks-final.txt`: Go formatting, source/docs whitespace, standard-library/module-only dependencies, no network package pass. `checks.txt` retains the initial whole-diff whitespace failure: three trailing spaces in captured pmset lines and a final blank line in a replay log. These are harmless raw-evidence formatting, not functional findings.

Source review: both listener entry points install `awake.New().Set`; the deferred release covers normal returns, damaged-record errors and lost ownership. caffeinate watches the listener PID and receives no inherited standard streams. Owed covers Open/Claimed/QUESTION/CHANGES and the answer-note transition, excludes superseded, decided and operator-decision states, and ages out without needing a new record entry. The two-hour limit in row 76 is a reasonable amendment for listeners that now run throughout an idle session; the operator still decides that policy.

Sleep aggregation is bounded by two minutes of ordinary ticks, flushes on return (including Emit timeout and ownership loss), and checks late ticks before delivery. An Alive=false tick deliberately exits before examining/delivering anything new; its deferred report flushes an already-observed sleep. Thus a final unseen time jump after the session loses ownership is not reported to that departed session. Existing tests cover grouped wakes, short pauses, deadline jumps, immediate delivery after a jump and Emit streaming.

Limits: no actual Mac sleep/wake, no closed-lid trial, and no live model session/hook invocation. The real hook here was invoked from a shell. Check 9 remains open. No production source edits, shared-checkout switch, merge, push or deployment performed.
