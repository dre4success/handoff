#!/usr/bin/env bash
# Collects the H6 round-2 evidence. Run from the root of a checkout of the
# commit under review. The reviewers' probe scripts are committed on main;
# MAIN points at a checkout of main (default: the shared checkout).
#   bash evidence/h6/r2/collect.sh
set -u
OUT=evidence/h6/r2
MAIN=${MAIN:-~/handoff}
COMMIT=$(git rev-parse HEAD)
BASE=639ed6f # H6 as reviewed, with the CHANGES
BY="Opus 5.5 (implementer), Claude Code 2.1.295, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The round-2 regressions fail on 639ed6f and pass here.
RUN='TestAppendAfterACutThatStopsBeforeHeadIsNotAnEdit|TestH6CutThenPostKilledAfterSyncIsRecoverable|TestH6CutThenRepairKilledAfterSyncIsRecoverable'
{
  header "the H6 round-2 regressions, on a worktree of $BASE with this commit's tests copied in, then on this commit"
  W="$SCRATCH/red"
  git worktree add -q --detach "$W" "$BASE"
  cp internal/record/record_test.go "$W/internal/record/"
  cp cmd/handoff/main_test.go cmd/handoff/race_test.go "$W/cmd/handoff/"
  echo "## at $BASE (expected to fail)"
  (cd "$W" && go test -count=1 -v -run "$RUN" ./internal/record ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:')
  git worktree remove --force "$W"
  echo
  echo "## at $COMMIT (expected to pass), with the H6 round-1 regressions"
  go test -count=1 -v -run "$RUN|TestEditedLastLine|TestForgedAppend|TestBrokenRecordDeliversOnlyVerifiedEntries|TestH6EditedLastLineIsWithheld|TestH5ForgedLineDuringAnEmittingWait" ./internal/... ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log"

# 3. GPT-6's H6 probes, unchanged, against this commit's binary. Its first
#    output line names its author; the run is the implementer's.
{
  header "python3 -I $MAIN/evidence/reviewer/H6-639ed6f/probes.py <binary of this commit>   (GPT-6's script, run by the implementer)"
  python3 -I "$MAIN/evidence/reviewer/H6-639ed6f/probes.py" "$SCRATCH/handoff" 2>&1
  echo "# exit: $?"
} > "$OUT/reviewer-probes-rerun.txt"

# 4. Fable's gate probes, unchanged, against this commit.
{
  header "bash $MAIN/evidence/gate/H5-089a64f/probes.sh <worktree of this commit> <scratch>   (Fable's script, run by the implementer)"
  W="$SCRATCH/wt"
  git worktree add -q --detach "$W" "$COMMIT"
  bash "$MAIN/evidence/gate/H5-089a64f/probes.sh" "$W" "$SCRATCH/probes" 2>&1
  git worktree remove --force "$W"
} > "$OUT/gate-probes-rerun.txt"

# 5. Repeats, and vet, format and dependencies.
{
  header "go test -count=5 -v -run 'TestH6|TestH5|TestCheck3|TestRow51|TestCheck8' ./cmd/handoff"
  go test -count=5 -v -run 'TestH6|TestH5|TestCheck3|TestRow51|TestCheck8' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|woke'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/repeat.log"
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
