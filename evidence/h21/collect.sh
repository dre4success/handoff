#!/usr/bin/env bash
# Collects the H21 evidence, but for the live runs (live-codex.sh,
# live-claude.sh). Run from the root of a checkout of the commit under review:
#   bash evidence/h21/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H21 builds on: main after the v0.1.1 release.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h21}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-0f68be5}
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
# The CLIs' user settings are empty scratch folders, never the operator's.
export CLAUDE_CONFIG_DIR="$SCRATCH/claude-home" CODEX_HOME="$SCRATCH/codex-home"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME/rules"

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
newproject() { local d="$SCRATCH/$1"; mkdir "$d" && cd "$d" && git init -q -b main && git commit -q --allow-empty -m base; }

# 2. settings --global, init and the briefs' listening lines, before and after,
# with nothing set up; then after, once what --global prints is added.
listening() { # listening <bin> <label> <env...> -- <args...>
  local bin=$1 label=$2; shift 2
  local e=(); while [ "$1" != "--" ]; do e+=("$1"); shift; done; shift
  echo "## $bin: $label"
  env "${e[@]}" "$SCRATCH/$bin" "$@" | grep '^Listening:'
}
briefs() { # briefs <bin> <tag>
  listening $1 "brief, in a Claude Code session" CLAUDE_CODE_SESSION_ID=s-$2 CLAUDE_PID=$$ -- brief --for reviewer
  listening $1 "brief, in a Codex session" CODEX_THREAD_ID=t-$2 -- brief --for reviewer
  listening $1 "brief, outside a session" HANDOFF_ROLE=reviewer -- brief
  listening $1 "role, in a Claude Code session" CLAUDE_CODE_SESSION_ID=s-role-$2 CLAUDE_PID=$$ -- role implementer
  listening $1 "role, in a Codex session" CODEX_THREAD_ID=t-role-$2 -- role reviewer-2
  echo '{"source":"startup"}' | listening $1 "hook brief, as a hook gives it" HANDOFF_ROLE=reviewer -- hook brief
}
{
  header "settings --global, init and each brief's listening line: built from $BASE (before) and from here (after); nothing set up, then what --global prints added"
  cd "$SCRATCH" && for bin in before after; do
    for c in claude codex; do echo "## $bin: settings $c --global, outside any project"; "$SCRATCH/$bin" settings $c --global; echo "# exit $?"; done
  done
  "$SCRATCH/after" settings claude --global --as reviewer; echo "# exit $?"
  for bin in before after; do
    newproject p-$bin; echo "## $bin: init, nothing set up"; "$SCRATCH/$bin" init; echo "# exit $?"
    briefs $bin $bin
  done
  echo "## after: what settings --global prints, added as it says"
  out=$("$SCRATCH/after" settings claude --global); printf '%s\n' "$out" | sed -n '/^{/,$p' > "$CLAUDE_CONFIG_DIR/settings.json"
  out=$("$SCRATCH/after" settings codex --global)
  printf '%s\n' "$out" | sed -n "\\|^$CODEX_HOME/hooks.json:\$|,/^\$/p" | sed '1d;$d' > "$CODEX_HOME/hooks.json"
  printf '%s\n' "$out" | sed -n "\\|^$CODEX_HOME/rules/handoff.rules:\$|,\$p" | sed '1d' > "$CODEX_HOME/rules/handoff.rules"
  wc -l "$CLAUDE_CONFIG_DIR/settings.json" "$CODEX_HOME/hooks.json" "$CODEX_HOME/rules/handoff.rules"
  newproject p-set; echo "## after: init, both set up"; "$SCRATCH/after" init; echo "# exit $?"
  briefs after set
  echo "## after: Codex's own check of the rule: show, decide"
  codex execpolicy check --rules "$CODEX_HOME/rules/handoff.rules" "$SCRATCH/after" show 1 2>&1 | head -3
  codex execpolicy check --rules "$CODEX_HOME/rules/handoff.rules" "$SCRATCH/after" decide W1 --approve 2>&1 | head -3
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
