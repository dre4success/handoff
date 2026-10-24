# H12 independent review

Reviewer: Codex (`reviewer`), 2026-10-21 UTC, on the operator's Mac.

Request #58, claim #59. Exact commit: `50a8c2d9aa372f47308c40b7d2cb78699102a6fe`; base: `4797d929c89c7cdc4c8335f0197d9fedd71ab5dc`.
Detached worktree: `/private/tmp/handoff-review-H12-50a8c2d`. Built binary: `/private/tmp/handoff-review-H12-bin`.

Verdict: CHANGES. See `reply.md` for the two findings.

## Independent Run

Every log made with `run.py` includes the command, UTC date, reviewer, cwd, exact worktree HEAD and exit status.

- `build.txt`: PASS, built the exact commit.
- `vet.txt`: PASS, `go vet ./...`.
- `checks.txt`: PASS, gofmt, branch diff whitespace, standard library plus this module only, no net package.
- `moved-wait.txt`: REPRODUCED, a displaced ordinary wait holds the lock and consumes the replacement's note; the replacement then times out. `probes.py wait`.
- `moved-hook.txt`: PASS, displaced hook exits and replacement receives the note. `probes.py hook`.
- `shared-codex-parent.txt`: REPRODUCED, two thread ids with a common hook parent cause a silent role transfer. `probes.py shared-codex-parent` uses a Python process as the shared parent, no live Codex/model run.
- `race.txt`: PASS, full `go test -race -count=1 ./...` on the exact commit, CLI package 420.342s, all other packages pass.
- `codex-version.txt`: installed Codex 0.162.1 (read-only).

Scratch records were initialized and mutated through the exact built CLI, never by editing `.handoff/`. Scratch notification output went to `/dev/null`. The stand-in parent processes were stopped after each probe. Source tree unchanged by this reviewer.

## Source / Inferred

- New and modified production code, tests, README and proposal row 73 reviewed against the stated base.
- Inspected supplied `evidence/h12/` logs, including live two-Claude results. These are the implementer's Run, not independently repeated live runs.
- Read-only local `codex --help`, `codex --version`, `codex app-server --help`, `codex execpolicy check --help`: version 0.162.1; CLI exposes a shared background server and `--no-daemon`.
- [OpenAI maintainer's explanation of the app-server/hook runtime](https://github.com/openai/codex/issues/48500): app server hosts active threads and hooks; several clients can connect. This supplies the runtime premise for the shared-parent reproduction. The reviewer did not launch two live Codex sessions.
- [Official hooks documentation](https://learn.chatgpt.com/docs/hooks): SessionStart payload identifies its session and start source. Current code ignores the source when deciding a same-PID id change means continuity.

Posted CHANGES as reply #60, exact commit `50a8c2d9aa372f47308c40b7d2cb78699102a6fe`; `reply-receipt.txt` records the CLI command and successful receipt. Foreground reviewer wait restarted after the reply.
