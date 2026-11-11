#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H26 (f3be27c). Run as
#   W=<detached worktree at the commit> W2=<a second one, with appendall-readers_test.go.txt copied in as internal/record/zz_reviewer_h26_test.go> OUT=<this dir> bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; W2=${W2:?second worktree}; OUT=${OUT:?out}
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { sed -E "s|$W2|<worktree 2>|g; s|$W|<worktree>|g; s|$HOME|~|g"; }
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the evidence commit touches"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (1af34c7):"; git log --oneline --no-decorate 1af34c7..HEAD
  echo "## diffstat:"; git diff --stat 1af34c7..HEAD
  echo "## the implementer's evidence names 0d4eecd; what the commit under review adds to it:"; git diff --stat 0d4eecd..HEAD
  echo "## the worktree is clean: [$(git status --short)]"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
{
  header "go test -race -count=1 -v ./... (no -timeout); then the H26 tests, -count=5; then the reviewer's own AppendAll test, -count=3"
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$OUT/.all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|\?|panic: test timed out)|^--- FAIL|WARNING: DATA RACE' "$OUT/.all.out"
  echo "# passed: $(grep -c '^--- PASS' "$OUT/.all.out"); failed: $(grep -c '^--- FAIL' "$OUT/.all.out"); skipped: $(grep -c '^--- SKIP' "$OUT/.all.out"); data races: $(grep -c 'WARNING: DATA RACE' "$OUT/.all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the H26 tests in that run"
  grep -E '^--- (PASS|FAIL|SKIP): Test(H26|AppendAll)' "$OUT/.all.out"
  rm -f "$OUT/.all.out"
  echo; echo "## the H26 tests, 5 times each under the race detector"
  go test -race -count=5 -run '^TestH26' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//' | sort | uniq -c
  go test -race -count=5 -run '^TestAppendAll' -v ./internal/record 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//' | sort | uniq -c
  echo; echo "## the reviewer's test (appendall-readers_test.go.txt), in a second worktree at the same commit, 3 times under the race detector"
  (cd "$W2" && git rev-parse HEAD && git status --short && go test -race -count=3 -run 'TestReviewerAppendAll' -v ./internal/record 2>&1 | grep -v '^$')
} 2>&1 | clean > "$OUT/go-test-race.log"
