# H10b evidence: Codex sessions keep themselves going

Run by Opus 5.5 (implementer) on the operator's Mac. `collect.sh` regenerates every file here except `live-codex.txt` (a real throwaway Codex session, made by `live-codex.sh`), `probe/` (the probe made before the code) and `probe-env/` (the probe for round 3); each probe's README says how. Each file names its date and commit. Notifications go to a file, never to the operator's screen.

## What H10b does

- **`handoff settings codex --as <role>`** prints three files for the project's `.codex/`: `config.toml`, `hooks.json` and `rules/handoff.rules`.
- **Its commands have its role.** `config.toml` sets `HANDOFF_ROLE` for every command the session runs, so `handoff` on its own shows the role's inbox, never the operator's question.
- **The session keeps listening.** Codex has no way to wake an idle session, so `handoff hook listen` runs as its `Stop` hook. It holds the turn open while it waits; when something arrives it exits 2, and Codex continues with it. It's the same command Claude Code uses.
- **It knows its job.** `SessionStart` gives it the brief at its first turn and after a summary.
- **It can reach the operator.** The rule lets the session's own `handoff` commands run outside Codex's sandbox, so its replies notify the operator and show the pop-up. `handoff decide` is forbidden. Hooks run outside the sandbox anyway.
- **Each hook names the role** (`--as`) and starts with `exec`, since Codex runs hooks through a shell, with its own environment.

Design: proposal §17 row 68; round 3's correction, row 71. The operator trusts the hooks once, in Codex's own prompt; the live run, which can't show that prompt, bypasses it with Codex's flag.

## Results (Run)

| What | File | Result |
|---|---|---|
| A real Codex session, set up only by the printed files, idle past Codex's default hook timeout, with nobody typing | `live-codex.txt` | It got the brief and answered "READY — Codex." It was also asked to run `handoff` on its own and give the first line; it didn't, here or in the run before, so the role's reach rests on `probe-env/` and the tests. Its `Stop` hook listened; a second `wait` exited 4. After 660 s idle (Codex's default hook timeout is 600 s) it was still listening. 4 s after claude posted, Codex claimed; 14 s after, having reviewed in a temporary worktree (cloned into /tmp first), it replied APPROVE, and that reply notified the operator (to the file). Then it listened again. Stopped, it left the lock free and no hook running |
| Round 3: the tests for round 2's findings fail on round 2 and pass here | `round3.log` | On `418a0c4`, `settings codex` prints no `config.toml`, and following the brief's question to the operator exits 2 (`--item or --re is required`). Both pass here |
| Whether a project's `config.toml` reaches a Codex session's commands, before the code | `probe-env/` | A command in a throwaway `codex exec`, started with a clean environment, saw `HANDOFF_ROLE=codex` from the project's `config.toml`; Codex logged no ignored setting |
| What Codex does with hooks and rules, before the code | `probe/` | A `Stop` hook exiting 2 continued the turn; `SessionStart`'s stdout reached the model; the hook had no `CODEX_SANDBOX`; a command allowed by a rule ran outside the sandbox |
| The new tests fail on H10a and pass here | `regressions.log` | On H10a, the settings, brief and role tests fail (`settings codex` and the brief's `--as` aren't there); the `shellQuote` test passes there, since H10a quotes paths too. All pass here, with H10a's settings and brief tests |
| What the operator pastes; Codex's own checker on the rule; the brief Codex gets | `demo.txt` | The three files. `show`, `claim` and `reply` allowed, `decide` forbidden, by `codex execpolicy check`; the brief names `--as codex` throughout |
| Every test, under the race detector | `go-test-race-v.log` | All pass |
| The H10b tests, 5 runs each | `repeat.log` | 25 passes, no failures |
| Vet, format, dependencies | `deps-vet-fmt.log` | Clean. No third-party module, no `net` |

## Tests

`cmd/handoff/codex_test.go`:
- `settings codex` prints `hooks.json` that parses, with `exec <handoff> hook brief --as <role>` on `SessionStart` and `exec <handoff> hook listen --as <role>` on `Stop`, timeout a day; the two rules; and `config.toml` setting the role. It refuses without `--as` or for an unknown CLI;
- with the role `config.toml` gives, a bare `handoff` at a terminal, with an approval waiting, shows the role's inbox and never asks the operator's question; with no role it asks;
- where Codex is installed, `codex execpolicy check` reads the printed rules as meant: `show` and `reply` allowed, `decide` forbidden;
- the brief names the role in every command;
- `shellQuote` leaves plain paths alone and quotes the rest.
