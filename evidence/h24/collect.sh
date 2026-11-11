#!/usr/bin/env bash
# Collects the H24 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h24/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H24 builds on: main after v0.1.2.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h24}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-bf9cb0b}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
KEEP=${KEEP:-}
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
# The CLIs' user settings are scratch folders, never the operator's.
NONE="$SCRATCH/none"
mkdir -p "$NONE/claude" "$NONE/codex"
export CLAUDE_CONFIG_DIR="$NONE/claude" CODEX_HOME="$NONE/codex"
echo "please review" > "$SCRATCH/body.md"

# 1. Every test under the race detector with go test's default limit, 10
# minutes (no -timeout), built from BASE (before) and from here (after), one
# after the other, timed. Set KEEP to a folder to keep the full outputs.
mkdir "$SCRATCH/base-src" && git archive "$BASE" | tar -x -C "$SCRATCH/base-src"
for side in before after; do
  dir=$PWD; [ $side = before ] && dir="$SCRATCH/base-src"
  {
    header "go test -count=1 -race -v ./... (no -timeout: the default 10 minutes), $side: built from $([ $side = before ] && echo "$BASE" || echo here), timed (one line per test)"
    start=$(date +%s)
    (cd "$dir" && go test -count=1 -race -v ./... > "$SCRATCH/go-test-$side.out" 2>&1); code=$?
    grep -E '^(--- |ok|FAIL|PASS|panic: test timed out)|_test\.go:[0-9]+:' "$SCRATCH/go-test-$side.out"
    echo "# go test exit: $code; wall time: $(( $(date +%s) - start )) s"
  } > "$OUT/go-test-race-$side.log"
  [ -n "$KEEP" ] && cp "$SCRATCH/go-test-$side.out" "$KEEP/"
done

# 2. The flaky test's cause, made certain. With quiet_after 1s set before the
# claim, any handoff command that checks for silence more than 1 s after the
# claim tells the operator, the claim itself included when it runs slow. A
# second claim 1.2 s later runs that same check. The old order: the second
# claim tells, and the wait has nothing left to tell. The new order: the
# limit is cut after the claims, and the wait tells. Then each changed test,
# 20 times under -race.
go build -o "$SCRATCH/after" ./cmd/handoff
clean() { sed "s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g"; }
quiet() { python3 -I -c 'import json, sys
p = sys.argv[1]; c = json.load(open(p)); c["quiet_after"] = sys.argv[2]
open(p, "w").write(json.dumps(c, indent=2) + "\n")' "$1/.handoff/config.json" "$2"; }
{
  header "the old and the new order of TestH8QuietClaimTellsTheOperatorOnce, by hand, with a second claim standing in for a slow one; then the two changed tests, -race -count=20"
  for order in old new; do
    d="$SCRATCH/p-$order"; mkdir "$d"; export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts-$order.txt"; : > "$HANDOFF_TEST_NOTIFY"
    git -C "$d" init -q -b main && git -C "$d" commit -q --allow-empty -m base && git -C "$d" switch -q -c w1 && git -C "$d" commit -q --allow-empty -m work
    run() { (cd "$d" && "$SCRATCH/after" "$@"); }
    run init > /dev/null
    echo; echo "## $order order"
    [ $order = old ] && { quiet "$d" 1s; echo "quiet_after: 1s, before the request"; }
    HANDOFF_ROLE=implementer run post --item W1 --to reviewer --commit HEAD --title "W1 work" --body "$SCRATCH/body.md"
    HANDOFF_ROLE=reviewer run claim 1
    sleep 1.2
    echo "# 1.2 s later, a second claim, as a claim slow to check would be:"
    HANDOFF_ROLE=reviewer run claim 1
    [ $order = new ] && { quiet "$d" 1s; echo "quiet_after: 1s, after the claims"; }
    echo "# wait --for implementer --timeout 4s, its stderr:"
    run wait --for implementer --timeout 4s 2>&1 >/dev/null; echo "# wait exit $?"
    echo "# told the operator ($(wc -l < "$SCRATCH/alerts-$order.txt" | tr -d ' ')):"; cat "$SCRATCH/alerts-$order.txt"
  done
  export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
  echo; echo "## go test -race -count=20 -run 'TestH8QuietClaimTellsTheOperatorOnce|TestH8ClaimingAgainWritesNothingAndTellsOnce' ./cmd/handoff"
  go test -race -count=20 -run 'TestH8QuietClaimTellsTheOperatorOnce$|TestH8ClaimingAgainWritesNothingAndTellsOnce$' ./cmd/handoff 2>&1; echo "# exit $?"
} 2>&1 | clean > "$OUT/flake.log"

# Which tests run in parallel, and which alone, from the source.
{
  header "the tests that call t.Parallel, and those that don't"
  for f in cmd/handoff/*_test.go; do
    awk -v f="$(basename "$f")" '/^func Test[A-Za-z0-9_]*\(t \*testing\.T\) \{$/ { name = $2; sub(/\(.*/, "", name); getline nxt; print (nxt ~ /t\.Parallel\(\)/ ? "parallel" : "alone   "), f, name }' "$f"
  done | sort
  echo "# $(grep -c 't.Parallel()' cmd/handoff/*_test.go | awk -F: '{s+=$2} END {print s}') parallel"
} > "$OUT/parallel.log"

# 3. The sweep of every tracked file here.
{
  header "H14's sweep of every tracked file here"
  git ls-files -z | python3 -I -c '
import re, sys
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
files = [f for f in sys.stdin.read().split("\0") if f]
print(f"{len(files)} files")
for label, rx in pats:
    r = re.compile(rx)
    lines = names = 0
    for f in files:
        names += bool(r.search(f))
        try:
            text = open(f, encoding="utf-8", errors="surrogateescape").read()
        except OSError:
            continue
        lines += sum(1 for l in text.split("\n") if r.search(l))
    print(f"{label}: {lines} lines, {names} file names")
' "$PATTERNS"
} > "$OUT/names.log" 2>&1

# 4. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} > "$OUT/deps-vet-fmt.log" 2>&1
