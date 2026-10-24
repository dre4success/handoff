#!/usr/bin/env bash
# Collects the H10a evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h10/collect.sh
# Notifications go to a file, never to the operator's screen. live-claude.sh, which
# starts a real throwaway Claude Code session, is run separately.
set -u
OUT=${OUT:-evidence/h10}
COMMIT=$(git rev-parse HEAD)
BASE=$(git merge-base HEAD main)
ROUND1=${ROUND1:-ebc04f2} # the commit review round 1 looked at
ROUND2=${ROUND2:-d320393} # and round 2
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P) # the real path, as handoff names the record
trap 'git worktree remove --force "$SCRATCH/main" 2>/dev/null; git worktree remove --force "$SCRATCH/round1" 2>/dev/null; git worktree remove --force "$SCRATCH/round2" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
unset HANDOFF_ROLE
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/main" "$BASE"
(cd "$SCRATCH/main" && go build -o "$SCRATCH/handoff-main" ./cmd/handoff) || exit 1
project() { # $1 = directory: a git repository with a record, roles claude and codex
  mkdir -p "$1" && (cd "$1" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
    "$SCRATCH/handoff" init --roles claude,codex --reviewers claude,codex --main main >/dev/null)
  echo "please review" > "$1/req.md"
}

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. Main against this commit: the new tests, then two waits for one role.
RUN='TestH10'
two_waits() { # $1 = binary, $2 = project
  local H=$1 R=$2
  (
    cd "$R"
    "$H" wait --for claude --timeout 20s > "$R/w1.out" 2> "$R/w1.err" &
    sleep 1
    "$H" wait --for claude --timeout 20s > "$R/w2.out" 2> "$R/w2.err" &
    W2=$!
    sleep 1
    "$H" post --as codex --item T1 --to claude --commit HEAD --title "T1" --body req.md
    wait
    echo "first wait printed:  $(cat "$R/w1.out" "$R/w1.err")"
    echo "second wait printed: $(cat "$R/w2.out" "$R/w2.err")"
  )
}
{
  header "the H10a tests on $BASE (main) with this commit's tests copied in, then here; then two waits for one role through each binary"
  cp cmd/handoff/hook_test.go "$SCRATCH/main/cmd/handoff/"
  echo "## the tests at $BASE (expected not to build: presence, brief and the hooks aren't there)"
  (cd "$SCRATCH/main" && go test -count=1 -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL)|undefined|unknown field' | head -8)
  echo
  echo "## the tests at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
  echo
  echo "## two waits for claude, then codex posts to claude, through main's binary: both deliver it"
  project "$SCRATCH/on-main"; two_waits "$SCRATCH/handoff-main" "$SCRATCH/on-main"
  echo
  echo "## the same through this commit's binary: the second exits 4 at once; the first delivers"
  project "$SCRATCH/here"; two_waits "$SCRATCH/handoff" "$SCRATCH/here"
} > "$OUT/regressions.log" 2>&1

# 2b. Round 2: the tests for the review's findings, on round 1's commit, then here.
RUN2='TestH10ResumedOrphanLeavesTheEntry|TestH10APathWithSpaces|TestH10SettingsInALinkedWorktree|TestH10SettingsClaude'
{
  header "the round-2 tests on $ROUND1 (round 1) with round 2's tests ($ROUND2) copied in, then here"
  git worktree add -q --detach "$SCRATCH/round1" "$ROUND1"
  git show "$ROUND2:cmd/handoff/hook_test.go" > "$SCRATCH/round1/cmd/handoff/hook_test.go"
  echo "## at $ROUND1 (expected to fail: the orphan delivers, the path isn't quoted, the rules name .handoff/ relatively)"
  (cd "$SCRATCH/round1" && go test -count=1 -v -run "$RUN2" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | sed "s|$SCRATCH|<scratch>|g")
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN2" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/round2.log" 2>&1

# 2c. Round 3: the test for round 2's finding, on round 2's commit, then here.
RUN3='TestH10BriefCommandsRun'
{
  header "the round-3 test on $ROUND2 (round 2) with this commit's tests copied in, then here"
  git worktree add -q --detach "$SCRATCH/round2" "$ROUND2"
  cp cmd/handoff/hook_test.go "$SCRATCH/round2/cmd/handoff/"
  echo "## at $ROUND2 (expected to fail: the question to the operator names no item, and note refuses it)"
  (cd "$SCRATCH/round2" && go test -count=1 -v -run "$RUN3" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:|--item' | sed -e "s|$SCRATCH|<scratch>|g" -e 's|/[^ `]*/handoff-bin-[0-9]*/handoff|<test binary>|g')
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN3" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/round3.log" 2>&1

# 3. The pieces a session meets, at the command line.
{
  header "settings claude, brief, hook listen and the hooks off duty, in a scratch project"
  R="$SCRATCH/demo"; project "$R"; cd "$R"
  echo '$ handoff settings claude --as claude'
  "$SCRATCH/handoff" settings claude --as claude 2>&1 | sed "s|$SCRATCH|<scratch>|g"
  echo
  echo '$ HANDOFF_ROLE=claude handoff hook brief   (what SessionStart shows the model)'
  HANDOFF_ROLE=claude "$SCRATCH/handoff" hook brief </dev/null 2>&1 | sed "s|$SCRATCH|<scratch>|g"
  echo
  echo '$ HANDOFF_ROLE=claude handoff hook listen &   ;  handoff post --as codex --to claude ...'
  echo '{"hook_event_name":"Stop"}' | HANDOFF_ROLE=claude "$SCRATCH/handoff" hook listen --timeout 30s > "$SCRATCH/l.out" 2> "$SCRATCH/l.err" &
  L=$!
  sleep 1
  echo "presence: $(cat .handoff/presence/claude.json)" | sed -E 's/"pid":[0-9]+/"pid":<pid>/'
  "$SCRATCH/handoff" post --as codex --item T1 --to claude --commit HEAD --title "T1: please review" --body req.md
  wait $L; echo "# hook listen exit: $? (2 wakes the session; what follows is what the session is shown)"
  sed "s|$SCRATCH|<scratch>|g" "$SCRATCH/l.err"
  echo
  echo '$ hooks with no role, and outside a project   (each: exit 0, nothing printed)'
  for h in listen brief notify; do printf '%s: ' "$h"; echo '{}' | "$SCRATCH/handoff" hook "$h" 2>&1; echo "exit $?"; done
  for h in listen brief; do printf '%s outside a project: ' "$h"; (cd "$SCRATCH" && echo '{}' | HANDOFF_ROLE=claude "$SCRATCH/handoff" hook "$h" 2>&1); echo "exit $?"; done
  cd - >/dev/null
} > "$OUT/demo.txt" 2>&1

# 4. Repeats, and vet, format and dependencies.
{
  header "go test -count=5 -v -run '$RUN' ./cmd/handoff"
  go test -count=5 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/repeat.log"
{
  header "go vet ./... ; gofmt -l cmd internal ; go.mod ; go list -deps (non-standard) ; go list -deps | grep net"
  echo '$ go vet ./...'; go vet ./... 2>&1; echo "# exit: $?"
  echo '$ gofmt -l cmd internal   (no output = formatted)'; gofmt -l cmd internal
  echo '$ cat go.mod'; cat go.mod
  echo '$ go list -deps -f "{{if not .Standard}}{{.ImportPath}}{{end}}" ./...'
  go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d'
  echo '$ go list -deps ./... | grep -E "^net(/|$)"   (no output = no net package)'
  go list -deps ./... | grep -E '^net(/|$)'; echo "# grep exit: $? (1 = no match)"
} > "$OUT/deps-vet-fmt.log"
echo "collected in $OUT at $COMMIT"
