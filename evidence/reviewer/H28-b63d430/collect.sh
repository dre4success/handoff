#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H28 round 2 (b63d430). Run as
#   W=<detached worktree at the commit> W2=<a second one, with goahead-random_test.go.txt copied in as internal/status/zz_reviewer_h28_test.go> WB=<a worktree at the base, b8e1c7e> OUT=<this dir> [ALL=<a saved go test -race -count=1 -v ./... output from W>] bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; W2=${W2:?second worktree}; WB=${WB:?worktree at the base}; OUT=${OUT:?out}
BASE=b8e1c7e6f4fc2811bc1900ef62e8fbacf0e89aa5
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { sed -E "s|$W2|<worktree 2>|g; s|$WB|<worktree at the base>|g; s|$W|<worktree>|g; s|$HOME|~|g"; }
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the evidence commit touches"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (b8e1c7e):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## diffstat:"; git diff --stat $BASE..HEAD
  echo "## the implementer's evidence names 77a48cb; what the commit under review adds to it:"; git diff --stat 77a48cb..HEAD
  echo "## since round 1 (6b2f241):"; git diff --stat 6b2f241..HEAD
  echo "## the worktree is clean: [$(git status --short)]"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
{
  header "go test -race -count=1 -v ./... (no -timeout); the test names at the base and here; the H28 tests, -count=5; the reviewer's own randomized test"
  if [ -n "${ALL:-}" ] && [ -f "$ALL" ]; then cp "$ALL" "$T/all.out"; echo "# (the run was started at $(stat -f '%SB' -t '%Y-%m-%d %H:%M:%S' "$ALL" 2>/dev/null) local time, in this worktree, and its output read here)"; else go test -race -count=1 -v ./... > "$T/all.out" 2>&1; fi
  grep -E '^(ok|FAIL|\?|panic: test timed out)|^--- FAIL|WARNING: DATA RACE' "$T/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$T/all.out"); failed: $(grep -c '^--- FAIL' "$T/all.out"); skipped: $(grep -c '^--- SKIP' "$T/all.out"); data races: $(grep -c 'WARNING: DATA RACE' "$T/all.out")"
  echo; echo "## the H28 tests in that run"
  grep -E '^--- (PASS|FAIL|SKIP): TestH28' "$T/all.out"
  echo; echo "## the tests' names at the base that are not here, and here that are not at the base"
  (cd "$WB" && go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort) > "$T/base.names"
  go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort > "$T/here.names"
  diff "$T/base.names" "$T/here.names" | grep -E '^[<>]' || echo "(none)"
  echo "# tests at the base: $(wc -l < "$T/base.names" | tr -d ' '); here: $(wc -l < "$T/here.names" | tr -d ' ')"
  echo; echo "## the H28 tests, 5 times each under the race detector"
  go test -race -count=5 -run '^TestH28' -v ./cmd/handoff ./internal/status 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//; s/\t[0-9.]+s$//' | sort | uniq -c
  echo; echo "## the reviewer's test (goahead-random_test.go.txt), in a second worktree at the same commit"
  (cd "$W2" && git rev-parse HEAD && git status --short && go test -count=1 -run 'TestReviewerGoAhead' -v ./internal/status 2>&1 | grep -v '^$')
} 2>&1 | clean > "$OUT/go-test-race.log"
