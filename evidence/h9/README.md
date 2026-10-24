# H9 evidence: the operator decides in one step, at a terminal

Run by Opus 5.5 (implementer) on 2026-10-15, on the operator's Mac. `collect.sh` regenerates every file here; each one names its commit. It sends alerts from its scratch projects to a file, never to the operator's screen.

## What H9 does

- **`handoff` on its own** lists what waits for your decision and what has gone quiet. At a terminal it asks approve, deny or skip for each, then an optional note.
- **`handoff decide H8 --approve`** takes the item's name. It asks y/N. A note is optional (`--note "…"`).
- **No role needed.** With no `HANDOFF_ROLE` and no `--as`, the caller is you. An agent's role is still refused.
- **An agent can't decide.** Both need a terminal to write, and an agent's shell has none. Without one, `handoff` only lists. In an agent's session it shows that agent's inbox.

Design choices: proposal §17 row 64. The Mac pop-up with Approve and Deny buttons (row 59) comes next, as H9b.

## Results (Run)

| What | File | Result |
|---|---|---|
| What you see, at a real terminal: approve one item with a note, skip another, deny it by name, then nothing left | `terminal-demo.txt` | As intended. Piped input with no terminal writes nothing; an agent's session gets its inbox |
| The H9 tests fail on `main` and pass here | `regressions.log` | 9 fail on `3688243`, all 11 pass here. The two that pass on `main` (check 16; outside a project) guard behaviour that didn't change |
| Every test, under the race detector | `go-test-race-v.log` | All pass |
| The H9 tests, 5 runs each | `repeat.log` | 55 passes, no failures |
| Vet, format, dependencies | `deps-vet-fmt.log` | Clean. No third-party module, no `net` |

## Tests

`cmd/handoff/decide_test.go`, each through a pseudo-terminal from `script(1)`, answering prompts as you would:
- decide by item with no role, then y; deny with a note; Enter alone writes nothing;
- an item with two entries waiting is refused with both ids, and either id works, with or without `#`;
- `handoff` on its own: approve with a note, skip, then deny the skipped one on the next run, then "Nothing needs your decision.";
- without a terminal it lists and writes nothing; in an agent's session it's that agent's inbox; outside a project it prints the usage;
- refusals: no terminal; an agent's role, with or without a terminal; `--note` with `--body`; an item with nothing waiting.

`TestDecideWithATerminal` (in `main_test.go`) keeps the old path working: `HANDOFF_ROLE=operator`, an entry id and `--body`, now answered with y.
