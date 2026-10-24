#!/usr/bin/env bash
# Collects the step-1 evidence logs. Run from the repository root, on a clean
# checkout of the commit under review:
#   bash evidence/step1/collect.sh
set -u
ROOT=$(pwd)
OUT=evidence/step1
COMMIT=$(git rev-parse HEAD)
BY="Opus 5.5 (implementer), Claude Code 2.1.295, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"

header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}

# 1. Every test, verbose, under the race detector. Under -race the CLI that
#    the process-level tests start is race-built as well.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The timing and process checks, repeated to look for flakiness.
{
  header "go test -count=5 -v -run 'TestCheck3|TestCheck8|TestCheck13|TestCheck19|TestRow51|TestH5' ./cmd/handoff ; go test -count=3 -v -run TestCheck1Fifty ./cmd/handoff"
  go test -count=5 -v -run 'TestCheck3|TestCheck8|TestCheck13|TestCheck19|TestRow51|TestH5' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|woke'
  echo "# go test exit: ${PIPESTATUS[0]}"
  go test -count=3 -v -run TestCheck1Fifty ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|processes'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/repeat.log"

# 3. Vet, format, dependencies, no net.
{
  header "go vet ./... ; gofmt -l . ; cat go.mod ; go list -deps (non-standard) ; go list -deps | grep net"
  echo '$ go vet ./...'; go vet ./... 2>&1; echo "# exit: $?"; echo
  echo '$ gofmt -l .   (no output = formatted)'; gofmt -l .; echo
  echo '$ cat go.mod   (no require lines = no third-party dependency)'; cat go.mod; echo
  echo '$ go list -deps -f "{{if not .Standard}}{{.ImportPath}}{{end}}" ./...'
  go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d'; echo
  echo '$ go list -deps ./... | grep -E "^net(/|$)"   (no output = no net package)'
  go list -deps ./... | grep -E '^net(/|$)'; echo "# grep exit: $? (1 = no match)"
} > "$OUT/deps-vet-fmt.log"

# 4. The H5 regressions: they fail on the reviewed commit 7c1d0a2 and pass here.
{
  header "the H5 regression tests, on a worktree of 7c1d0a2 with this commit's tests copied in, then on this commit"
  W=$(mktemp -d)/red
  git worktree add -q --detach "$W" 7c1d0a2
  cp internal/wait/wait_test.go "$W/internal/wait/"
  cp internal/status/status_test.go "$W/internal/status/"
  cp cmd/handoff/main_test.go cmd/handoff/race_test.go "$W/cmd/handoff/"
  RUN='TestDamageArrivingDuringAWaitStopsIt|TestBrokenRecordDeliversOnlyVerifiedEntries|TestNewAdvisoryDuringAWaitIsPrinted|TestH5ForgedLineDuringAnEmittingWait|TestH5LinkedEscalationIsNotAnAnswer'
  echo "## at 7c1d0a2 (expected to fail)"
  (cd "$W" && go test -count=1 -v -run "$RUN" ./internal/wait ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:')
  echo "## at 7c1d0a2, status (expected to fail to compile or fail: the test is new)"
  (cd "$W" && go test -count=1 -v -run TestOnlyANoteToTheReviewerAnswers ./internal/status 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:')
  git worktree remove --force "$W"
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN|TestOnlyANoteToTheReviewerAnswers|TestAppendedLineWithAWrongPrevIsNotVerified|TestEditedMiddleLineBreaksChain" ./internal/... ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/h5-regressions.log"

# 5. The hand run.
bash "$OUT/demo.sh" > "$OUT/demo.log" 2>&1
echo "collected in $OUT at $COMMIT"
