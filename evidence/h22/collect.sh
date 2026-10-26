#!/usr/bin/env bash
# Collects the H22 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h22/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H22 builds on: main after H21.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h22}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-b6d164e}
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
NONE="$SCRATCH/none" SET="$SCRATCH/set"
mkdir -p "$NONE/claude" "$NONE/codex" "$SET/claude" "$SET/codex/rules"

# 1. Every test under the race detector. Under -race, cmd/handoff takes close
# to go test's default 10-minute limit, so the limit is 30 minutes. Set KEEP
# to a file to keep the full output.
{
  header "go test -count=1 -race -timeout 30m -v ./... (one line per test)"
  CLAUDE_CONFIG_DIR="$NONE/claude" CODEX_HOME="$NONE/codex" go test -count=1 -race -timeout 30m -v ./... > "$SCRATCH/go-test.out" 2>&1; code=$?
  grep -E '^(--- |ok|FAIL|PASS|panic: test timed out)|_test\.go:[0-9]+:' "$SCRATCH/go-test.out"
  echo "# go test exit: $code"
} > "$OUT/go-test-race.log"

# Built from BASE (before) and from here (after).
mkdir "$SCRATCH/base-src" && git archive "$BASE" | tar -x -C "$SCRATCH/base-src"
(cd "$SCRATCH/base-src" && go build -o "$SCRATCH/before" ./cmd/handoff)
go build -o "$SCRATCH/after" ./cmd/handoff
clean() { sed "s|$SCRATCH/before|handoff|g; s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g"; }

# 2. init, before and after, with nothing set up, with Codex alone set up,
# and with both set up as settings --global says.
out=$(CLAUDE_CONFIG_DIR="$SET/claude" "$SCRATCH/after" settings claude --global)
printf '%s\n' "$out" | sed -n '/^{/,$p' > "$SET/claude/settings.json"
out=$(CODEX_HOME="$SET/codex" "$SCRATCH/after" settings codex --global)
printf '%s\n' "$out" | sed -n "\\|^$SET/codex/hooks.json:\$|,/^\$/p" | sed '1d;$d' > "$SET/codex/hooks.json"
printf '%s\n' "$out" | sed -n "\\|^$SET/codex/rules/handoff.rules:\$|,\$p" | sed '1d' > "$SET/codex/rules/handoff.rules"
{
  header "init: built from $BASE (before) and from here (after); nothing set up, Codex alone set up, both set up"
  wc -l "$SET/claude/settings.json" "$SET/codex/hooks.json" "$SET/codex/rules/handoff.rules"
  n=0
  for setup in "nothing set up:$NONE/claude:$NONE/codex" "Codex alone set up:$NONE/claude:$SET/codex" "both set up:$SET/claude:$SET/codex"; do
    IFS=: read -r label claude codex <<< "$setup"
    for bin in before after; do
      n=$((n + 1)); d="$SCRATCH/p$n"
      mkdir "$d" && git -C "$d" init -q -b main && git -C "$d" commit -q --allow-empty -m base
      echo; echo "## $bin: init, $label"
      (cd "$d" && CLAUDE_CONFIG_DIR="$claude" CODEX_HOME="$codex" "$SCRATCH/$bin" init); echo "# exit $?"
    done
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
