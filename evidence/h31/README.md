# H31: a session started outside the project is warned

Run by Opus 5.5 (implementer) on the operator's Mac, 2026-11-15, Claude Code 2.1.325, on branch `h31-outside`. §17 rows 101, 103 and 104 say what changed.

## Round 3

- The branch has main, with H32, merged in (`be95d21`). The conflicts were the tests' list of session variables and §17's rows; both kept, rows in number order.
- `mutate.py`, `mutations.log`: two more conditions, twenty-four in all: an Antigravity session isn't looked for among Codex's rollouts, and a `turn_context` written with spaces after its colons is found. Each of the twenty-four fails a test.
- `go-test-race.log`: build, vet, gofmt and every test with `-race`, on the merged branch.

## Round 2

- `background.log`, from `bg-probe.py` and `dump2.sh`: a session begun in `P` runs `cd proj`, then is sent to the background. The new session's SessionStart and Stop ran in `P`, with `CLAUDE_PROJECT_DIR` `P`. The background worker's own process, its `CLAUDE_PID`, is in `P` too, as the first process was after the `cd`.
- `resume.log`, from `resume-probe.py`: the reviewer's R4 and R6 on real sessions, with this round's build. Each session ran `handoff role` itself, from the project. Begun in `P`: warned. Resumed from the project: not warned. Begun in the project: not warned. Resumed from `P`: warned.
- Round 2's mutations: twenty-two conditions, each failing a test (round 3's log has them all). Writing them found two pieces of code that changed nothing, and they were taken out: a check that the rollout line began within what was read, and an early stop after reading a small rollout whole. They also found a test case that could never fail, a line "mentioning" `turn_context` inside a string, where the quotes are escaped. It is now a line holding a `turn_context` object deeper, with a folder.

## Round 1

The files below are as they were, except `mutate.py` and the two logs above, which this round replaced.

- `outside-probe.py`, `dump.sh`, `settings.local.json.txt`: a throwaway interactive session (Haiku) through a pseudo-terminal, started in `<scratch>/P`, which is no git repository. `<scratch>/P/proj` is one, after `handoff init`. `P/.claude/settings.local.json` ran `dump.sh` at SessionStart, UserPromptSubmit, Stop and SessionEnd. The session replied once, ran `cd proj`, ran `pwd`, and was closed; then it was resumed from `P` and ran `pwd`. Command: `python3 -I outside-probe.py <scratch>/P <scratch>/hooks.log`.
- `probe-hooks.log`: where each hook ran. A hook runs in the folder the session is in: the stdin's `cwd` and the hook's own folder agree. Before the `cd`, that was `P`. After it, `P/proj`, so a Stop then reached the project. The resume put the session back in `P`: its SessionStart, and every hook after, ran there. `CLAUDE_PROJECT_DIR` was `P` throughout, and Claude Code's own process stayed in `P`. Paths are written `<scratch>`.
- `replay.log`: the probe's session takes a role from inside the project, with the installed build and with this branch's. The installed one says nothing; this branch warns, naming `P`. With the transcript's first folder made the project, this branch says nothing.
- `on-this-mac.txt`: this branch's `handoff role`, with no role named, which only reads, as each of the three sessions with a role in another project. The implementer started in the folder above that project and is warned. The two reviewers, a Claude Code and a Codex session, started in it and are not. No binding changed. The project and the folder above it are written in general words.
- Round 1's mutations: eighteen conditions, each failing a test. A first run found a case that held nothing: its session id had a comma, from the test's name, so no transcript was looked for.

After these files were made, each round's probe sessions were removed, and their data deleted with `claude purge`.
