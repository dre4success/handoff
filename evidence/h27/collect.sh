#!/usr/bin/env bash
# Collects the H27 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h27/collect.sh
# BASE is what H27 builds on: main after H26.
set -u
OUT=${OUT:-evidence/h27}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-3119db5}
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

# 1. An fyi, at a terminal, in a project with init's three roles: the
# implementer listens with a plain wait, the reviewer with the hook its CLI
# runs when a turn ends, and reviewer-2 has no session. Both listeners wake
# with it; nothing waits for anyone after; no alert is sent.
{
  header "handoff fyi at a terminal (script(1)), with wait --for implementer and hook listen for reviewer running, reviewer-2 with no session"
  p=$(project fyi)
  cd "$p"
  "$H" wait --for implementer --timeout 60s > "$SCRATCH/impl.out" 2> "$SCRATCH/impl.err" &
  impl=$!
  echo '{"hook_event_name":"Stop"}' | HANDOFF_ROLE=reviewer "$H" hook listen --timeout 60s > "$SCRATCH/rev.out" 2> "$SCRATCH/rev.err" &
  rev=$!
  sleep 2
  echo "## \$ handoff fyi \"Carry on while I'm away.\""
  atTerminal fyi "Carry on while I'm away."
  echo "# exit $?"
  wait $impl; ci=$?
  wait $rev; cr=$?
  echo
  echo "## the implementer's wait: exit $ci"
  cat "$SCRATCH/impl.out" "$SCRATCH/impl.err"
  echo "## the reviewer's hook listen: exit $cr (2 wakes the session)"
  cat "$SCRATCH/rev.out" "$SCRATCH/rev.err"
  echo
  echo "## handoff show 1"
  "$H" show 1
  echo
  echo "## the record's lines"
  sed -E 's/"prev":"[0-9a-f]{12}[0-9a-f]*"/"prev":"<hash>"/' .handoff/channel.jsonl
  echo
  for role in implementer reviewer reviewer-2 operator; do
    HANDOFF_ROLE=$role "$H" inbox --quiet; echo "## inbox --quiet for $role: exit $? (0: nothing waits)"
  done
  echo "## alerts sent: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
  echo
  echo "## the brief's line for an fyi (brief --for reviewer)"
  "$H" brief --for reviewer | grep -- '- An fyi'
  cd - > /dev/null
} 2>&1 | clean > "$OUT/fyi.log"

# 2. What fyi refuses, and that a refusal writes nothing; tell is gone; the
# usage.
{
  header "handoff fyi's refusals; handoff tell; the usage"
  p=$(project refusals)
  cd "$p"
  sleep 300 & cli=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$cli "$H" role implementer > /dev/null
  run() { echo; echo "## $1"; shift; "$@" < /dev/null; echo "# exit $?"; }
  run "no terminal" "$H" fyi "Carry on."
  run "no terminal, HANDOFF_ROLE=operator" env HANDOFF_ROLE=operator "$H" fyi "Carry on."
  run "an agent's role" env HANDOFF_ROLE=implementer "$H" fyi "Carry on."
  run "in the implementer's session" env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$cli "$H" fyi "Carry on."
  run "in a Codex thread with no role" env CODEX_THREAD_ID=t-new "$H" fyi "Carry on."
  run "no message" "$H" fyi
  run "a blank message" "$H" fyi "  "
  run "a flag" "$H" fyi --as operator "Carry on."
  echo; echo "## handoff tell, at a terminal (first line of its output; the usage follows)"
  atTerminal tell "Carry on." | head -1
  echo; echo "## lines in the record after these: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo; echo "## at a terminal, with the implementer's session: the fyi goes through"
  atTerminal fyi "Carry on."
  echo "# exit $?"
  kill $cli; wait $cli 2> /dev/null
  echo; echo "## at a terminal, once that session's process has ended: no session"
  atTerminal fyi "Carry on."
  echo "# exit $?; lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo; echo "## handoff --help: the lines that name fyi or tell"
  "$H" --help 2>&1 | grep -n -E 'fyi|tell'
  cd - > /dev/null
} 2>&1 | clean > "$OUT/refusals.log"

# 3. tell as the command or the kind is nowhere left, and the new test fails
# on main, where tell is a command.
{
  header "git grep for tell as the command or the kind; TestH27TellIsNowFyi on $BASE"
  echo "## files changed since $BASE:"
  git diff -M --stat "$BASE" | sed 's/^ *//'
  echo; echo "## git grep -n -E '\"tell\"|cmdTell|handoff tell|[Aa] tell\\b|\`tell' -- cmd internal README.md docs/prompts"
  git grep -n -E '"tell"|cmdTell|handoff tell|[Aa] tell\b|`tell' -- cmd internal README.md docs/prompts
  echo "# (the one match is the H27 test, which runs tell to see it refused)"
  echo; echo "## TestH27TellIsNowFyi against $BASE, where tell is a command"
  old="$SCRATCH/base"
  git worktree add -q --detach "$old" "$BASE"
  { printf 'package main\n\nimport (\n\t"strings"\n\t"testing"\n)\n'; sed -n '/^\/\/ H27 (row 94)/,$p' cmd/handoff/fyi_test.go; } > "$old/cmd/handoff/h27_test.go"
  (cd "$old" && go test -count=1 -run '^TestH27TellIsNowFyi$' ./cmd/handoff 2>&1 | grep -E '_test.go|^(ok|FAIL|---)')
  git worktree remove --force "$old"
  echo "## the same test at this commit"
  go test -count=1 -run '^TestH27TellIsNowFyi$' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|---)'
} 2>&1 | clean > "$OUT/leftovers.log"

# 4. Every test, under the race detector.
{
  header "go test -race -count=1 ./...; then the H26 and H27 tests, -count=10"
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$SCRATCH/all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|panic: test timed out)|^--- FAIL' "$SCRATCH/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$SCRATCH/all.out"); failed: $(grep -c '^--- FAIL' "$SCRATCH/all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the H26 and H27 tests in that run"
  grep -E '^--- (PASS|FAIL): Test(H26|H27|AppendAll)' "$SCRATCH/all.out"
  echo; echo "## the H26 and H27 tests, 10 times each under the race detector"
  go test -race -count=10 -run '^TestH2[67]' ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|---)'
  go test -race -count=10 -run '^TestAppendAll' ./internal/record 2>&1 | grep -E '^(ok|FAIL|---)'
} 2>&1 | clean > "$OUT/go-test-race.log"

# 5. No home path or email address added; no dependencies, vet and gofmt.
{
  header "added lines with a home path or an email address; go.mod requires; go list -deps for net; go vet ./...; gofmt -l cmd internal"
  echo "## added lines in git diff $BASE matching /Users/, /home/ or an email address: $(git diff "$BASE" | grep '^+' | grep -v '^+++' | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## the same in evidence/h27's other logs as collected: $(cat $(ls "$OUT"/*.log | grep -v '/deps-vet-fmt.log$') | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## net packages in go list -deps ./...: $(go list -deps ./... | grep -cE '^net(/|$)')"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} > "$OUT/deps-vet-fmt.log" 2>&1
