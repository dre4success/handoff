# H24 evidence

Two test problems, with no change to handoff (§17 row 90): a flaky H8 test, and the race run past go test's default 10 minutes.

Collected by Opus 5.5 (implementer) on the operator's Mac, 2026-10-26, at the H24 commit named in each file's header, with `bash evidence/h24/collect.sh <patterns file>`. The patterns file is H14's, outside the repository. H24 is built on main after v0.1.2 (`bf9cb0b`), so "before" below is v0.1.2.

| File | What it shows |
|---|---|
| `go-test-race-before.log` | Every test under the race detector with no `-timeout`, built from `bf9cb0b`: `cmd/handoff` stops at go test's default 10 minutes ("panic: test timed out after 10m0s"), with 160 tests passed |
| `go-test-race-after.log` | The same from here: all 163 pass, `cmd/handoff` in 407 s, 408 s in all. A run of the same tests just before took 335 s |
| `flake.log` | The cause of the flaky test, made certain by hand: with `quiet_after` 1 s set before the claim, a second claim 1.2 s later, standing in for a claim slow to check, tells the operator, and the wait tells nothing. With the limit cut after the claims, the wait tells, once. Then both changed tests, 20 times each under the race detector: all pass |
| `parallel.log` | Which tests call `t.Parallel()` (53) and which run alone (41), from the source |
| `names.log` | H14's sweep of every tracked file: every count is 0 |
| `deps-vet-fmt.log` | No dependencies; `go vet` and `gofmt` clean; no alert sent |
