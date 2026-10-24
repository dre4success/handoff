#!/usr/bin/env bash
# Collects the H16 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h16/collect.sh
# BASE is what H16 builds on (main). Alerts and holds go to files. The demo
# stands in for CLI sessions as the tests do: by their ids, with a sleep for
# each Claude Code process.
set -u
OUT=${OUT:-evidence/h16}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
PIDS=()
trap 'kill "${PIDS[@]}" 2>/dev/null; git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
mkdir "$SCRATCH/bin" && go build -o "$SCRATCH/bin/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/base" "$BASE"
TESTS='TestH16|TestH11InitWithNoFlags|TestH11BriefCommandsRunForClaudeAndCodex|TestH12ACodexThreadKeepsItsRoleAlone'

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 2. The new and changed tests on main, then here.
{
  header "this commit's session_test.go, pickup_test.go and roles_test.go, on $BASE (main), then here: -run '$TESTS'"
  cp cmd/handoff/session_test.go cmd/handoff/pickup_test.go cmd/handoff/roles_test.go "$SCRATCH/base/cmd/handoff/"
  echo "## at $BASE (expected to fail: a session takes the role named after its CLI, settings pass --prefer, init makes claude and codex, and there is no roles command)"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$TESTS" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | cut -c1-300)
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$TESTS" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 3. A walk-through, with this commit's handoff on PATH.
export PATH="$SCRATCH/bin:$PATH"
project() { mkdir "$SCRATCH/$1" && cd "$SCRATCH/$1" && git init -q -b main && echo x > f && git add f && git commit -qm base; }
start() { echo "{\"session_id\":\"$1\",\"hook_event_name\":\"SessionStart\",\"source\":\"startup\"}"; }
show() { echo; echo "\$ $*"; "$@" 2>&1; echo "# exit: $?"; }
# The two Claude Code processes; kept off the log's pipe, so it closes.
sleep 300 > /dev/null 2>&1 & PIDS+=($!); OPUS=$!
sleep 300 > /dev/null 2>&1 & PIDS+=($!); FABLE=$!
{
  header "handoff init, settings, roles add and role, in scratch projects; sessions stood in for by their ids"
  echo "## A new project: init with no flags"
  (project new && show handoff init && show handoff settings codex)
  echo
  echo "## A project made with roles named after the agents, as the operator's was, and settings from before H16 that pass --prefer"
  (project old && handoff init --roles claude,codex > /dev/null &&
    echo "\$ handoff init --roles claude,codex" &&
    echo && echo "# Claude Code on Opus starts; its hook passes --prefer claude:" &&
    start s-opus | CLAUDE_CODE_SESSION_ID=s-opus CLAUDE_PID=$OPUS handoff hook brief --prefer claude &&
    echo "# Codex starts:" &&
    start t-codex | CODEX_THREAD_ID=t-codex handoff hook brief --prefer codex &&
    echo "# sessions with a role: [$(ls .handoff/sessions 2>/dev/null | tr '\n' ' ')]" &&
    show handoff roles add implementer reviewer,reviewer-2 &&
    echo && echo "# The operator gives each session its role:" &&
    echo "# in Claude Code on Opus:" && CLAUDE_CODE_SESSION_ID=s-opus CLAUDE_PID=$OPUS show handoff role implementer | sed -n '2,3p' &&
    echo "# in Claude Code on Fable:" && CLAUDE_CODE_SESSION_ID=s-fable CLAUDE_PID=$FABLE show handoff role reviewer | sed -n '2,3p' &&
    echo "# in Codex:" && CODEX_THREAD_ID=t-codex show handoff role reviewer-2 | sed -n '2,3p' &&
    echo "# sessions with a role: [$(ls .handoff/sessions | tr '\n' ' ')]" &&
    echo && echo "# Work flows to the new roles:" &&
    printf 'please review\n' > "$SCRATCH/req.md" &&
    echo "# in Claude Code on Opus:" && CLAUDE_CODE_SESSION_ID=s-opus CLAUDE_PID=$OPUS show handoff post --item W1 --to reviewer-2 --commit HEAD --title "W1 work" --body "$SCRATCH/req.md" &&
    echo "# in Codex:" && CODEX_THREAD_ID=t-codex show handoff inbox)
} 2>&1 | sed "s|$SCRATCH|<scratch>|g" > "$OUT/demo.log"

# 4. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1
