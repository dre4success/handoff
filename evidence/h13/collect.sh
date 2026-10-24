#!/usr/bin/env bash
# Collects the H13 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h13/collect.sh
# BASE is what H13 builds on (main, after H12). FIX is the commit with the
# sleep note fix alone, whose wait tests still build on BASE. Alerts go to
# a file; the live check holds real sleep assertions for a few seconds.
set -u
OUT=${OUT:-evidence/h13}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
FIX=${FIX:-$(git log --format=%h -1 --grep='^fix: wait notes each sleep once')}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
unset HANDOFF_ROLE HANDOFF_TEST_AWAKE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
[ -n "$FIX" ] || { echo "no fix commit found"; exit 1; }
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/base" "$BASE"

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 2. The H13 tests, in full.
{
  header "go test -count=1 -v on the H13 tests"
  go test -count=1 -v -run 'TestOwedFollowsTheMoveAndItsAge' ./internal/status 2>&1 | grep -E '^(--- |ok|FAIL)'
  go test -count=1 -v -run 'TestDeadlineIsWallClockAndGapIsReported|TestEntryIsDeliveredBeforeAnExpiredDeadline|TestOneSleepWithBriefWakesIsReportedOnce|TestAPauseUnderAMinuteIsNotReported|TestAwakeWhileAnAgentOwesAMove' ./internal/wait 2>&1 | grep -E '^(--- |ok|FAIL)'
  go test -count=1 -v ./internal/awake 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:'
  go test -count=1 -v -run 'TestH13' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
} > "$OUT/h13-tests.log" 2>&1

# 3. The sleep note tests on main, then here. FIX's wait tests are this
# commit's without the keep-awake test, which needs this commit's code.
RUN='TestDeadlineIsWallClockAndGapIsReported|TestEntryIsDeliveredBeforeAnExpiredDeadline|TestOneSleepWithBriefWakesIsReportedOnce|TestAPauseUnderAMinuteIsNotReported'
{
  header "the sleep note tests of $FIX on $BASE (main), then here"
  git show "$FIX:internal/wait/wait_test.go" > "$SCRATCH/base/internal/wait/wait_test.go" || exit 1
  echo "## at $BASE (expected to fail: a line for each late tick, a 50 s pause noted, a sleep that ends with a delivery not noted)"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$RUN" ./internal/wait 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | cut -c1-400)
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./internal/wait 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 4. The H7 trial's 27 logged sleeps, replayed through one wait on main and
# here.
{
  header "trial-replay_test.go.txt in internal/wait, on $BASE (main), then here; TRIAL_LOG=evidence/trial/implementer-wait.log"
  for where in "$SCRATCH/base" "$PWD"; do
    echo "## at $([ "$where" = "$PWD" ] && echo "$COMMIT" || echo "$BASE (main)")"
    cp "$OUT/trial-replay_test.go.txt" "$where/internal/wait/trial_replay_test.go"
    (cd "$where" && TRIAL_LOG="$PWD/evidence/trial/implementer-wait.log" go test -count=1 -v -run TestTrialReplay ./internal/wait 2>&1 | grep -E '^(--- |ok|FAIL)|trial_replay_test\.go:[0-9]+:' | sed -E 's/^ +trial_replay_test\.go:[0-9]+: //')
    rm "$where/internal/wait/trial_replay_test.go"
    echo
  done
} > "$OUT/trial-replay.log" 2>&1

# 5. The live check: a real wait and hook, the real caffeinate.
bash "$OUT/live-awake.sh" "$SCRATCH/handoff" 2>&1 | sed "s|$SCRATCH|<scratch>|g" > "$OUT/live-awake.txt"

# 6. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1
