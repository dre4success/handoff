# The reviewer's evidence for H31 (`c57d29e`, request #169)

Run by Fable 5.2 (reviewer) on the operator's Mac, 2026-11-15, from a detached worktree at `c57d29eed941ff63acb91cce953dfc62d421fc94`. The reply is CHANGES.

## Real sessions

- `real-probe.log`: six throwaway Claude Code sessions (2.1.325, Haiku) and one Codex session (0.169.0). `<P>` is a scratch folder in no git repository; `<P/proj>` is a repository in it, after `handoff init`.
  - R1: started in P, `cd proj`, takes a role, is warned; closed and resumed from P.
  - R2: then `/compact` (the brief warns) and `/clear` (the session is back in P, with no role).
  - R3: then `/background` (the new session starts in P; the role doesn't follow).
  - R4: closed and resumed from P/proj: it gets its brief there, and is still warned.
  - R5: `/compact`, then `/background`; the new session, given a turn with `claude attach`, is warned.
  - R6: started in P/proj, resumed from P: its hooks run in P, and it isn't warned.
  - X1: Codex started in P and resumed from P/proj: its hooks run in P/proj, and it is still warned.
- `real-probe.py`, `attach-probe.py`, `codex-probe.sh`: the drivers. `shim-handoff.txt`: the shim first on the sessions' PATH. The hooks in the user's settings run `handoff` by name, so they ran the shim, which notes each run and runs this build.
- The screen's text in the log has lost some letters and spaces to the terminal's redrawing.
- Alerts went to a scratch file; none were written. The background sessions were stopped and removed, the scratch project's Claude Code data deleted with `claude purge`, and the one Codex rollout deleted.

## Offline

- `probes.log`, from `probes.sh`: N1 spellings of the project's folder, N2 repositories, N3 transcripts, N4 where the warning stands, N5 no session, N6 time. Scratch home and settings folders.
- `on-this-mac.log`, from `on-this-mac.py`: read-only counts over the transcripts on this Mac. No path, id or text.
- `go-test-race.log`, `build-vet-deps.log`, from `collect.sh`: the tests, and the implementer's `mutate.py` rerun in a scratch copy.
- `merge-and-sweep.log`, from `sweep.sh`: the merge with main and the name sweep. The patterns are kept outside the repository.
