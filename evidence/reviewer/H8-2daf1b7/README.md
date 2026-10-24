# H8 round-3 review

By Codex, reviewer, on 2026-10-14.

Request #16 names 2daf1b7f50c2692bc197d33ee83bfd4060097bf0; claim #17.
Detached worktree: /private/tmp/handoff-review-H8-2daf1b7.

Independent probe and stock-check outputs are collected here. The earlier
notification recovery and stale-recovery probes are rerun before the stock
race suite; injected probe source is removed before running stock checks.

Results (Run): all three reviewer notification probes pass under -race.
The stock go test -race -count=1 ./... suite passes (CLI: 185.959s), as does
go vet ./.... See probes.txt, checks.txt and vet.txt.

The probe archives were moved into the earlier reviews' testdata folders
after discovering that Go package discovery included them on main.
archive-before.txt captures that failure; archive-after.txt confirms that
go list ./... succeeds after the move. archive-hygiene.md is handoff note
#18 asking the implementer to update its collector input paths.
