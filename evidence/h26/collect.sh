#!/usr/bin/env bash
# Collects the H26 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h26/collect.sh
# BASE is what H26 builds on: main after H25. Round 2 adds sessions.log,
# which runs round1.py.
set -u
OUT=${OUT:-evidence/h26}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-1af34c7}
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
# runs when a turn ends, and reviewer-2 has no session. Both listeners wake
# with it; nothing waits for anyone after; no alert is sent.
{
  header "handoff tell at a terminal (script(1)), with wait --for implementer and hook listen for reviewer running, reviewer-2 with no session"
  p=$(project tell)
  cd "$p"
  "$H" wait --for implementer --timeout 60s > "$SCRATCH/impl.out" 2> "$SCRATCH/impl.err" &
  impl=$!
  echo '{"hook_event_name":"Stop"}' | HANDOFF_ROLE=reviewer "$H" hook listen --timeout 60s > "$SCRATCH/rev.out" 2> "$SCRATCH/rev.err" &
  rev=$!
  sleep 2
  echo "## \$ handoff tell \"Carry on while I'm away. If both reviewers approve, merge.\""
  t0=$(now)
  atTerminal tell "Carry on while I'm away. If both reviewers approve, merge."
  echo "# exit $?"
  wait $impl; ci=$?; ti=$(now)
  wait $rev; cr=$?; tr=$(now)
  echo
  echo "## the implementer's wait: exit $ci, ended $(perl -e "printf '%.1f', $ti - $t0") s after the tell began"
  cat "$SCRATCH/impl.out" "$SCRATCH/impl.err"
  echo "## the reviewer's hook listen: exit $cr (2 wakes the session), ended $(perl -e "printf '%.1f', $tr - $t0") s after the tell began"
  cat "$SCRATCH/rev.out" "$SCRATCH/rev.err"
  echo
  echo "## handoff show 1"
  "$H" show 1
  echo
  echo "## the record's tell lines (channel.jsonl), and the chain"
  grep '"kind":"tell"' .handoff/channel.jsonl | sed -E 's/"prev":"[0-9a-f]{12}[0-9a-f]*"/"prev":"<hash>"/'
  echo
  for role in implementer reviewer reviewer-2 operator; do
    HANDOFF_ROLE=$role "$H" inbox --quiet; echo "## inbox --quiet for $role: exit $? (0: nothing waits)"
  done
  echo "## alerts sent: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
  echo
  echo "## the brief's line for a tell (brief --for reviewer)"
  "$H" brief --for reviewer | grep -- '- A tell'
  cd - > /dev/null
} 2>&1 | clean > "$OUT/tell.log"

# 2. What a tell refuses, and that a refusal writes nothing. Then a tell with
# no session at all, and one killed halfway through its write.
{
  header "handoff tell's refusals; with no session; killed mid-write (HANDOFF_TEST_FAULT=torn)"
  p=$(project refusals)
  cd "$p"
  sleep 300 & cli=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$cli "$H" role implementer > /dev/null
  run() { echo; echo "## $1"; shift; "$@" < /dev/null; echo "# exit $?"; }
  run "no terminal" "$H" tell "Carry on."
  run "no terminal, HANDOFF_ROLE=operator" env HANDOFF_ROLE=operator "$H" tell "Carry on."
  run "an agent's role" env HANDOFF_ROLE=implementer "$H" tell "Carry on."
  run "in the implementer's session" env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$cli "$H" tell "Carry on."
  run "in a Codex thread with no role" env CODEX_THREAD_ID=t-new "$H" tell "Carry on."
  run "no message" "$H" tell
  run "a blank message" "$H" tell "  "
  run "a flag" "$H" tell --as operator "Carry on."
  echo; echo "## lines in the record after these: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo; echo "## at a terminal, with the implementer's session: the tell goes through"
  atTerminal tell "Carry on."
  echo "# exit $?"
  kill $cli; wait $cli 2> /dev/null
  echo; echo "## at a terminal, once that session's process has ended: no session"
  atTerminal tell "Carry on."
  echo "# exit $?; lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  cd - > /dev/null

  p=$(project torn)
  cd "$p"
  sleep 300 & a=$!; sleep 300 & b=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$a "$H" role implementer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$b "$H" role reviewer > /dev/null
  echo; echo "## killed halfway through the write of two tells"
  export HANDOFF_TEST_FAULT=torn; atTerminal tell "Carry on."; c=$?; unset HANDOFF_TEST_FAULT
  echo "# exit $c (script(1) gives 9 for its child killed by SIGKILL); the record: $(wc -c < .handoff/channel.jsonl | tr -d ' ') bytes, $(wc -l < .handoff/channel.jsonl | tr -d ' ') whole lines, then the cut"
  echo "## its whole lines:"
  head -n "$(wc -l < .handoff/channel.jsonl)" .handoff/channel.jsonl | sed -E 's/"prev":"[0-9a-f]{64}"/"prev":"<hash>"/'
  echo "after" > "$SCRATCH/body.md"
  run "a note after it" env HANDOFF_ROLE=implementer "$H" note --item W1 --to reviewer --title after --body "$SCRATCH/body.md"
  run "repair" env HANDOFF_ROLE=reviewer "$H" repair
  echo "after repair" > "$SCRATCH/body.md"
  run "a note after repair" env HANDOFF_ROLE=implementer "$H" note --item W1 --to reviewer --title after --body "$SCRATCH/body.md"
  kill $a $b; wait $a $b 2> /dev/null
  cd - > /dev/null
} 2>&1 | clean > "$OUT/refusals.log"

# 2b. Round 2 (row 93): a tell is for the session told. Real hook listens,
# as Claude Code's Stop hook runs them, in sessions given the role with
# `handoff role`; a sleep stands in for each CLI's process.
{
  header "round 2: a later session never gets an old tell (the reviewer's P1a, P1b); the session told does"
  p=$(project sessions)
  cd "$p"
  listen() { echo "{\"hook_event_name\":\"Stop\",\"session_id\":\"$1\"}" | CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 "$H" hook listen --timeout 2s 2>&1; echo "# exit $?"; }
  role() { CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 "$H" role implementer > /dev/null; }
  sleep 300 & one=$!; role s-one $one
  echo "## s-one has the role; a tell"
  atTerminal tell "Merge when the reviewer approves."; echo "# exit $?"
  sed -E 's/"prev":"[0-9a-f]{64}"/"prev":"<hash>"/' .handoff/channel.jsonl | tail -1
  kill $one; wait $one 2> /dev/null
  sleep 300 & two=$!; role s-two $two
  echo; echo "## P1a: s-one's process ended before it read the tell; s-two takes the role and listens"
  listen s-two $two
  echo; echo "## P1b: a tell to s-two; the operator gives the role to s-three before s-two reads it"
  atTerminal tell "Carry on, two."; echo "# exit $?"
  sleep 300 & three=$!; role s-three $three
  echo "## s-three listens"
  listen s-three $three
  echo "## s-two listens, having lost the role"
  listen s-two $two
  echo; echo "## a tell to s-three, then a note for the role: s-three listens and gets both"
  atTerminal tell "Carry on, three."; echo "# exit $?"
  echo "a note" > "$SCRATCH/note.md"
  HANDOFF_ROLE=reviewer "$H" note --item W1 --to implementer --body "$SCRATCH/note.md"
  listen s-three $three
  echo; echo "## handoff show 3"
  "$H" show 3
  kill $two $three; wait $two $three 2> /dev/null
  cd - > /dev/null
  echo; echo "## TestH26ATellIsForTheSessionTold against round 1 (f3be27c), its Session check left out, as round 1 has no such field"
  old="$SCRATCH/round1"
  git worktree add -q --detach "$old" f3be27c
  python3 -I evidence/h26/round1.py cmd/handoff/tell_test.go "$old/cmd/handoff/round2_test.go"
  (cd "$old" && go test -count=1 -run '^TestH26ATellIsForTheSessionTold$' ./cmd/handoff 2>&1 | grep -E '_test.go|^(ok|FAIL|---)')
  git worktree remove --force "$old"
  echo "## the same test at this commit"
  go test -count=1 -run '^TestH26ATellIsForTheSessionTold$' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|---)'
} 2>&1 | clean > "$OUT/sessions.log"

# 3. Every test, under the race detector, with go test's default timeout.
{
  header "go test -race -count=1 ./... (no -timeout); then the H26 tests, -count=10"
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$SCRATCH/all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|panic: test timed out)|^--- FAIL' "$SCRATCH/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$SCRATCH/all.out"); failed: $(grep -c '^--- FAIL' "$SCRATCH/all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the H26 tests in that run"
  grep -E '^--- (PASS|FAIL): Test(H26|AppendAll)' "$SCRATCH/all.out"
  echo; echo "## the H26 tests, 10 times each under the race detector"
  go test -race -count=10 -run '^TestH26' ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|---)'
  go test -race -count=10 -run '^TestAppendAll' ./internal/record 2>&1 | grep -E '^(ok|FAIL|---)'
} 2>&1 | clean > "$OUT/go-test-race.log"

# 4. What the change adds names no home path or email address. The full
# sweep needs H14's patterns file, which stays outside the repository.
{
  header "git diff $BASE: added lines with a home path or an email address"
  echo "## files changed: $(git diff --name-only "$BASE" | wc -l | tr -d ' ')"
  echo "## added lines matching /Users/, /home/ or an email address: $(git diff "$BASE" | grep '^+' | grep -v '^+++' | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## the same in evidence/h26's other logs as collected: $(cat $(ls "$OUT"/*.log | grep -v '/names.log$') | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
} > "$OUT/names.log" 2>&1

# 5. No dependencies, vet and gofmt.
{
  header "go.mod requires; go list -deps for net; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## net packages in go list -deps ./...: $(go list -deps ./... | grep -cE '^net(/|$)')"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} > "$OUT/deps-vet-fmt.log" 2>&1
