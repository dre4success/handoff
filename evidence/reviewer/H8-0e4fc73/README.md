# H8 round-2 review

By Codex, reviewer, on 2026-10-14.

Request #13: 0e4fc73c864fa5c8f735deb8ce0bc50a15ed7a4f. Claim #14.
Detached review worktree: /private/tmp/handoff-review-H8-0e4fc73.
The shared checkout remains on main.

- changes.md: verdict body; previous findings resolved, one new recovery race.
- probes.txt: old notification probes pass; stale-recovery race fails with
  two deliveries.
- testdata/stale-recovery-probe_test.go: deterministic interleaving of two notifiers
  recovering the same stale orphan; both use the existing test sink.
- unit-checks.txt, cli-checks.txt: stock notification/status/wait and H8 CLI
  tests under -race; both passed with exit 0. The CLI package took 30.174s.
- run.py: independent command collector with date, commit and output.

To reproduce, copy testdata/stale-recovery-probe_test.go to
internal/notify/review_h8_r2_test.go in a disposable checkout of the exact
commit, and the previous review's testdata/notify-probes_test.go to
internal/notify/review_h8_test.go. Run
go test -count=1 -v -run TestReview ./internal/notify.

The stale-recovery probe uses the injected clock and stderr writer to pause
each notifier at a known point, making the file-ownership race reproducible
without sleeps. No live record file is edited and no real notification is
displayed. Injected tests were removed before running the stock checks.
