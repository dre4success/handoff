# H13 evidence

Small fixes from the step 3 plan (§17 row 76): a listening session keeps the Mac awake while an agent owes a move, the sleep note comes once per sleep, and the README shows `go install` and a current status.

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-21, at the H13 code commit named in each file's header. `bash evidence/h13/collect.sh` writes everything here except the two scripts it runs.

| File | What it shows |
|---|---|
| `go-test-race.log` | Every test under the race detector: 150 pass, none fail |
| `h13-tests.log` | The H13 tests in full: `Owed`, the sleep note, keep-awake in `wait`, the `awake` package against the real `caffeinate`, and both listeners holding and releasing |
| `regressions.log` | The sleep note tests on `main`: all four fail there (a line per late tick, a 50 s pause noted, a sleep ending in a delivery not noted), and pass here |
| `trial-replay.log` | The H7 trial's 27 logged sleeps replayed through one wait: 27 lines on `main`, 13 here. The five 18 to 19 s gaps of 07:16 to 07:19 fold into one |
| `trial-replay_test.go.txt` | The replay, copied into `internal/wait` by `collect.sh`; not part of the build |
| `live-awake.sh`, `live-awake.txt` | A real `wait` and a real `hook listen`, the real `caffeinate`: no hold with nothing owed; a hold, seen in `pmset -g assertions`, once a request is posted; released when the reviewer approves, when the hook is woken, and when the wait is killed with SIGKILL. The hold's stdin, stdout and stderr are `/dev/null` |
| `deps-vet-fmt.log` | No dependencies; `go vet` clean; `gofmt` clean |

Not tried: a real sleep and wake (check 9 stays open, for the operator to try); a hold from inside a running Claude Code or Codex session, rather than from a shell.
