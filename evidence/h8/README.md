# H8 evidence: the operator hears when work goes quiet or needs a decision

Run by Opus 5.5 (implementer) on 2026-10-14, on the operator's Mac. `collect.sh` regenerates every file here; each one names its commit.

## What H8 does

- **A move owed too long reaches the operator.** If a request isn't picked up, a review goes quiet, or a QUESTION or CHANGES isn't answered within `quiet_after` (30 minutes by default), the operator gets a Mac notification. The time counts from the request's last entry, so a claim or an answer restarts it.
- **A decision reaches the operator at once.** An APPROVE, or a note to the operator, notifies the operator from the command that wrote it, so no `wait` needs to be running.
- **Each alert is sent once.** That holds across processes and after a sleep.
- **Missed ones stay visible.** `inbox` lists anything quiet under "gone quiet".

Design choices: proposal §17 row 61.

## Results (Run)

| What | File | Result |
|---|---|---|
| The trial's stall, replayed: entries #1 to #6 of `evidence/trial/record/`, ending with Codex's claim of #5 | `trial-replay.txt` | `inbox` lists "H7: reviewer claimed #5 and has been quiet for 47 h". A `wait` tells the operator once; a second `wait` stays quiet |
| The H8 command-level tests fail on `main` and pass here | `regressions.log` | 5 fail on `56da566`, 5 pass. The unit tests use the new API, so they don't build on `main` |
| Every test, under the race detector | `go-test-race-v.log` | All pass. Check 8 woke in 1.54 s with the race-built CLI |
| The H8 and alert tests, 5 runs each | `repeat.log` | 140 passes, no failures |
| Vet, format, dependencies | `deps-vet-fmt.log` | Clean. No third-party module, no `net` |
| A real notification on the operator's screen | (by hand, 21:22 UTC) | The `osascript` call ran without error. Whether it showed is the operator's to confirm |

**Inferred, from the rule and `evidence/trial/implementer-wait.log`:** in the trial, Codex claimed #5 at 21:38:56 on 10-12. The operator would have been told on the first `wait` tick after 22:08:56. The Mac was asleep then, so that's 22:18:52, when it woke. Instead, the operator found out two days later.

## Tests

- `internal/status/alert_test.go`: who owes the move, at each step; hours for long silences; only the addressee's claim restarts the clock; an APPROVE alerts at once until decided; a note to the operator; superseded requests; a missing timestamp.
- `internal/notify/notify_test.go`: once per key, across processes (20 racing); a new key is a new alert; markers stay in the record directory; unsafe keys are refused.
- `internal/wait/wait_test.go`: a silence that comes due during a sleep is passed on at the first tick after waking; a wait that returns at once still passes alerts on.
- `cmd/handoff/alert_test.go`: as separate processes: `init` writes `quiet_after`; bad values are refused; the trial's stall, told once; APPROVE told at once; `inbox` lists it.

Tests never pop notifications: `HANDOFF_TEST_NOTIFY` sends them to a file, `/dev/null` by default.
