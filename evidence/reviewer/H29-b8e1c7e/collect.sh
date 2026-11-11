#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H29 round 2 (b8e1c7e). Run as
#   W=<detached worktree at the commit> WB=<a worktree at the base, 70630c4> OUT=<this dir> [ALL=<a saved go test -race -count=1 -v ./... output from W>] bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; WB=${WB:?worktree at the base}; OUT=${OUT:?out}
BASE=70630c4ab85359d1892f5e32ffac5d8c77460ac7
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { sed -E "s|$WB|<worktree at the base>|g; s|$W|<worktree>|g; s|$HOME|~|g"; }
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the evidence commit touches; the tool built at the base and here, compared"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (70630c4):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## diffstat:"; git diff --stat $BASE..HEAD
  echo "## the implementer's evidence names 902e36e; what the commit under review adds to it:"; git diff --stat 902e36e..HEAD
  echo "## since round 1 (0ae44e4):"; git diff --stat 0ae44e4..HEAD
  echo "## files under cmd, internal and go.mod that differ from round 1: [$(git diff --name-only 0ae44e4 HEAD -- cmd internal go.mod | tr '\n' ' ')]"
  echo "## the worktree is clean: [$(git status --short)]"
  echo
  echo "## No code changes? Files under cmd, internal and go.mod that differ from the base, tests aside:"
  git diff --name-only $BASE..HEAD -- cmd internal go.mod | grep -v '_test\.go$' || echo "(none)"
  echo "## handoff built at the base and here (go build -trimpath -buildvcs=false), sha256:"
  (cd "$WB" && go build -trimpath -buildvcs=false -o "$T/handoff-base" ./cmd/handoff) && go build -trimpath -buildvcs=false -o "$T/handoff" ./cmd/handoff
  echo "base: $(shasum -a 256 "$T/handoff-base" | cut -d' ' -f1)"; echo "here: $(shasum -a 256 "$T/handoff" | cut -d' ' -f1)"
  cmp -s "$T/handoff-base" "$T/handoff" && echo "## the two are the same, byte for byte" || echo "## THEY DIFFER"
  echo "## line counts: README.md $(git show $BASE:README.md | wc -l | tr -d ' ') at the base, $(wc -l < README.md | tr -d ' ') here; CLAUDE.md $(git show $BASE:CLAUDE.md | wc -l | tr -d ' ') and $(wc -l < CLAUDE.md | tr -d ' ')"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
{
  header "go test -race -count=1 -v ./... (no -timeout); the test names at the base and here; the notify tests, -count=5"
  if [ -n "${ALL:-}" ] && [ -f "$ALL" ]; then cp "$ALL" "$T/all.out"; echo "# (the run was started at $(stat -f '%SB' -t '%Y-%m-%d %H:%M:%S' "$ALL" 2>/dev/null) local time, in this worktree, and its output read here)"; else go test -race -count=1 -v ./... > "$T/all.out" 2>&1; fi
  grep -E '^(ok|FAIL|\?|panic: test timed out)|^--- FAIL|WARNING: DATA RACE' "$T/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$T/all.out"); failed: $(grep -c '^--- FAIL' "$T/all.out"); skipped: $(grep -c '^--- SKIP' "$T/all.out"); data races: $(grep -c 'WARNING: DATA RACE' "$T/all.out")"
  echo; echo "## the test the commit changes, in that run"
  grep -E '^--- (PASS|FAIL|SKIP): TestEachAlertIsSentOnce' "$T/all.out"
  echo; echo "## the tests' names at the base that are not here, and here that are not at the base"
  (cd "$WB" && go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort) > "$T/base.names"
  go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort > "$T/here.names"
  diff "$T/base.names" "$T/here.names" | grep -E '^[<>]' || echo "(none)"
  echo "# tests at the base: $(wc -l < "$T/base.names" | tr -d ' '); here: $(wc -l < "$T/here.names" | tr -d ' ')"
  echo; echo "## internal/notify's tests, 5 times each under the race detector"
  go test -race -count=5 -v ./internal/notify 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//' | sort | uniq -c
} 2>&1 | clean > "$OUT/go-test-race.log"
