# The reviewer's evidence for H31 round 2 (`797808f`, request #174)

Run by Fable 5.2 (reviewer) on the operator's Mac, 2026-11-15, from a detached worktree at `797808f943440139cd8da86683c3e044b32c7f70`. The reply is CHANGES: the commit no longer merges with main; the rest is right.

- `real-probe.log`: round 1's real cases again with this build. R4 (begun above, resumed from the project), R6 (begun in the project, resumed from above), R2 (`/compact`, `/clear`), X1 (Codex, resumed from the project). `real-probe.py`, `codex-probe.sh`: the drivers; `shim-handoff.txt`: the shim first on the sessions' PATH.
- `probes.log`, from `probes.sh`: M1 the folder of the session's process (lsof), M2 `CLAUDE_PROJECT_DIR` in a hook, M3 the rollout's last `turn_context`, M4 the words.
- `merge-with-main.log`, from `merge-check.sh`: the merge with main in a scratch clone, its two conflicts, and a build and tests once they are settled the plain way.
- `on-this-mac.log`, from `on-this-mac.py`: read-only counts. No path, id or text.
- `go-test-race.log`, `build-vet-deps.log`, from `collect.sh`: the tests, and the implementer's `mutate.py` rerun in a scratch copy.
- `sweep.log`, from `sweep.sh`: the name sweep. Its merge section is empty, since the merge conflicts; `merge-with-main.log` has it. The patterns are kept outside the repository.

Alerts went to a scratch file; none were written. The scratch project's Claude Code data was deleted with `claude purge`, and the two Codex rollouts deleted.
