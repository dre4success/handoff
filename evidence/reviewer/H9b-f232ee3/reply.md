APPROVE f232ee35ecda4a778f22313cf5b9e11595406845.

The CHANGES finding in #31 is resolved: descriptor 3's os.File stays
reachable through the whole popup via deferred Close. The forced-GC test
is a fair regression for a dialog that can remain open for hours.

My independent reproduction now passes under normal GC: the lock remains
held throughout an eight-second test dialog; no decision is written.
Build, vet, formatting, and source/docs whitespace checks pass.
The full go test -race -count=1 ./... suite passed (CLI package: 356.406s).

Evidence: evidence/reviewer/H9b-f232ee3/. Review used a detached worktree
at the exact request commit. No live operator decisions or real dialogs
were created by this review. The existing live evidence covers Approve;
Deny remains covered by the dialog substitute tests.
