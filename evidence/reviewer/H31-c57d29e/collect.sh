#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H31 (c57d29e). Run as
#   W=<detached worktree at the commit> WB=<the base's tree, 37e341b, from git archive> WM=<a scratch copy of W's tree, for mutate.py> OUT=<this dir> ALL=<a saved go test -race -count=1 -json ./... output from W> bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; WB=${WB:?tree at the base}; WM=${WM:?scratch copy}; OUT=${OUT:?out}; ALL=${ALL:?saved test output}
BASE=37e341b3ea84f72dcce589e760c22a34fa48fb97
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { perl -pe "s|\Q$WM\E|<scratch copy>|g; s|\Q$WB\E|<the base's tree>|g; s|\Q$W\E|<worktree>|g; s|\Q$HOME\E|~|g"; }
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the later commits touch"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (37e341b):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## diffstat:"; git diff --stat $BASE..HEAD
  echo "## the implementer's test and mutation logs were made before the evidence commit; what it adds to 2576877:"; git diff --stat 2576877..HEAD
  echo "## files under cmd or internal that differ from the base:"; git diff --stat $BASE..HEAD -- cmd internal | sed '$d'
  echo "## the worktree is clean: [$(git status --short)]"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
{
  header "go test -race -count=1 -json ./... (no -timeout but go's own); the test names at the base and here; the H31 tests, -count=5; the implementer's mutate.py"
  echo "# (the full run was started at $(stat -f '%SB' -t '%Y-%m-%d %H:%M:%S' "$ALL" 2>/dev/null) local time, in this worktree, with a scratch HOME, and its output is read here)"
  python3 -I - "$ALL" <<'EOF'
import json, sys, collections
tests = collections.Counter(); sub = collections.Counter(); races = 0; pk = []
for l in open(sys.argv[1]):
    try: e = json.loads(l)
    except ValueError: continue
    a = e.get("Action")
    if a in ("pass", "fail", "skip"):
        if e.get("Test"): (sub if "/" in e["Test"] else tests)[a] += 1
        else: pk.append((a, e["Package"], e.get("Elapsed")))
    if "DATA RACE" in e.get("Output", ""): races += 1
for a, p, s in sorted(pk, key=lambda x: x[1]): print("%-5s %s  %ss" % (a, p, s))
print("# tests: passed %d, failed %d, skipped %d; subtests: passed %d, failed %d; data races: %d" % (tests["pass"], tests["fail"], tests["skip"], sub["pass"], sub["fail"], races))
EOF
  echo; echo "## the tests' names at the base that are not here, and here that are not at the base"
  (cd "$WB" && go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort) > "$T/base.names"
  go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort > "$T/here.names"
  diff "$T/base.names" "$T/here.names" | grep -E '^[<>]' || echo "(none)"
  echo "# tests at the base: $(wc -l < "$T/base.names" | tr -d ' '); here: $(wc -l < "$T/here.names" | tr -d ' ')"
  echo; echo "## the H31 tests, 5 times each under the race detector"
  go test -race -count=5 -run '^TestH31' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//; s/\t[0-9.]+s$//' | sort | uniq -c
  echo; echo "## the implementer's evidence/h31/mutate.py, run in a scratch copy of the tree: each condition taken out in turn, then the H31 tests"
  (cd "$WM" && python3 -I evidence/h31/mutate.py 2>&1; echo "## the scratch copy's cmd and internal are as in the commit again: $(diff -rq "$W/cmd" cmd > /dev/null && diff -rq "$W/internal" internal > /dev/null && echo yes || echo NO)")
} 2>&1 | clean > "$OUT/go-test-race.log"
