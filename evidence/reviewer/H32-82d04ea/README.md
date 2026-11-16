# The reviewer's evidence for H32 (`82d04ea`, request #172)

Run by Fable 5.2 (reviewer) on the operator's Mac, 2026-11-15, from a detached worktree at `82d04ea0aaff11f3b156ec621457dda4baa1b32b`. The reply is APPROVE.

## Real sessions

- `real-probe.log`: throwaway sessions of the Antigravity CLI 1.5.9 (`gemini-3.8-flash-low`) in a scratch project, with the hooks this build's `handoff settings antigravity` prints.
  - A1: the session takes a role. Its agent runs `fyi`, `away`, `back`, `decide` and `handoff` on its own with this build (all refused or the inbox), then `fyi` with the base's build (written as the operator). A request wakes it; Esc ends the listener; a second request wakes it in the same process.
  - A2: print mode, with and without `--sandbox`: a command has a terminal and the variable, and `fyi` is refused.
  - A3: ten notes wake the session ten times; at the limit nothing listens and the operator is told once; a typed message starts it again.
  - A4: the conversation reopened with nothing typed (nothing listens); then print mode on it (it doesn't end).
- `agy-probe.py`, `agy-limit.py`, `agy-resume.py`, `envcheck.py`: the drivers. `shim-handoff.txt`: the shim in the build's place, which notes each run and runs the build.
- Alerts went to a scratch file. The five conversations, their data and logs are deleted, and the scratch folder is out of Antigravity's trusted workspaces. Gemini CLI was tried once and isn't signed in; what that left is removed too.

## Offline

- `probes.log`, from `probes.sh` and `withtty.py`: Q1 the operator's commands with a terminal on stdin under each CLI's variables, on the base's build and this one; Q2 Antigravity's hook inputs; Q3 settings and the brief.
- `as-operator.log`, from `as-operator.sh`: the other ways to name the operator, under each CLI's variable.
- `go-test-race.log`, `build-vet-deps.log`, from `collect.sh`: the tests, and the implementer's `mutate.py` rerun in a scratch copy.
- `merge-and-sweep.log`, from `sweep.sh`: the merge with main and the name sweep. The patterns are kept outside the repository.
