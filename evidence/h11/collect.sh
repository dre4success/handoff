#!/usr/bin/env bash
# Collects the H11 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h11/collect.sh
# BASE is what H11 builds on (main, after H10b). Notifications go to a file.
# No repeat runs: H11 changes no locking, waiting or concurrency.
set -u
OUT=${OUT:-evidence/h11}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
unset HANDOFF_ROLE
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

# 2. The H11 tests on main, then here.
RUN='TestH11|TestH10BriefCommandsRun'
{
  header "the H11 tests on $BASE (main) with this commit's tests copied in, then here"
  cp cmd/handoff/pickup_test.go cmd/handoff/hook_test.go "$SCRATCH/base/cmd/handoff/"
  echo "## at $BASE (expected: the TestH11 tests fail, since init needs --roles, settings needs --as and the brief has no post command; TestH10BriefCommandsRun passes)"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | cut -c1-220 | sed -e "s|$SCRATCH|<scratch>|g" -e 's|/[^ `]*/handoff-bin-[0-9]*/handoff|<test binary>|g')
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 3. What the operator sees: init with no flags, git's view, the settings' roles, and
# the brief.
{
  header "init with no flags, git status, settings claude and codex, the brief for claude, init elsewhere"
  R="$SCRATCH/demo"
  mkdir -p "$R" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base
  echo '$ handoff init'
  "$SCRATCH/handoff" init 2>&1
  echo; echo '$ cat .handoff/config.json'; cat .handoff/config.json
  echo; echo '$ cat .handoff/.gitignore'; cat .handoff/.gitignore
  echo; echo '$ git status --porcelain --untracked-files=all   (no output = nothing to commit)'; git status --porcelain --untracked-files=all
  echo '$ git status --porcelain --ignored'; git status --porcelain --ignored
  echo; echo '$ handoff settings claude | grep -E "HANDOFF_ROLE|is then"'
  "$SCRATCH/handoff" settings claude 2>&1 | grep -E 'HANDOFF_ROLE|is then'
  echo '$ handoff settings codex | grep -E "HANDOFF_ROLE|--as|is then"'
  "$SCRATCH/handoff" settings codex 2>&1 | grep -E 'HANDOFF_ROLE|--as|is then'
  echo; echo '$ handoff hook brief --as claude   (what SessionStart gives the model)'
  "$SCRATCH/handoff" hook brief --as claude </dev/null 2>&1
  echo; echo '$ handoff init --roles implementer,reviewer   (in a repository whose branch is master)'
  M="$SCRATCH/master"
  mkdir -p "$M" && cd "$M" && git init -q -b master && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base
  "$SCRATCH/handoff" init --roles implementer,reviewer 2>&1
  echo '$ handoff settings claude'
  "$SCRATCH/handoff" settings claude 2>&1; echo "# exit: $?"
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
