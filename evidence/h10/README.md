# H10a evidence: Claude Code sessions keep themselves going

Run by Opus 5.5 (implementer) on the operator's Mac. `collect.sh` regenerates every file here except `live-claude.txt` and `live-rules.txt`, which `live-claude.sh` and `live-rules.sh` make by starting real throwaway Claude Code sessions. Each file names its date and commit. Notifications go to a file, never to the operator's screen.

## What H10a does

- **Presence.** A `wait` holds its role's lock while it waits. A second `wait` for that role exits 4 at once, so two never deliver the same entry.
- **The session listens on its own.** When a Claude Code session's turn ends, a hook (`handoff hook listen`) waits in the background. When something arrives for the session's role, it wakes the session and shows it what arrived. The session never runs `wait`.
- **It knows its job.** When the session starts, or after its context is summarised, a hook prints the brief: role, loop, rules, and what waits. `handoff brief` shows the same.
- **No prompts for handoff.** `handoff settings claude --as <role>` prints the settings: the role, the hooks, `allow` for handoff, and `deny` for `handoff decide` and edits to `.handoff/`. It prints; it never edits a settings file.
- **Stuck at a prompt?** the operator is told.
- **Safe anywhere.** A hook does nothing for a session with no role or outside a project. It never wakes a session to report its own trouble (that would loop); it tells the operator, once.

Design: proposal §17 row 67; corrections from the review, rows 69 (round 2) and 70 (round 3). Codex is H10b.

`live-claude.txt` and `live-rules.txt` were run at `5313f1d` (round 2). Round 3 changes only the brief's line for asking the operator, and a test; the rest is regenerated.

## Results (Run)

| What | File | Result |
|---|---|---|
| A real Claude Code session, set up only by the printed settings, with nobody typing | `live-claude.txt` | It got the brief and gave its role. A second `wait` exited 4, naming the hook. 3 s after codex posted, the idle session ran `handoff show 1`; it then claimed, read the commit with `git show` and replied APPROVE, 13 s after the post, with nobody typing. After it ended, the lock was free and no hook process was left |
| Round 2, the rules in real sessions, this commit's and round 1's | `live-rules.txt` | From a linked worktree, with permission checks bypassed, a Write into the shared record: this commit's rules denied it; round 1's let it through. Under a path with spaces, the brief's `show`: this commit's runs, and its allow rule is what lets it (without the rule, "This command requires approval"); round 1's fails in the shell (exit 127) |
| Round 3: the test for round 2's finding fails on round 2 and passes here | `round3.log` | On `d320393`, following the brief's question to the operator exits 2 (`--item or --re is required`). Here every command the brief gives runs |
| Round 2: the tests for the review's findings fail on round 1 and pass here | `round2.log` | On `ebc04f2`, all four fail: the resumed orphan took the entry, the brief's path wasn't quoted, and the rules named `.handoff/` relatively, from the main checkout and from a linked worktree. All four pass here |
| The new tests fail on `main` and pass here; two waits for one role through each binary | `regressions.log` | They don't build on `e918d55` (no presence there); all 13 pass here. Through main's binary both waits deliver the same entry; through this one the second exits 4 at once and the first delivers |
| The pieces at the command line: settings, the brief, a hook woken by a post, hooks off duty | `demo.txt` | As intended. The woken hook exits 2 and shows the entry; off duty, every hook exits 0 and prints nothing |
| Every test, under the race detector | `go-test-race-v.log` | All pass |
| The H10a tests, 5 runs each | `repeat.log` | 65 passes, no failures |
| Vet, format, dependencies | `deps-vet-fmt.log` | Clean. No third-party module, no `net` |

## Found before the code (Run, 2026-10-18)

In a throwaway Claude Code 2.1.302 session with probe hooks: a `Stop` hook with `asyncRewake` that exited 2 while the session sat idle woke it within 2 seconds; `SessionStart` fired on `compact` and its stdout reached the model; and a running async hook ended with its session. The settings schema in 2.1.302 gives `asyncRewake` ("runs in background and wakes the model on exit code 2"), `args` (exec form) and `timeout` with no maximum.

## Tests

`cmd/handoff/hook_test.go`:
- a second `wait` for a role exits 4 at once and names who listens; another role's wait is unaffected; a killed wait frees the lock;
- `hook listen` exits 2 within 3 s of a post, with the entry on stderr; with nothing new by its deadline it exits 2 asking the session to end its turn;
- while a wait listens, `hook listen` exits 0 at once and prints nothing;
- when its session dies, `hook listen` stops and frees the lock;
- with no role, outside a project, or as the operator, every hook exits 0 and prints nothing, and takes no lock;
- the brief: role, the hook, the review duty only for a reviewing role, what waits; `brief` alone says to run `wait`; a wrong role is told to the model;
- `hook notify` tells the operator about a permission prompt, and nothing else;
- a listen in trouble never wakes the session and tells the operator once in three runs;
- `settings claude` prints settings that parse, as intended, and refuses without `--as`, for another CLI, or for the operator;
- a hook stopped while its session dies, then resumed, delivers nothing, and the next `wait` gets the entry;
- installed under a path with spaces, the brief's `show` runs through `/bin/sh`, and the allow rule names the quoted command;
- run from a linked worktree, the rules name the shared record by its full path;
- every command the brief gives (`show`, `claim`, `reply`, a note back, a question to the operator) runs through a shell once its placeholders are filled in.
