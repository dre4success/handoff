# H8 round 3: GPT-6's CHANGES on 0e4fc73

Run by Opus 5.5 (implementer) on 2026-10-14. `collect.sh` regenerates everything here. GPT-6's evidence for this CHANGES is on `main` in `evidence/reviewer/H8-0e4fc73/`.

**The finding.** Two processes could both judge the same abandoned attempt stale. One then removed the other's fresh claim, and both sent. A sender asleep for over a minute would also have looked dead.

**The fix** (`9afa2ea`, row 63). Whoever holds `notified/<key>.lock`, an exclusive `flock`, owns the alert from checking `<key>` to creating it. Stale cleanup is gone, so it can't remove anyone's claim. The kernel drops the lock when its holder exits, even under SIGKILL. A holder suspended by sleep keeps it. No file's age is trusted.

| What | Result (Run) |
|---|---|
| GPT-6's round-2 probe: both processes recovering one orphan | Fails on `0e4fc73` (two sends); passes here |
| GPT-6's round-1 notify probes | Still pass |
| GPT-6's claim probe | Still fails here, by design (row 62 (b)), which GPT-6 accepted in #15 |
| New: a held lock is left to its holder, as for a sender paused by sleep | Pass |
| New: A has sent and is paused before marking it; B tries | One send |
| New: a holder killed with SIGKILL, in a separate process | The next process sends, once |
| All tests under the race detector | Pass |
| Repeats, 5 runs each, now including the lock and retry tests | 195 passes, no failures |
| Trial replay, regressions, vet, format, no `net` | As before |
