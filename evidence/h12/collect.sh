#!/usr/bin/env bash
# Collects the H12 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h12/collect.sh
# BASE is what H12 builds on (main, after H11). Notifications go to a file.
# live-two-claude.sh, which starts two real throwaway Claude Code sessions, is
# run separately; probe-ids/ holds the probes made before the code.
set -u
OUT=${OUT:-evidence/h12}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
ROUND1=${ROUND1:-50a8c2d} # the commit review round 1 looked at
ROUND2=${ROUND2:-589c1cd} # the commit review round 2 looked at
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; git worktree remove --force "$SCRATCH/round1" 2>/dev/null; git worktree remove --force "$SCRATCH/round2" 2>/dev/null; rm -rf "$SCRATCH" /private/tmp/handoff-h12-startup-race-*' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
# The demo's git, like the tests', ignores the user's git configuration.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/base" "$BASE"

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 2. The H12 tests on main, then here.
RUN='TestH12|Settings'
{
  header "the H12 tests on $BASE (main) with this commit's tests copied in, then here"
  cp cmd/handoff/session_test.go cmd/handoff/decide_test.go cmd/handoff/main_test.go "$SCRATCH/base/cmd/handoff/"
  echo "## at $BASE (expected: the TestH12 tests fail, since there is no handoff role; the settings tests as main has them pass there)"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | cut -c1-220 | sed -e "s|$SCRATCH|<scratch>|g" -e 's|/[^ `]*/handoff-bin-[0-9]*/handoff|<test binary>|g')
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 2b. Round 2: the tests for round 1's findings, on round 1's commit, then here.
RUN2='TestH12ACodexThreadKeepsItsRoleAlone|TestH12AWaitStopsWhenItsRoleMoves|TestH12ARoleOutlivesClearAndResume'
{
  header "the round-2 tests on $ROUND1 (round 1) with this commit's tests copied in, then here"
  git worktree add -q --detach "$SCRATCH/round1" "$ROUND1"
  cp cmd/handoff/session_test.go "$SCRATCH/round1/cmd/handoff/"
  echo "## at $ROUND1 (expected to fail: a second Codex thread under the same parent, and a startup in the same Claude process, take the first one's role; a plain wait keeps waiting after its role moves)"
  (cd "$SCRATCH/round1" && go test -count=1 -v -run "$RUN2" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:|you are implementer|still waits' | cut -c1-220 | sed -e "s|$SCRATCH|<scratch>|g" -e 's|/[^ `]*/handoff-bin-[0-9]*/handoff|<test binary>|g')
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN2" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/round2.log" 2>&1

# 2c. Round 3: GPT-6's startup race, as it supplied it, on round 2's commit
# and here. It needs no test-only delay: 300 unrelated bindings widen the
# moment, and it tries up to 30 times. Exit 0 and "found: True" mean the
# displaced listener took the new holder's note.
{
  header "GPT-6's startup-race.py (evidence/reviewer/H12-589c1cd/) against $ROUND2 (round 2), then here"
  git worktree add -q --detach "$SCRATCH/round2" "$ROUND2"
  (cd "$SCRATCH/round2" && go build -o "$SCRATCH/handoff-round2" ./cmd/handoff)
  mkdir -p "$SCRATCH/rev/H12-589c1cd" "$SCRATCH/rev/H12-50a8c2d"
  # GPT-6's evidence is on main, as supplied, not on this branch.
  git show main:evidence/reviewer/H12-50a8c2d/probes.py > "$SCRATCH/rev/H12-50a8c2d/probes.py" &&
    git show main:evidence/reviewer/H12-589c1cd/startup-race.py | sed 's|^p.BINARY = .*|p.BINARY = sys.argv[2]|' > "$SCRATCH/rev/H12-589c1cd/startup-race.py" || exit 1
  echo "(read from main; its one change: the binary comes from the second argument)"
  for kind in hook wait; do
    for which in round2 here; do
      b="$SCRATCH/handoff"; [ "$which" = round2 ] && b="$SCRATCH/handoff-round2"
      echo; echo "## $kind, $which ($([ "$which" = round2 ] && echo "$ROUND2; expected: found: True" || echo "$COMMIT; expected: found: False"))"
      python3 -I "$SCRATCH/rev/H12-589c1cd/startup-race.py" "$kind" "$b" 2>&1 | grep -E '^(REPRODUCED|found:)'
      echo "# exit: ${PIPESTATUS[0]}"
    done
  done
} 2>&1 | sed -e "s|$SCRATCH|<scratch>|g" > "$OUT/round3.log"

# 3. What the operator and the sessions see: the settings with no role fixed, two
# stand-in sessions of one CLI taking roles, and a session with none.
{
  header "settings claude and codex with no --as; two Claude Code sessions (stood in for by their variables) take roles and work a round; a session with no role"
  R="$SCRATCH/demo"
  mkdir -p "$R" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
    echo hello > a.txt && git add a.txt && git -c user.name=t -c user.email=t@t commit -q -m "T1: add a.txt"
  "$SCRATCH/handoff" init --roles implementer,reviewer >/dev/null
  echo '$ handoff settings claude'; "$SCRATCH/handoff" settings claude 2>&1
  echo; echo '$ handoff settings codex'; "$SCRATCH/handoff" settings codex 2>&1
  sleep 300 & A=$!; sleep 300 & B=$!
  opus() { CLAUDE_CODE_SESSION_ID=s-opus CLAUDE_PID=$A "$SCRATCH/handoff" "$@"; }
  fable() { CLAUDE_CODE_SESSION_ID=s-fable CLAUDE_PID=$B "$SCRATCH/handoff" "$@"; }
  echo; echo '# session s-opus, told "you implement":'; echo '$ handoff role implementer'; opus role implementer 2>&1 | head -3
  echo; echo '# session s-fable, told "you review":'; echo '$ handoff role reviewer'; fable role reviewer 2>&1 | head -1
  echo; echo '$ cat .handoff/sessions/*.json'; cat .handoff/sessions/*.json | sed -E 's/"pid":[0-9]+/"pid":<the stand-in process>/'
  echo "Please review T1." > req.md
  echo; echo '# s-opus, with no --as:'; echo '$ handoff post --item T1 --to reviewer --commit HEAD --title "T1: add a.txt" --body req.md'; opus post --item T1 --to reviewer --commit HEAD --title "T1: add a.txt" --body req.md 2>&1
  echo; echo "# s-fable's Stop hook:"; echo '$ handoff hook listen --timeout 5s   (stdin: {"session_id":"s-fable","hook_event_name":"Stop"})'
  echo '{"session_id":"s-fable","hook_event_name":"Stop"}' | fable hook listen --timeout 5s; echo "# exit: $?"
  echo; echo '# s-fable tries the other role:'; echo '$ handoff claim --as implementer 1'; fable claim --as implementer 1 2>&1; echo "# exit: $?"
  echo; echo '# a third session starts with no role:'; echo '$ handoff hook brief   (stdin: {"session_id":"s-new","hook_event_name":"SessionStart"})'
  echo '{"session_id":"s-new","hook_event_name":"SessionStart"}' | CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$$ "$SCRATCH/handoff" hook brief 2>&1
  kill $A $B 2>/dev/null; wait $A $B 2>/dev/null
  cd - >/dev/null
} 2>&1 | sed -e "s|$SCRATCH|<scratch>|g" > "$OUT/demo.txt"

# 4. Vet, format, dependencies and whitespace.
{
  header "go vet ./... ; gofmt -l cmd internal ; go list -deps (non-standard) ; go list -deps | grep net ; git diff --check"
  echo '$ go vet ./...'; go vet ./... 2>&1; echo "# exit: $?"
  echo '$ gofmt -l cmd internal   (no output = formatted)'; gofmt -l cmd internal
  echo '$ go list -deps -f "{{if not .Standard}}{{.ImportPath}}{{end}}" ./...   (no output = none outside the standard library but this module)'
  go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d' | grep -v '^github.com/dre4success/handoff'
  echo '$ go list -deps ./... | grep -E "^net(/|$)"   (no output = no net package)'
  go list -deps ./... | grep -E '^net(/|$)'; echo "# grep exit: $? (1 = no match)"
  echo "\$ git diff --check $BASE HEAD   (no output = clean)"; git diff --check "$BASE" HEAD; echo "# exit: $?"
} > "$OUT/deps-vet-fmt.log" 2>&1
echo "collected in $OUT at $COMMIT"
