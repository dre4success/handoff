#!/usr/bin/env bash
# Collects the H10b evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h10b/collect.sh
# BASE is what H10b builds on (h10-sessions, H10a). Notifications go to a
# file. live-codex.sh, which starts a real throwaway Codex session, is run
# separately; probe/ holds the probe made before the code, and probe-env/ the
# one for round 3's config.toml.
set -u
OUT=${OUT:-evidence/h10b}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD h10-sessions)}
ROUND2=${ROUND2:-418a0c4} # the commit review round 2 looked at
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'), $(codex --version 2>/dev/null))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; git worktree remove --force "$SCRATCH/round2" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
unset HANDOFF_ROLE
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/base" "$BASE"

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The new tests on H10a, then here.
RUN='TestH10b'
{
  header "the H10b tests on $BASE (H10a) with this commit's tests copied in, then here"
  cp cmd/handoff/codex_test.go "$SCRATCH/base/cmd/handoff/"
  echo "## at $BASE (expected: the settings, brief and role tests fail, since settings codex and the brief's --as aren't there; the shellQuote test passes, since H10a has it)"
  (cd "$SCRATCH/base" && go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:' | sed "s|$SCRATCH|<scratch>|g")
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN|TestH10Settings|TestH10Brief" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log" 2>&1

# 2b. Round 3: the tests for round 2's findings, on round 2's commit, then here.
RUN3='TestH10bTheRoleKeepsBareHandoffFromTheOperatorsQuestion|TestH10BriefCommandsRun'
{
  header "the round-3 tests on $ROUND2 (round 2) with this commit's tests copied in, then here"
  git worktree add -q --detach "$SCRATCH/round2" "$ROUND2"
  cp cmd/handoff/codex_test.go cmd/handoff/hook_test.go "$SCRATCH/round2/cmd/handoff/"
  echo "## at $ROUND2 (expected to fail: settings codex prints no config.toml; the brief's question to the operator names no item)"
  (cd "$SCRATCH/round2" && go test -count=1 -v -run "$RUN3" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:|--item' | cut -c1-220 | sed -e "s|$SCRATCH|<scratch>|g" -e 's|/[^ `]*/handoff-bin-[0-9]*/handoff|<test binary>|g')
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN3" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/round3.log" 2>&1

# 3. What the operator pastes, and how Codex reads the rule.
{
  header "settings codex, Codex's rule check, and the brief a Codex session gets"
  R="$SCRATCH/demo"
  mkdir -p "$R" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
    "$SCRATCH/handoff" init --roles claude,codex --reviewers claude,codex --main main >/dev/null
  echo '$ handoff settings codex --as codex'
  "$SCRATCH/handoff" settings codex --as codex > "$SCRATCH/settings.txt" 2>&1
  sed "s|$SCRATCH|<scratch>|g" "$SCRATCH/settings.txt"
  sed -n '/^\.codex\/rules\/handoff\.rules:$/,$p' "$SCRATCH/settings.txt" | sed '1d' > "$SCRATCH/handoff.rules"
  echo
  for c in "show 1" "claim --as codex 1" "reply --as codex 1 --verdict APPROVE --commit HEAD --body r.md" "decide T1 --approve"; do
    echo "\$ codex execpolicy check --rules handoff.rules -- handoff $c"
    codex execpolicy check --pretty --rules "$SCRATCH/handoff.rules" -- "$SCRATCH/handoff" $c 2>&1 | grep '^  "decision"'
  done
  echo
  echo '$ handoff hook brief --as codex   (what SessionStart gives the model)'
  "$SCRATCH/handoff" hook brief --as codex </dev/null 2>&1 | sed "s|$SCRATCH|<scratch>|g"
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
