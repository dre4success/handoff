# H9b review

By Codex, reviewer, 2026-10-17. Request #29 names
6783e4a8badcf847086db14ed5e9fd7238d983e3, based on
9b2dfec4d9a511bf2bbaef0dc1afe73db9082fd9. Claim #30.

Review uses the detached worktree /private/tmp/handoff-review-H9b-6783e4a.
The shared checkout stays on main. No live operator decision was made.
CHANGES was posted as reply #31; the reviewer resumed the foreground wait.

## Finding: inherited lock can be finalized while the dialog is open

cmd/handoff/popup.go:80 wraps descriptor 3 in an anonymous os.File that is
unreachable after holdsLock returns. Its finalizer can close the descriptor
before popup returns, dropping the alert's flock. Keep the os.File alive for
the entire popup, for example with a named local and deferred Close.

lock-gc.py creates a disposable Git project and record through the CLI,
posts a legal 891 KB multiline note to the operator, and uses the shipped
HANDOFF_TEST_POPUP sink, which never clicks. Its displayed preview is short
because popupText truncates to ten lines. While that dialog remains
unanswered, an independent flock succeeds; another CLI write starts a
second dialog for the same note. No marker or record is edited by the probe.
It verifies that no decision was written.

lock-gc-multiline.txt is the final reproduction on the unmodified request
commit, with GOGC=100 (Go's normal default). The expected failing assertion
reports both the free lock and the second dialog. lock-gc-off.txt is the
control: with garbage collection disabled, the lock stays held throughout
the eight-second observation window. That earlier control used a single
long body line. The normal-GC failure also occurred with that earlier body
(lock-gc-default-clean.txt) and GOGC=1 (lock-gc-confirmed.txt).

Preliminary logs are retained for transparency: lock-gc.txt hit a missing
--item in the probe's second CLI command after observing the released lock;
lock-gc-default.txt observed both dialogs but its scratch cleanup raced a
helper's last marker write. Both fixture issues were corrected before the
final reproduction. These failures are separate from the production bug.

## Checks

The dated logs record command, exact commit, working directory, and exit:
build.txt, vet.txt, deps.txt, race.txt. The complete
go test -race -count=1 ./... suite passed (CLI package: 270.933s).
All dependencies listed are this module's packages plus the standard library.

On 2026-10-17 Codex also ran these in the review worktree, both exit 0 with
empty output:

    git diff --check 9b2dfec4d9a511bf2bbaef0dc1afe73db9082fd9 HEAD
    gofmt -l cmd/handoff/popup.go cmd/handoff/popup_test.go cmd/handoff/cmds.go cmd/handoff/main.go cmd/handoff/main_test.go internal/notify/notify.go internal/notify/notify_test.go internal/status/status.go

Source review found no default-button issue in the first dialog: Apple's
[AppleScript command reference](https://developer.apple.com/library/archive/documentation/AppleScript/Conceptual/AppleScriptLangGuide/reference/ASLR_cmds.html)
says that explicit buttons without default button have no default. This is
documentation evidence only, not a new live UI check. Real Deny was not
clicked during this review; the request's prior live run also covers only
Approve. Stock tests cover both outcomes through a dialog substitute.
