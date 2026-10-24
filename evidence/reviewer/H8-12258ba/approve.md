APPROVE on request #19, commit 12258baf57c0ff29b7888f06a21d0b6decffa149.

No blocking findings remain. The earlier failed-send loss and false success
message are fixed. Sending and completion marking are now protected by a
stable, exclusive non-blocking flock; stale-attempt deletion is gone, so a
new owner cannot have its claim removed by a competing recovery process.
The lock remains held across sleep and is released by process exit. Retries
and backoff remain intact. Repeated claims intentionally remain idempotent,
with that behavior and the record-only quiet policy stated in rows 62/63.

Run (reviewer), saved in evidence/reviewer/H8-2daf1b7/:
- All three independent notification probes pass under -race: delivery
  recovery across processes, marker-storage recovery in a live notifier,
  and the concurrent recovery/interrupted-acknowledgement interleaving.
- The full stock go test -race -count=1 ./... suite passes (CLI: 185.959s).
- go build ./..., go vet ./..., and gofmt inspection pass.
- go list -deps ./... has no net package; the module has no third-party
  dependency.

Run (reviewer), saved in evidence/reviewer/H8-12258ba/:
- The exact revision diff changes only archived probe input paths in the
  round-2 and round-3 collectors. Paths now match the fixtures in testdata.
- git diff --exit-code confirms every other path, including executable
  source and tests, is identical to the independently validated 2daf1b7.
- git diff --check against the request's base passes.

The collector-only revision therefore carries forward the completed code
validation. Review used exact-commit git show/diff from a detached temporary
worktree; the shared checkout was never switched by the reviewer.

The reviewer-introduced evidence archive issue was corrected separately:
probe source is now under testdata, and go list ./... on main passes again.

Scope: this approves the overdue-move and decision alerts described by the
request. It does not close presence check 15, and it does not claim that a
real Mac notification was visually confirmed; that observation remains
the operator's. A long active review can still be reported quiet, as row 62 states.

H8 now awaits the operator's decide. The reviewer stops after this APPROVE.
