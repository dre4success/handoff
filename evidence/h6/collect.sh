#!/usr/bin/env bash
# Collects the H6 evidence. Run from the repository root, on a clean checkout
# of the commit under review:
#   bash evidence/h6/collect.sh
set -u
OUT=evidence/h6
COMMIT=$(git rev-parse HEAD)
BASE=1b75b87 # main before H6
BY="Opus 5.5 (implementer), Claude Code 2.1.295, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The H6 regressions: the wait and process tests fail on main before H6
#    and pass here. The record tests are not copied back: they use a field
#    (Problem.ByHead) that main does not have.
RUN='TestEditedLastLineIsNotDelivered|TestBrokenRecordDeliversOnlyVerifiedEntries|TestH6EditedLastLineIsWithheld|TestH5ForgedLineDuringAnEmittingWait'
{
  header "the H6 regression tests, on a worktree of $BASE with this commit's wait and process tests copied in, then on this commit"
  W="$SCRATCH/red"
  git worktree add -q --detach "$W" "$BASE"
  cp internal/wait/wait_test.go "$W/internal/wait/"
  cp cmd/handoff/main_test.go cmd/handoff/race_test.go "$W/cmd/handoff/"
  echo "## at $BASE (expected to fail)"
  (cd "$W" && go test -count=1 -v -run "$RUN" ./internal/wait ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:')
  git worktree remove --force "$W"
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN|TestEditedLastLineIsABreak|TestForgedAppend" ./internal/... ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/h6-regressions.log"

# 3. The new process tests and the timing check, repeated.
{
  header "go test -count=5 -v -run 'TestH6|TestH5|TestCheck8|TestCheck13' ./cmd/handoff"
  go test -count=5 -v -run 'TestH6|TestH5|TestCheck8|TestCheck13' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|woke'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/repeat.log"

# 4. Fable's gate probes, unchanged, against this commit. The script's own
#    first output line names its author; the run is the implementer's.
{
  header "bash evidence/gate/H5-089a64f/probes.sh <worktree of this commit> <scratch>   (Fable's script, run by the implementer)"
  W="$SCRATCH/wt"
  git worktree add -q --detach "$W" "$COMMIT"
  bash evidence/gate/H5-089a64f/probes.sh "$W" "$SCRATCH/probes" 2>&1
  git worktree remove --force "$W"
} > "$OUT/gate-probes-rerun.txt"

# 5. Vet, format, dependencies, no net.
{
  header "go vet ./... ; gofmt -l . ; go.mod ; go list -deps (non-standard) ; go list -deps | grep net"
  echo '$ go vet ./...'; go vet ./... 2>&1; echo "# exit: $?"
  echo '$ gofmt -l .   (no output = formatted)'; gofmt -l .
  echo '$ cat go.mod'; cat go.mod
  echo '$ go list -deps -f "{{if not .Standard}}{{.ImportPath}}{{end}}" ./...'
  go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d'
  echo '$ go list -deps ./... | grep -E "^net(/|$)"   (no output = no net package)'
  go list -deps ./... | grep -E '^net(/|$)'; echo "# grep exit: $? (1 = no match)"
} > "$OUT/deps-vet-fmt.log"
echo "collected in $OUT at $COMMIT"
