# H27 evidence: `tell` is now `fyi` (§17 row 94)

Collected by `collect.sh` at `791d933`, on the operator's Mac, 2026-11-10/11, by Opus 5.5 (implementer). Each log starts with its command, date, who ran it and the commit.

| File | What it shows |
|---|---|
| `fyi.log` | `handoff fyi` at a terminal, with the implementer's `wait` and the reviewer's `hook listen` running and reviewer-2 with no session: both listeners wake with it, the entries' kind is `fyi`, nothing waits for anyone after, no alert is sent, and the brief's line begins "An fyi from the operator" |
| `refusals.log` | Each refusal now names `fyi` and writes nothing; `handoff tell` is an unknown command; the usage names `fyi` and not `tell` |
| `leftovers.log` | `git grep` for `tell` as the command or the kind finds only the H27 test; `TestH27TellIsNowFyi` fails on `3119db5`, where `tell` is a command, and passes here |
| `go-test-race.log` | Every test under the race detector: 171 pass, 0 fail. Then the H26 and H27 tests 10 times each under the race detector |
| `deps-vet-fmt.log` | No home path or email address added; no dependencies, no `net`; vet and gofmt clean; no alert sent during collection |
