#!/usr/bin/env bash
# Collects the H28 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h28/collect.sh
# BASE is what H28 builds on: H29 as the reviewer approved it.
set -u
OUT=${OUT:-evidence/h28}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-b8e1c7e}
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

alerts() { echo "## alerts sent so far: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"; sed 's/^/  /' "$SCRATCH/alerts.txt" 2>/dev/null; }
body() { echo "$1" > "$SCRATCH/body.md"; echo "$SCRATCH/body.md"; }
id() { sed -E 's/^posted #([0-9]+).*/\1/;s/^replied #([0-9]+).*/\1/' | head -1; }
# answer runs handoff at a terminal, under script(1), typing one line.
answer() {
  local line=$1; shift
  ( sleep 1; printf '%s\n' "$line"; sleep 1 ) | script -q /dev/null "$H" "$@" > "$SCRATCH/terminal.out" 2>&1; local c=$?
  tr -d '\r' < "$SCRATCH/terminal.out" | sed 's/^\^D//'
  return $c
}

# 1. Away with a go-ahead, two reviewers, both approve at one commit: no
# alert, the implementer's listener brings the APPROVE saying it is approved
# under the go-ahead. Then back, and an approval after it waits for the
# operator as before.
{
  header "handoff away, two reviewers approving one commit, the implementer's hook listen, show, handoff, brief, back, and an approval after back"
  p=$(project away)
  cd "$p"
  git switch -q -c w1 && git commit -q --allow-empty -m A && A=$(git rev-parse HEAD) && git commit -q --allow-empty -m B && B=$(git rev-parse HEAD)
  sleep 600 & i=$!; sleep 600 & r1=$!; sleep 600 & r2=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" role implementer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$r1 "$H" role reviewer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev-2 CLAUDE_PID=$r2 "$H" role reviewer-2 > /dev/null
  echo "## \$ handoff away \"Merge when the reviewers approve.\""
  atTerminal away "Merge when the reviewers approve."; echo "# exit $?"
  alerts
  echo; echo "## the record so far"
  sed -E 's/"prev":"[0-9a-f]{64}"/"prev":"<hash>"/' .handoff/channel.jsonl
  echo; echo "## the implementer posts W1 at A to reviewer and reviewer-2"
  one=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$(body 'please review')" 2>&1 | tee /dev/stderr | id)
  two=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer-2 --commit "$A" --title "W1 work" --body "$(body 'please review')" 2>&1 | tee /dev/stderr | id)
  echo; echo "## reviewer approves #$one"
  HANDOFF_ROLE=reviewer "$H" reply "$one" --verdict APPROVE --commit "$A" --body "$(body ok)"
  alerts
  echo; echo "## reviewer-2 approves #$two"
  last=$(HANDOFF_ROLE=reviewer-2 "$H" reply "$two" --verdict APPROVE --commit "$A" --body "$(body ok)" 2>&1 | tee /dev/stderr | id)
  alerts
  echo; echo "## the implementer's hook listen, as at the end of its turn"
  echo '{"hook_event_name":"Stop","session_id":"s-impl"}' | CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" hook listen --timeout 2s 2>&1; echo "# exit $? (2 wakes the session)"
  echo; echo "## handoff show $last"
  "$H" show "$last"
  echo; echo "## handoff, in the operator's terminal"
  atTerminal; echo "# exit $?"
  echo; echo "## handoff decide W1 --approve"
  atTerminal decide W1 --approve; echo "# exit $?"
  echo; echo "## the implementer's brief: its go-ahead lines"
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" brief | grep -E 'away|go-ahead'
  echo; echo "## \$ handoff back"
  atTerminal back; echo "# exit $?"
  echo; echo "## after back: W2 posted and approved"
  w2=$(HANDOFF_ROLE=implementer "$H" post --item W2 --to reviewer --commit "$B" --title "W2 work" --body "$(body 'please review')" 2>&1 | tee /dev/stderr | id)
  HANDOFF_ROLE=reviewer "$H" reply "$w2" --verdict APPROVE --commit "$B" --body "$(body ok)"
  alerts
  echo; echo "## \$ handoff back, again"
  atTerminal back; echo "# exit $?"
  kill $i $r1 $r2; wait $i $r1 $r2 2> /dev/null
  cd - > /dev/null
} 2>&1 | clean > "$OUT/away.log"

# 2. Approvals at two commits: CHANGES from reviewer-2, a fix sent to it
# alone; the go-ahead covers neither, and the operator is told why.
{
  header "away; reviewer approves A, reviewer-2 asks for changes, approves the fix at B: not covered"
  rm -f "$SCRATCH/alerts.txt"
  p=$(project commits)
  cd "$p"
  git switch -q -c w1 && git commit -q --allow-empty -m A && A=$(git rev-parse HEAD) && git commit -q --allow-empty -m B && B=$(git rev-parse HEAD)
  atTerminal away "Merge when approved."; echo "# exit $?"
  one=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$(body r)" 2>&1 | id)
  two=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer-2 --commit "$A" --title "W1 work" --body "$(body r)" 2>&1 | id)
  echo "## reviewer approves #$one at A"; HANDOFF_ROLE=reviewer "$H" reply "$one" --verdict APPROVE --commit "$A" --body "$(body ok)"; alerts
  echo "## reviewer-2 asks for changes on #$two"; HANDOFF_ROLE=reviewer-2 "$H" reply "$two" --verdict CHANGES --commit "$A" --body "$(body 'fix it')"; alerts
  three=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer-2 --commit "$B" --title "W1 fixed" --supersedes "$two" --body "$(body fixed)" 2>&1 | id)
  echo "## the fix, to reviewer-2 alone at B (#$three); reviewer-2 approves it"; HANDOFF_ROLE=reviewer-2 "$H" reply "$three" --verdict APPROVE --commit "$B" --body "$(body ok)"; alerts
  echo; echo "## handoff, in the operator's terminal (no answer given)"
  "$H" < /dev/null; echo "# exit $?"
  cd - > /dev/null
} 2>&1 | clean > "$OUT/commits.log"

# 3. Away when something is approved already: away names it and asks. No
# writes nothing; yes approves it.
{
  header "handoff away with W1 approved already: answered n, then y"
  rm -f "$SCRATCH/alerts.txt"
  p=$(project confirm)
  cd "$p"
  git switch -q -c w1 && git commit -q --allow-empty -m A && A=$(git rev-parse HEAD)
  one=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$(body r)" 2>&1 | id)
  HANDOFF_ROLE=reviewer "$H" reply "$one" --verdict APPROVE --commit "$A" --body "$(body ok)" > /dev/null
  echo "## lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo "## answered n"; answer n away "Merge when approved."; echo "# exit $?; lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo "## answered y"; answer y away "Merge when approved."; echo "# exit $?; lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  alerts
  echo; echo "## handoff, in the operator's terminal"
  "$H" < /dev/null
  cd - > /dev/null

  echo; echo "## a fresh project: the implementer has a session and has had W1's APPROVE; then away approves W1, and the listener brings the fyi naming it"
  rm -f "$SCRATCH/alerts.txt"
  p=$(project confirm2)
  cd "$p"
  git switch -q -c w1 && git commit -q --allow-empty -m A && A=$(git rev-parse HEAD)
  sleep 300 & i=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" role implementer > /dev/null
  one=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$(body r)" 2>&1 | id)
  HANDOFF_ROLE=reviewer "$H" reply "$one" --verdict APPROVE --commit "$A" --body "$(body ok)" > /dev/null 2>&1
  echo '{"hook_event_name":"Stop","session_id":"s-impl"}' | CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" hook listen --timeout 2s 2>&1; echo "# exit $?"
  answer y away "Merge when approved."; echo "# exit $?"
  alerts
  echo '{"hook_event_name":"Stop","session_id":"s-impl"}' | CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" hook listen --timeout 2s 2>&1; echo "# exit $?"
  echo; echo "## the implementer's brief: its first lines"
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" brief | head -2
  kill $i; wait $i 2> /dev/null
  cd - > /dev/null
} 2>&1 | clean > "$OUT/confirm.log"

# 3b. The one rule, whatever the words: both reviewers in session, an item
# sent to one, under "Merge when both reviewers approve." (the reviewer's
# K1). away says the rule; one approval approves; the notification names
# the one reviewer.
{
  header "away \"Merge when both reviewers approve.\" with both reviewers in session; W1 sent to reviewer alone"
  rm -f "$SCRATCH/alerts.txt"
  p=$(project one)
  cd "$p"
  git switch -q -c w1 && git commit -q --allow-empty -m A && A=$(git rev-parse HEAD)
  sleep 600 & i=$!; sleep 600 & r1=$!; sleep 600 & r2=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$i "$H" role implementer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$r1 "$H" role reviewer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev-2 CLAUDE_PID=$r2 "$H" role reviewer-2 > /dev/null
  atTerminal away "Merge when both reviewers approve."; echo "# exit $?"
  one=$(HANDOFF_ROLE=implementer "$H" post --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$(body r)" 2>&1 | tee /dev/stderr | id)
  HANDOFF_ROLE=reviewer "$H" reply "$one" --verdict APPROVE --commit "$A" --body "$(body ok)"
  alerts
  echo; echo "## \$ handoff back"
  atTerminal back; echo "# exit $?"
  kill $i $r1 $r2; wait $i $r1 $r2 2> /dev/null
  cd - > /dev/null
} 2>&1 | clean > "$OUT/one.log"

# 4. What away and back refuse; a refusal writes nothing; the usage.
{
  header "handoff away's and back's refusals; the usage"
  p=$(project refusals)
  cd "$p"
  sleep 300 & s=$!
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$s "$H" role implementer > /dev/null
  run() { echo; echo "## $1"; shift; "$@" < /dev/null; echo "# exit $?"; }
  run "away, no terminal" "$H" away "Merge when approved."
  run "away, an agent's role" env HANDOFF_ROLE=implementer "$H" away "Merge when approved."
  run "away, in the implementer's session" env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$s "$H" away "Merge when approved."
  run "away, no message" "$H" away
  run "away, a flag" "$H" away --as operator "Merge."
  run "back, no terminal" "$H" back
  run "back, in the implementer's session" env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$s "$H" back
  run "back, with words" "$H" back now
  echo; echo "## back at a terminal, not away"; atTerminal back; echo "# exit $?"
  echo; echo "## lines in the record after these: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
  echo; echo "## handoff --help: the lines for away and back"
  "$H" --help 2>&1 | grep -n -E 'away|back' 
  kill $s; wait $s 2> /dev/null
  cd - > /dev/null
} 2>&1 | clean > "$OUT/refusals.log"

# 5. Every test, under the race detector; the H28 tests ten times.
{
  header "go test -race -count=1 ./...; then the H28 tests, -count=10"
  start=$(date +%s)
  go test -race -count=1 -v ./... > "$SCRATCH/all.out" 2>&1; code=$?
  grep -E '^(ok|FAIL|panic: test timed out)|^--- FAIL' "$SCRATCH/all.out"
  echo "# passed: $(grep -c '^--- PASS' "$SCRATCH/all.out"); failed: $(grep -c '^--- FAIL' "$SCRATCH/all.out"); exit $code; wall time $(( $(date +%s) - start )) s"
  echo; echo "## the H28 tests in that run"
  grep -E '^--- (PASS|FAIL): TestH28' "$SCRATCH/all.out"
  echo; echo "## the H28 tests, 10 times each under the race detector"
  go test -race -count=10 -run '^TestH28' ./cmd/handoff ./internal/status 2>&1 | grep -E '^(ok|FAIL|---)'
} 2>&1 | clean > "$OUT/go-test-race.log"

# 6. No home path or email address added; no dependencies, vet and gofmt.
{
  header "added lines with a home path or an email address; go.mod requires; go list -deps for net; go vet ./...; gofmt -l cmd internal"
  echo "## added lines in git diff $BASE matching /Users/, /home/ or an email address: $(git diff "$BASE" | grep '^+' | grep -v '^+++' | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## the same in evidence/h28's other logs as collected: $(cat $(ls "$OUT"/*.log | grep -v '/deps-vet-fmt.log$') | grep -cE '/Users/|/home/|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}')"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## net packages in go list -deps ./...: $(go list -deps ./... | grep -cE '^net(/|$)')"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1
