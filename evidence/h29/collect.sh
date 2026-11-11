#!/usr/bin/env bash
# Collects the H29 evidence. Run from the root of a checkout of the commit
# under review:
#   PATTERNS=<the name patterns, kept outside the repository> bash evidence/h29/collect.sh
# BASE is what H29 builds on: main after the v0.1.3 release.
set -u
OUT=${OUT:-evidence/h29}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-70630c4}
REPO=$(pwd -P)
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"; kill $(jobs -p) 2>/dev/null' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
# The CLIs' user settings and the home folder are scratch folders, never the
# operator's.
export HOME="$SCRATCH/home" CLAUDE_CONFIG_DIR="$SCRATCH/home/.claude" CODEX_HOME="$SCRATCH/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
go build -o "$SCRATCH/handoff" ./cmd/handoff
H="$SCRATCH/handoff"
clean() { sed "s|$SCRATCH/handoff|handoff|g; s|$SCRATCH|<scratch>|g"; }
now() { perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
project() {
  local p="$SCRATCH/$1"; mkdir "$p"
  git -C "$p" init -q -b main && git -C "$p" commit -q --allow-empty -m base
  (cd "$p" && "$H" init > /dev/null)
  echo "$p"
}
# atTerminal runs handoff under script(1), which gives it a terminal, as the
# operator's own terminal does. The terminal echoes the end of script's input
# as ^D, which the log leaves out.
atTerminal() {
  script -q /dev/null "$H" "$@" < /dev/null > "$SCRATCH/terminal.out" 2>&1; local c=$?
  tr -d '\r' < "$SCRATCH/terminal.out" | sed 's/^\^D//'
  return $c
}

# 1. A tell, at a terminal, in a project with init's three roles: the
# implementer listens with a plain wait, the reviewer with the hook its CLI


# 1. Every command the README names is one handoff has, and its quick start
# works as written, in a scratch project with a scratch home folder.
{
  header "the README's commands against handoff --help; its quick start in a scratch project"
  echo "## each command in the README's table, and whether the usage lists it"
  "$H" --help > "$SCRATCH/usage.txt" 2>&1
  for c in $(grep -E '^\| `' README.md | cut -d'|' -f2 | grep -oE '`[a-z]+' | tr -d '`' | grep -v '^handoff$' | sort -u); do
    if grep -qE "^  $c( |$)" "$SCRATCH/usage.txt"; then echo "$c: listed"; else echo "$c: NOT LISTED"; fi
  done
  echo; echo "## \$ handoff settings claude --global (its first lines)"
  "$H" settings claude --global 2>&1 | head -4
  echo; echo "## \$ handoff settings codex --global (its first lines)"
  "$H" settings codex --global 2>&1 | head -4
  p=$(project quick)
  cd "$p"
  echo; echo "## \$ handoff init, in a fresh project (made here by the collector)"
  rm -rf .handoff; "$H" init; echo "# exit $?"
  echo "## git status --short --ignored: $(git status --short --ignored | tr '\n' ' ')"
  echo "## .handoff/config.json"; cat .handoff/config.json
  sleep 300 & a=$!; sleep 300 & b=$!
  echo; echo "## a Claude Code session told \"you're the implementer\" runs: handoff role implementer"
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$a "$H" role implementer; echo "# exit $?"
  CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$b "$H" role reviewer > /dev/null
  echo; echo "## \$ handoff brief, in the implementer's session (first line)"
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$a "$H" brief | head -1
  echo; echo "## \$ handoff fyi \"Carry on while I'm away.\", at a terminal"
  atTerminal fyi "Carry on while I'm away."; echo "# exit $?"
  echo; echo "## \$ handoff roles add reviewer-3"
  "$H" roles add reviewer-3; echo "# exit $?"
  echo; echo "## \$ handoff (with nothing waiting)"
  "$H" < /dev/null; echo "# exit $?"
  echo; echo "## \$ handoff version"
  "$H" version
  kill $a $b; wait $a $b 2> /dev/null
  cd - > /dev/null
  p=$(project roles)
  cd "$p"
  echo; echo "## \$ handoff init --roles a,b, in a fresh project"
  rm -rf .handoff; "$H" init --roles a,b; echo "# exit $?"
  cd - > /dev/null
} 2>&1 | clean > "$OUT/readme.log"

# 2. What CLAUDE.md and the README lose. Not the removed lines themselves:
# this log is public, and they are what the operator wanted out (row 97).
{
  header "the lengths of CLAUDE.md and the README before and after, and what CLAUDE.md drops, in row 95's words"
  echo "## git diff --numstat $BASE -- CLAUDE.md README.md (added, removed, file)"
  git diff --numstat "$BASE" -- CLAUDE.md README.md
  echo "## CLAUDE.md drops: the decisions of 2026-10-09 (the models, the dates, D1 to D13 standing), the lineage, the trial plan, and the sentence on Source and Inferred reasoning; the rest of its rules stay"
  echo "## lines of CLAUDE.md here that are not at $BASE, word for word: $(git show "$BASE":CLAUDE.md | grep -vxF -f - CLAUDE.md | grep -c .)"
  echo; echo "## README.md lines: $(git show "$BASE":README.md | wc -l | tr -d ' ') at $BASE, $(wc -l < README.md | tr -d ' ') here"
  echo "## CLAUDE.md lines: $(git show "$BASE":CLAUDE.md | wc -l | tr -d ' ') at $BASE, $(wc -l < CLAUDE.md | tr -d ' ') here"
  echo "## files changed since $BASE:"; git diff --stat "$BASE" | sed 's/^ *//'
} > "$OUT/docs.log" 2>&1

# 3. The name sweep, the base against this commit. The patterns, and the
# lines they match, stay outside the repository.
{
  header "the name sweep of every tracked file, $BASE against HEAD"
  if [ -n "${PATTERNS:-}" ]; then
    python3 -I "$REPO/evidence/h29/sweep.py" "$PATTERNS" "$REPO" "$BASE" HEAD "$SCRATCH/matches.tsv"
  else
    echo "PATTERNS not given; no sweep"
  fi
} > "$OUT/sweep.log" 2>&1

# 4. Every test, under the race detector.
{
  header "go test -race -count=1 ./..."
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$SCRATCH/all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|panic: test timed out)|^--- FAIL' "$SCRATCH/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$SCRATCH/all.out"); failed: $(grep -c '^--- FAIL' "$SCRATCH/all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the notify test"
  grep -E '^--- (PASS|FAIL): TestEachAlertIsSentOnce' "$SCRATCH/all.out"
} 2>&1 | clean > "$OUT/go-test-race.log"

# 5. No home path or email address added; no dependencies, vet and gofmt.
{
  header "added lines with a home path or an email address; go.mod requires; go list -deps for net; go vet ./...; gofmt -l cmd internal"
  echo "## added lines in git diff $BASE matching /Users/, /home/ or an email address: $(git diff "$BASE" | grep '^+' | grep -v '^+++' | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## the same in evidence/h29's other logs as collected: $(cat $(ls "$OUT"/*.log | grep -v '/deps-vet-fmt.log$') | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  git diff "$BASE" -- CLAUDE.md | grep '^-' | grep -v '^---' | sed 's/^-//' | grep -v '^[[:space:]]*$' > "$SCRATCH/removed.txt"
  found=0; while IFS= read -r l; do cat "$OUT"/* | grep -qF -- "$l" && found=$((found + 1)); done < "$SCRATCH/removed.txt"
  echo "## lines CLAUDE.md removes since $BASE, not blank: $(wc -l < "$SCRATCH/removed.txt" | tr -d ' '); of those, found word for word in any file of evidence/h29 as collected: $found"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## net packages in go list -deps ./...: $(go list -deps ./... | grep -cE '^net(/|$)')"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} > "$OUT/deps-vet-fmt.log" 2>&1
