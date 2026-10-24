# Gate reading of H6: head.json as a witness

Gate reviewer: Fable 5.1, Claude Code 2.1.295, the operator's Mac. Date: 2026-10-10. Target: `639ed6fea63aa566ee693da1bc3996c8d70b7da8` on `h6-head-witness`. Read in two detached worktrees, both removed afterwards. Left uncommitted for the implementer under D10.

## Run

- On the target: `gofmt -l` empty; build and vet clean; only the module in `go list -m all`; no `net`; `go test ./... -count=1` 55 tests pass (25 s); `-race` passes (2 m 22 s).
- `probes-rerun.txt`: `evidence/gate/H5-089a64f/probes.sh` as committed on `main` (byte-identical to my copy, checked with `cmp`), run by me against the target. Case C: the edited last line is withheld, `wait` exits 2, no cursor is written, `show` marks it `verified: NO`. Case H: only line 4 on is withheld; #3 stays in "needs my response". Case E: still needs the operator, and #3 is no longer withheld. Cases D, F and I are unchanged. Case G is invalid in this script (duplicate bodies), as in the first run. Case J, added by hand: in the case-C state, `repair` and `post` both refuse with the row-52 message.

## Source

- `internal/record/record.go`: `Snapshot.witness` (rows 52 and 53) and its call from `parse` before the partition; `Problem.ByHead`.
- `internal/record/store.go` `read`: `head.json` is read before `parse`; `compareHead` is skipped when the break was found by `head.json`, so the case is reported once.
- The four new record tests and the two new wait tests named in the diff; `evidence/h6/README.md`.
- Proposal §17 rows 52 to 57 at `afc42b4`.
