# Gate reading: the step-1 record format

Gate reviewer: Fable 5.1, Claude Code 2.1.295, the operator's Mac. Date: 2026-10-09. Target: `089a64fc201afcac6ebf70417f284b01ada41e7f` on `h-step1-record`, the request that superseded `7c1d0a2`. Read in a detached worktree, removed afterwards. These files are gate evidence, left uncommitted for the implementer under D10, as GPT-6's were.

## Run

- On the target: `gofmt -l` empty; `go build ./...` and `go vet ./...` clean; `go list -m all` shows only the module; `go list -deps` has no `net`; `go test ./... -count=1` 52 tests pass (24 s); `go test ./... -race -count=1` passes (2 m 15 s).
- `probes.sh` and `probes.txt`: a binary built from the target, run against scratch repositories. Cases A to J are described in the channel entry. To repeat: `bash probes.sh <worktree of 089a64f> <empty scratch dir>`.

## Source

- `internal/record/record.go`: `parse` lines 181 to 244 (the withholding rule at 231 to 242), `coveredByRepair` 246 to 255, `compareHead` 268 to 281.
- `internal/record/store.go`: `Append` 100 to 146, `Repair` 152 to 206, `writeRecord` 225 to 239.
- Go 1.25.3 `internal/poll/fd_fsync_darwin.go` line 22: `File.Sync` on macOS uses `F_FULLFSYNC`, so the sync after each append is a real flush.
- Proposal revision 2.1 §5, row 51.

## Inferred

- A power loss can split the single `write` that `repair` makes; a SIGKILL cannot. Case E simulates the resulting bytes; no real power loss was induced.
