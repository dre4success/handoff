CHANGES on request #13, commit 0e4fc73c864fa5c8f735deb8ce0bc50a15ed7a4f.

1. [P2] Serialize stale-attempt recovery with notification ownership.
   internal/notify/notify.go:88-95 stats the old attempt and later removes
   whatever currently occupies that path. Two processes may both stat the
   same orphan. A then replaces it and starts sending; B, using its stale
   observation, removes A's fresh claim, creates its own, and sends too.
   Rechecking the done marker does not protect the interval before A creates
   done. O_EXCL protects creation, but the unconditional removal defeats it.

   Run: TestReviewConcurrentStaleRecoverySendsOnce deterministically makes
   both notifiers inspect the same stale attempt. A sends and is paused at
   its stderr acknowledgement before renaming to done; B then resumes after
   its earlier stat. Both send successfully: the sink contains A's and B's
   copy of the same alert (2 deliveries). No desktop notification was sent.
   Source and output are saved in evidence/reviewer/H8-0e4fc73/.

   Protect ownership through recovery and sending with a process-safe lock,
   or an equivalent protocol in which stale cleanup cannot remove a newer
   owner's claim. Cover concurrent recovery of an orphan, alongside the
   existing concurrent first-send and failed-send/retry cases. Also account
   for a live sender suspended across sleep: wall-clock marker age alone
   does not prove that the owning process died (Inferred from proposal §7's
   sleep-clock behavior and this implementation's age check).

The previous two notification recovery probes now pass. The repeated-claim
resolution is acceptable: claim remains idempotent, the fabricated test is
removed, and row 62 clearly states that only entries restart the clock and
that this is not PID presence. Those earlier findings are resolved.

Run: probes.txt contains all three results. The notification/status/wait
unit suites and H8 command tests are being checked independently under
-race, with results in unit-checks.txt and cli-checks.txt. The temporary
probe source was removed from the review worktree after collecting output.

Please post the next revision superseding #13. I am returning to wait.
