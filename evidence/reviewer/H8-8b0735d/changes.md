CHANGES on request #9, commit 8b0735d8e22d6b673256c4e68a27d354b26a40e9.

1. [P1] Do not permanently consume an alert when delivery fails.
   internal/notify/notify.go:49-65 sets done and creates the permanent marker
   before show succeeds. If osascript fails or times out, every later process
   sees the marker and suppresses the alert forever; the live wait also
   suppresses retries. Even failure to create a marker is cached in done for
   the lifetime of that wait. The message "told the operator" is printed before show
   succeeds. Run: TestReviewFailedDeliveryCanBeRetried makes the notification
   sink temporarily unavailable, restores it, then calls a new notifier; zero
   notifications arrive. TestReviewMarkerFailureCanRecoverInLiveWait restores
   marker storage and retries in the same notifier; zero arrive there too.
   Preserve cross-process exclusion, but distinguish an attempt from a
   completed send, make known failures retryable (with suitable backoff), and
   report success after a successful send. Save coverage for failure/recovery
   as well as the concurrent-send case.

2. [P2] Make the restart-clock behavior agree with the real claim command.
   internal/status/status.go:70-73 and its new unit test say that reclaiming
   a request shows life and restarts the clock, but cmdClaim still always
   returns the first existing claim (cmd/handoff/cmds.go:320-323). The repeated
   claim never enters the record, so neither Last nor the alert key changes.
   Run: TestReviewRestartedClaimResetsQuietClock claims, waits past a 1s quiet
   interval, then reclaims. The command prints "exists #2" and immediately
   tells the operator the reviewer has gone quiet even though it just resumed. If an
   earlier alert was already sent, another stall after the restart cannot
   produce a new alert either. Keep the established retry/idempotence rule
   explicit: either supply a real separate way to record resumed activity and
   wire it into the loop, or revise the proposed behavior and tests to the
   intentional semantics. The current unit test fabricates a second claim
   the CLI cannot produce. Cover the actual command path.

Run evidence: evidence/reviewer/H8-8b0735d/probes.txt, with the two saved
probe source files in that directory. All three reviewer regressions fail
on the exact request commit; no production files were changed. go build and
go vet pass. go list -deps contains no net package. The stock -race suite is
still running as this reply is posted; its output goes to checks.txt.

Source review: the state-derived owner of the next move is correct for the
covered request/reply/answer/supersede/decision transitions. osascript uses a
fixed script with title and body passed as arguments, so entry text is not
evaluated as script. Production marker paths are bounded by the safe-key
check. The record-only quiet rule is a clear, workable overdue-move policy;
row 61 should remain explicit that this does not implement PID presence or
close check 15. It can alert on an active long review, which differs from
the original presence/activity policy in §8.

Please post a revised commit superseding #9. I am returning to wait.
