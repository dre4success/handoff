#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H27 (9315eb4). Run as
#   [ONLY=build] W=<detached worktree at the commit> W2=<a second one, with H26's appendall-readers_test.go.txt copied in as internal/record/zz_reviewer_h26_test.go> WB=<a worktree at the base, 3119db5> OUT=<this dir> bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; W2=${W2:?second worktree}; WB=${WB:?worktree at the base}; OUT=${OUT:?out}
BASE=3119db5d17d7888cd4e5085a28a2e162038f337f
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { sed -E "s|$W2|<worktree 2>|g; s|$WB|<worktree at the base>|g; s|$W|<worktree>|g; s|$HOME|~|g"; }
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the evidence commit touches; the rename, word by word"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (3119db5):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## diffstat:"; git diff --stat $BASE..HEAD
  echo "## the implementer's evidence names 791d933; what the commit under review adds to it:"; git diff --stat 791d933..HEAD
  echo "## the worktree is clean: [$(git status --short)]"
  echo
  echo "## Is it only the name? The non-test Go source at the base and here, with every tell/Tell/fyi/Fyi word and its article folded to one token, then diffed (comment lines re-wrapped by the rename show as moved words only):"
  fold() { git show "$1:$2" | perl -pe 's/\b[Aa]n? (tell|fyi)\b/X/g; s/\b(tell|fyi|Tell|Fyi)\b/X/g; s/cmd(Tell|Fyi)/cmdX/g' | tr -s ' \t\n' '\n\n\n'; }
  for f in cmd/handoff/brief.go cmd/handoff/cmds.go cmd/handoff/main.go cmd/handoff/session.go internal/record/record.go internal/wait/wait.go; do
    d=$(diff <(fold $BASE "$f") <(fold HEAD "$f") | grep -E '^[<>]' | tr '\n' ' ')
    echo "$f: [${d}]"
  done
  echo "## files under cmd and internal that differ from the base, other than those six and the tests:"
  git diff --name-only $BASE..HEAD -- cmd internal | grep -vE '_test\.go$|cmd/handoff/(brief|cmds|main|session)\.go|internal/record/record\.go|internal/wait/wait\.go' || echo "(none)"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
[ "${ONLY:-}" = build ] && exit 0
{
  header "go test -race -count=1 -v ./... (no -timeout); then the H26 and H27 tests, -count=5; the reviewer's own AppendAll test, -count=3; TestH27TellIsNowFyi at the base"
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$OUT/.all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|\?|panic: test timed out)|^--- FAIL|WARNING: DATA RACE' "$OUT/.all.out"
  echo "# passed: $(grep -c '^--- PASS' "$OUT/.all.out"); failed: $(grep -c '^--- FAIL' "$OUT/.all.out"); skipped: $(grep -c '^--- SKIP' "$OUT/.all.out"); data races: $(grep -c 'WARNING: DATA RACE' "$OUT/.all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the H26 and H27 tests in that run"
  grep -E '^--- (PASS|FAIL|SKIP): Test(H26|H27|AppendAll)' "$OUT/.all.out"
  echo; echo "## the tests' names at the base that are not here, and here that are not at the base"
  (cd "$WB" && go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort) > "$OUT/.base.names"
  go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort > "$OUT/.here.names"
  diff "$OUT/.base.names" "$OUT/.here.names" | grep -E '^[<>]'
  echo "# tests at the base: $(wc -l < "$OUT/.base.names" | tr -d ' '); here: $(wc -l < "$OUT/.here.names" | tr -d ' ')"
  rm -f "$OUT/.all.out" "$OUT/.base.names" "$OUT/.here.names"
  echo; echo "## the H26 and H27 tests, 5 times each under the race detector"
  go test -race -count=5 -run '^TestH2[67]' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//' | sort | uniq -c
  go test -race -count=5 -run '^TestAppendAll' -v ./internal/record 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//' | sort | uniq -c
  echo; echo "## the reviewer's H26 test (evidence/reviewer/H26-8162dbf/appendall-readers_test.go.txt, unchanged: the store takes any kind), in a second worktree at this commit, 3 times under the race detector"
  (cd "$W2" && git rev-parse HEAD && git status --short && go test -race -count=3 -run 'TestReviewerAppendAll' -v ./internal/record 2>&1 | grep -v '^$')
  echo; echo "## TestH27TellIsNowFyi at the base (3119db5), with this commit's fyi_test.go's H27 test copied in alone as cmd/handoff/zz_h27_test.go"
  awk '/^\/\/ H27 \(row 94\)/{p=1} p' "$W/cmd/handoff/fyi_test.go" > "$OUT/.h27body"
  { printf 'package main\n\nimport (\n\t"strings"\n\t"testing"\n)\n\n'; cat "$OUT/.h27body"; } > "$WB/cmd/handoff/zz_h27_test.go"
  (cd "$WB" && git rev-parse HEAD && go test -count=1 -run '^TestH27TellIsNowFyi$' -v ./cmd/handoff 2>&1 | grep -vE '^$|^=== ')
  rm -f "$WB/cmd/handoff/zz_h27_test.go" "$OUT/.h27body"
  echo "## the base worktree is clean again: [$(git -C "$WB" status --short)]"
} 2>&1 | clean > "$OUT/go-test-race.log"
