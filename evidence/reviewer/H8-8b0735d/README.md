# H8 review of 8b0735d

By Codex, reviewer, on 2026-10-14.

Request #9 names commit 8b0735d8e22d6b673256c4e68a27d354b26a40e9,
based on 56da56631c434a3c985c1e3650c6886dd7a94582. Claim #10 was posted
through handoff. The shared checkout remained on main; review and commands
ran in the detached worktree /private/tmp/handoff-review-H8-8b0735d.

Files:

- changes.md: exact verdict body.
- build.txt, vet.txt, deps.txt: independent build, vet and dependency output.
- checks.txt: stock go test -race -count=1 ./... output and exit status.
- probes.txt: three reviewer regressions, all fail on this commit.
- testdata/notify-probes_test.go, testdata/claim-probe_test.go: source for
  those regressions, stored under testdata so Go does not discover them as
  packages when checking the repository.
- run.py: command-output collector; each file has date, reviewer, commit,
  working directory, command and exit status.

To reproduce the probes, copy testdata/notify-probes_test.go into
internal/notify/review_h8_test.go and testdata/claim-probe_test.go into
cmd/handoff/review_h8_test.go in a disposable checkout of the reviewed
commit. Run go test -count=1 -v -run TestReview ./internal/notify ./cmd/handoff.
The notification probes use the existing sink and temporary directories,
so they do not display desktop notifications. The injected tests were
removed from the temporary worktree after collecting their results.

The stock race command started before the probe files were added. The
probe command ran independently and its output is kept separately. Verify
the completed stock command's exit status before treating it as passed.
