#!/usr/bin/env bash
# Collects the H23 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h23/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H23 builds on: H22 (f773529).
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h23}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-f773529}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
KEEP=${KEEP:-}
trap '[ -n "$KEEP" ] && cp "$SCRATCH/go-test.out" "$KEEP" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
# The CLIs' user settings are scratch folders, never the operator's.
NONE="$SCRATCH/none"
mkdir -p "$NONE/claude" "$NONE/codex"
export CLAUDE_CONFIG_DIR="$NONE/claude" CODEX_HOME="$NONE/codex"
echo "please review" > "$SCRATCH/body.md"

# 1. Every test under the race detector. Under -race, cmd/handoff takes close
# to go test's default 10-minute limit, so the limit is 30 minutes. Set KEEP
# to a file to keep the full output.
{
  header "go test -count=1 -race -timeout 30m -v ./... (one line per test)"
  go test -count=1 -race -timeout 30m -v ./... > "$SCRATCH/go-test.out" 2>&1; code=$?
  grep -E '^(--- |ok|FAIL|PASS|panic: test timed out)|_test\.go:[0-9]+:' "$SCRATCH/go-test.out"
  echo "# go test exit: $code"
} > "$OUT/go-test-race.log"

# Built from BASE (before) and from here (after).
mkdir "$SCRATCH/base-src" && git archive "$BASE" | tar -x -C "$SCRATCH/base-src"
(cd "$SCRATCH/base-src" && go build -o "$SCRATCH/before" ./cmd/handoff)
go build -o "$SCRATCH/after" ./cmd/handoff
clean() { sed "s|$SCRATCH/before|handoff|g; s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g"; }

# 2. The implementer's brief (its line on asking for a review), post and
# note, before and after, as reviewers' sessions come and go. A sleep
# stands in for Claude Code's process; a Codex thread has none.
{
  header "brief, post and note: built from $BASE (before) and from here (after), as reviewers' sessions come and go"
  for bin in before after; do
    d="$SCRATCH/p-$bin"; mkdir "$d"
    git -C "$d" init -q -b main && git -C "$d" commit -q --allow-empty -m base && git -C "$d" switch -q -c w1 && git -C "$d" commit -q --allow-empty -m work
    run() { (cd "$d" && "$@"); }
    asks() { echo "## $bin: $1"; run env HANDOFF_ROLE=implementer "$SCRATCH/$bin" brief | grep '^To ask for a review'; }
    sends() { local label=$1; shift; echo "## $bin: $label"; run env HANDOFF_ROLE=implementer "$SCRATCH/$bin" "$@" 2>&1; echo "# exit $?"; }
    run "$SCRATCH/$bin" init > /dev/null
    echo; asks "init's roles, and no reviewer has a session yet"
    sleep 600 & rev=$!
    run env CLAUDE_CODE_SESSION_ID=s-rev-$bin CLAUDE_PID=$rev "$SCRATCH/$bin" role reviewer > /dev/null
    asks "reviewer has a session (Claude Code); reviewer-2 has none"
    sends "post to reviewer" post --item W1 --to reviewer --commit HEAD --title "W1 work" --body "$SCRATCH/body.md"
    sends "post to reviewer-2" post --item W2 --to reviewer-2 --commit HEAD --title "W2 work" --body "$SCRATCH/body.md"
    sends "note to the operator" note --item W1 --to operator --title "A question" --body "$SCRATCH/body.md"
    run env CODEX_THREAD_ID=t-rev-2-$bin "$SCRATCH/$bin" role reviewer-2 > /dev/null
    asks "reviewer-2 has a session too (Codex)"
    kill $rev; wait $rev 2>/dev/null
    asks "reviewer's session has ended; its role stays given"
    sends "note to reviewer" note --item W1 --to reviewer --title "A note" --body "$SCRATCH/body.md"
  done
} 2>&1 | clean > "$OUT/outputs.log"

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
