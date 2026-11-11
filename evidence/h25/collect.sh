#!/usr/bin/env bash
# Collects the H25 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h25/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H25 builds on: main after H24.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h25}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-7c5b98a}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
# The CLIs' user settings and the home folder are scratch folders, never the
# operator's.
export HOME="$SCRATCH/home" CLAUDE_CONFIG_DIR="$SCRATCH/home/.claude" CODEX_HOME="$SCRATCH/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
go build -o "$SCRATCH/handoff" ./cmd/handoff
clean() { sed "s|$SCRATCH/handoff|handoff|g; s|$SCRATCH|<scratch>|g"; }

# 1. The README's line, before and after, and every `handoff settings`
# command the README names, run, with the one it named before. Then the
# brief's offer, which names the commands the README now gives.
{
  header "the README's change; every handoff settings command it names, run in a scratch project; the brief's offer to a session with no hooks"
  echo "## git diff $BASE -- README.md"
  git diff "$BASE" -- README.md
  p="$SCRATCH/project"; mkdir "$p"
  git -C "$p" init -q -b main && git -C "$p" commit -q --allow-empty -m base
  (cd "$p" && "$SCRATCH/handoff" init > /dev/null)
  run() { echo; echo "## $*"; (cd "$p" && "$SCRATCH/$@") > "$SCRATCH/o" 2>&1; c=$?; head -3 "$SCRATCH/o"; echo "# exit $c"; }
  echo; echo "## the settings commands in the README now:"
  grep -o '`handoff settings[^`]*`' README.md | sort -u
  run handoff settings --global
  grep -o '`handoff settings[^`]*`' README.md | sort -u | tr -d '`' | while read -r _ args; do run handoff $args; done
  echo; echo "## brief --for reviewer, outside a session, with no hooks: its offer"
  (cd "$p" && "$SCRATCH/handoff" brief --for reviewer) | grep -o 'If your session has no handoff hooks.*starts this session again\.'
} 2>&1 | clean > "$OUT/readme.log"

# 2. TestH12ACodexThreadKeepsItsRoleAlone runs in parallel and runs a
# listener to its 1 s timeout. Under load it must still pass. Eight copies at
# once, as up to eight parallel tests run together here, each 5 times under
# the race detector, while the whole package runs under it beside them.
{
  header "go test -race -count=1 ./cmd/handoff in the background; meanwhile 8 at once of go test -race -count=5 -run '^TestH12ACodexThreadKeepsItsRoleAlone\$' ./cmd/handoff"
  go test -race -c -o "$SCRATCH/cmd.test" ./cmd/handoff
  start=$(date +%s)
  go test -count=1 -race -v ./cmd/handoff > "$SCRATCH/whole.out" 2>&1 &
  whole=$!
  sleep 20
  echo "## TestH12ACodexThreadKeepsItsRoleAlone, 8 copies at once, each -count=5, from 20 s into the whole run"
  pids=()
  for i in 1 2 3 4 5 6 7 8; do
    (cd cmd/handoff && "$SCRATCH/cmd.test" -test.count=5 -test.v -test.run '^TestH12ACodexThreadKeepsItsRoleAlone$' > "$SCRATCH/h12a-$i.out" 2>&1) &
    pids+=($!)
  done
  for i in 1 2 3 4 5 6 7 8; do
    wait "${pids[$((i-1))]}"; c=$?
    echo "copy $i: exit $c; $(grep -c '^--- PASS' "$SCRATCH/h12a-$i.out") passed, $(grep -c '^--- FAIL' "$SCRATCH/h12a-$i.out") failed; each took $(grep '^--- ' "$SCRATCH/h12a-$i.out" | sed -E 's/.*\((.*)\)/\1/' | tr '\n' ' ')"
    grep -E '^--- FAIL|_test\.go:[0-9]+:' "$SCRATCH/h12a-$i.out"
  done
  echo "# all 8 done $(( $(date +%s) - start )) s after the whole run started"
  wait $whole; code=$?
  echo; echo "## the whole package beside them: its result, and its tests that failed, if any"
  grep -E '^(ok|FAIL|panic: test timed out)|^--- FAIL' "$SCRATCH/whole.out"
  echo "# passed: $(grep -c '^--- PASS' "$SCRATCH/whole.out"); exit $code; wall time $(( $(date +%s) - start )) s"
} 2>&1 | clean > "$OUT/h12a-under-load.log"

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
