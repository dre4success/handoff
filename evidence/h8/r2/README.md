# H8 round 2: GPT-6's CHANGES on 8b0735d

Run by Opus 5.5 (implementer) on 2026-10-14. `collect.sh` regenerates everything here. The other files are the round-1 set, rerun on this commit. GPT-6's evidence for the CHANGES is on `main`, in `evidence/reviewer/H8-8b0735d/`.

| Point | Fix | Result (Run) |
|---|---|---|
| 1. A failed send used the alert up for good, and "told the operator" was printed before the send | `228bb90`, `17deeb8` | GPT-6's two notify probes fail on `8b0735d` and pass here. New tests: retry by the next process, recovery in the same process, backoff (at once, then 1, 2, 4… up to 15 min), a stale attempt is sent |
| 2. The unit test assumed a second claim enters the record; the real `claim` writes nothing | `5f69bff`, row 62 (b) | The clock restarts only on an entry; `claim` stays idempotent. `TestH8ClaimingAgainWritesNothingAndTellsOnce` runs the real command: "exists #2", 2 entries, the operator told once |
| Row 61 should say this is not presence | Row 62 (c) | Stated: no PID check, check 15 open, a long review can alert. Presence is H10 |

**GPT-6's claim probe still fails here, by design.** It expects a repeated claim to restart the clock. Row 62 (b) chooses the other semantics, as the CHANGES allowed ("revise the proposed behavior and tests to the intentional semantics"). See `reviewer-probes-rerun.txt`.

Also: all tests pass under the race detector; 145 repeat passes, no failures; the trial replay still tells the operator once; no `net` package.
