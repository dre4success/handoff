#!/usr/bin/env bash
# The reviewer's H16 evidence. Run from a checkout of the commit under review:
#   OUT=<this directory> bash collect.sh
# Alerts and sleep holds go to scratch files. Nothing outside OUT and a
# temporary directory is written; the temporary worktree is removed.
set -u
export OUT=${OUT:?}
COMMIT=$(git rev-parse HEAD)
BASE=ba519c17ae10e7a01d4d9a805fde3ec653f68541
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID

# 1. Build, vet, gofmt, no net, no dependencies; what H16 touches.
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go list -deps ./... | grep net; go.mod; git diff --stat"
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go list -deps ./... matching net: [$(go list -deps ./... | grep -E '(^|/)net(/|$)' | tr '\n' ' ')]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## commits on top of the base: $(git rev-list --count $BASE..HEAD)"
  echo "## files changed after dd07744, where the implementer's evidence was collected:"
  git diff --stat dd07744 HEAD
  echo "## files under internal/record changed by H16: $(git diff --name-only $BASE HEAD -- internal/record | wc -l | tr -d ' ')"
} > "$OUT/build-vet-deps.log" 2>&1

# 2. The record's torn-tail and head.json checks (the reviewer's H3 check), under -race.
{
  header "go test -count=1 -race -v ./internal/record -run 'Torn|Tail|Killed|Repair|Key|Head|Retry'"
  go test -count=1 -race -v ./internal/record -run 'Torn|Tail|Killed|Repair|Key|Head|Retry' 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/h3-record.log" 2>&1

# 3. H16's tests on main, then here.
T='TestH16|TestH11InitWithNoFlags|TestH11BriefCommandsRunForClaudeAndCodex|TestH12ACodexThreadKeepsItsRoleAlone'
{
  header "this commit's session_test.go, pickup_test.go and roles_test.go on $BASE (main), then here: -run '$T'"
  git worktree add -q --detach "$SCRATCH/base" "$BASE"
  cp cmd/handoff/session_test.go cmd/handoff/pickup_test.go cmd/handoff/roles_test.go "$SCRATCH/base/cmd/handoff/"
  echo "## at $BASE (main): expected six FAIL, one PASS"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$T" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)')
  echo; echo "## at $COMMIT: expected all PASS"
  go test -count=1 -v -run "$T" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 4. A dry run of the merge onto main, with objects written to a temporary directory (proposal §9).
{
  header "git merge-tree --write-tree --name-only main $COMMIT, GIT_OBJECT_DIRECTORY in a temporary directory"
  MAIN=$(git rev-parse main)
  OBJ=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/objects
  echo "## main is $MAIN; since the base it changed: $(git diff --name-only $BASE $MAIN | tr '\n' ' ')"
  before=$(git count-objects -v | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  mkdir "$SCRATCH/obj"
  GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git merge-tree --write-tree --name-only "$MAIN" "$COMMIT"
  echo "# merge-tree exit: $? (0: clean, no conflicts)"
  after=$(git count-objects -v | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  echo "## objects in the repository before $before, after $after; in the temporary directory: $(find "$SCRATCH/obj" -type f | wc -l | tr -d ' ')"
} > "$OUT/merge-dry-run.log" 2>&1

# 5. The reviewer's probes.
bash "$OUT/probes.sh" > "$OUT/probes.log" 2>&1

# 6. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
  echo "# alerts sent to the scratch file during this collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} > "$OUT/go-test-race.log" 2>&1
