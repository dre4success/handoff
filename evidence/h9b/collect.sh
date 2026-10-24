#!/usr/bin/env bash
# Collects the H9b evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h9b/collect.sh
# Nothing here reaches the operator's screen: notifications and pop-ups go to files.
# live-popup.sh, which shows real pop-ups, is run separately, with the operator.
set -u
OUT=${OUT:-evidence/h9b}
COMMIT=$(git rev-parse HEAD)
BASE=$(git merge-base HEAD main)
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(mktemp -d)
trap 'for w in main nofix; do git worktree remove --force "$SCRATCH/$w" 2>/dev/null; done; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts-unused.txt"
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$SCRATCH/main" "$BASE"
(cd "$SCRATCH/main" && go build -o "$SCRATCH/handoff-main" ./cmd/handoff) || exit 1

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. Main against this commit. The new tests need code main lacks, so they
#    cannot build there. What a user sees differs too: the same steps, with
#    notifications and pop-ups sent to files, through each binary.
RUN='TestH9b|TestSandboxedProcessLeavesAlertsToOthers|TestDecideAlertIsHandedToAsk'
steps() { # $1 = binary, $2 = directory for the project and the files
  local H=$1 D=$2 R=$2/demo
  mkdir -p "$R" && (cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base)
  (cd "$R" && "$H" init --roles claude,codex --reviewers claude,codex --main main >/dev/null)
  local SHA; SHA=$(cd "$R" && git rev-parse HEAD)
  echo "please review" > "$D/req.md"; echo "looks right" > "$D/ok.md"; echo "Only to staging, tonight." > "$D/ask.md"
  touch "$D/popups.txt.release" # a test pop-up then comes back at once, unanswered
  (
    cd "$R"
    export HANDOFF_TEST_NOTIFY="$D/alerts.txt" HANDOFF_TEST_POPUP="$D/popups.txt"
    echo '$ handoff post ... ; handoff reply 1 --verdict APPROVE ... ; handoff note --to operator ...'
    "$H" post --as claude --item H1 --to codex --commit "$SHA" --title "H1: the login page" --body "$D/req.md" 2>&1
    "$H" reply --as codex 1 --verdict APPROVE --commit "$SHA" --body "$D/ok.md" 2>&1
    "$H" note --as claude --item H2 --to operator --title "May I deploy H2?" --body "$D/ask.md" 2>&1
    for _ in $(seq 50); do [ -e .handoff/notified/decide-1-2 ] && [ -e .handoff/notified/decide-note-3 ] && break; sleep 0.1; done
    echo '$ cat alerts.txt   (notifications)'; cat "$D/alerts.txt" 2>/dev/null || echo "(none)"
    echo '$ cat popups.txt   (pop-ups: Approve, Deny or Later)'; cat "$D/popups.txt" 2>/dev/null || echo "(none)"
    echo '$ ls .handoff/notified   (alerts marked sent)'; ls .handoff/notified | grep -v '\.lock$'
    echo '$ handoff   (both still wait: nothing was decided)'
    env -u HANDOFF_ROLE "$H" </dev/null 2>&1
  )
}
{
  header "the H9b tests on $BASE (main) with this commit's tests copied in, then here; then the same steps through each binary"
  cp cmd/handoff/popup_test.go "$SCRATCH/main/cmd/handoff/"
  cp internal/notify/notify_test.go "$SCRATCH/main/internal/notify/"
  echo "## the tests at $BASE (expected not to build: the pop-up isn't there)"
  (cd "$SCRATCH/main" && go test -count=1 -run "$RUN" ./cmd/handoff ./internal/notify 2>&1 | grep -E '^(ok|FAIL|---)|undefined|has no field' | head -12)
  echo
  echo "## the tests at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff ./internal/notify 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
  echo
  echo "## the steps through main's binary ($BASE): a notification, and no pop-up"
  mkdir -p "$SCRATCH/on-main"; steps "$SCRATCH/handoff-main" "$SCRATCH/on-main"
  echo
  echo "## the same steps through this commit's binary: each decision goes to a pop-up"
  mkdir -p "$SCRATCH/here"; steps "$SCRATCH/handoff" "$SCRATCH/here"
} > "$OUT/regressions.log" 2>&1

# 2b. Round 2 (GPT-6, #31): the helper held the alert's lock through a file
#     nothing referred to, so garbage collection could free the lock while
#     the dialog was up. The regression fails with the fix taken out.
FIX=2e8de3a
{
  header "TestH9bPopupAcrossProcesses, 5 runs, with fix $FIX taken out in a scratch worktree, then here"
  W="$SCRATCH/nofix"
  git worktree add -q --detach "$W" HEAD
  echo "## the fix, taken out:"
  (cd "$W" && git show "$FIX" -- cmd/handoff/popup.go | git apply -R && git diff)
  echo
  echo "## without the fix (expected to fail, 5 of 5)"
  (cd "$W" && go test -count=5 -v -run TestH9bPopupAcrossProcesses ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|lock was free')
  git worktree remove --force "$W"
  echo
  echo "## here (expected to pass, 5 of 5)"
  go test -count=5 -v -run TestH9bPopupAcrossProcesses ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/round2-lock-gc.log" 2>&1

# 3. Only a click answers: the helper refuses to run without the alert's lock.
(
  header "handoff _popup started by hand, as an agent might"
  cd "$SCRATCH/here/demo" || exit 1
  echo '$ handoff _popup decide-note-3 3   (no descriptor 3)'
  HANDOFF_TEST_POPUP="$SCRATCH/never.txt" "$SCRATCH/handoff" _popup decide-note-3 3 3<&- 2>&1; echo "# exit: $?"
  echo '$ handoff _popup decide-note-3 3 3<some other file'
  HANDOFF_TEST_POPUP="$SCRATCH/never.txt" "$SCRATCH/handoff" _popup decide-note-3 3 3<"$SCRATCH/here/req.md" 2>&1; echo "# exit: $?"
  echo '$ handoff _popup decide-note-3 3 3<the lock file, while another process holds it'
  python3 -I -c 'import fcntl,sys,time; f=open(sys.argv[1],"a"); fcntl.flock(f, fcntl.LOCK_EX); print("held", flush=True); time.sleep(5)' .handoff/notified/decide-note-3.lock > "$SCRATCH/held" &
  for _ in $(seq 50); do grep -q held "$SCRATCH/held" 2>/dev/null && break; sleep 0.1; done
  HANDOFF_TEST_POPUP="$SCRATCH/never.txt" "$SCRATCH/handoff" _popup decide-note-3 3 3<.handoff/notified/decide-note-3.lock 2>&1; echo "# exit: $?"
  wait
  echo '$ cat never.txt   (no dialog was shown)'; cat "$SCRATCH/never.txt" 2>/dev/null || echo "(none)"
  echo '$ handoff inbox --for operator   (nothing was decided)'
  "$SCRATCH/handoff" inbox --for operator 2>&1
) > "$OUT/helper-refusals.txt" 2>&1

# 4. Repeats, and vet, format and dependencies.
{
  header "go test -count=5 -v -run '$RUN' ./cmd/handoff ./internal/notify"
  go test -count=5 -v -run "$RUN" ./cmd/handoff ./internal/notify 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/repeat.log"
{
  header "go vet ./... ; gofmt -l . ; go.mod ; go list -deps (non-standard) ; go list -deps | grep net"
  echo '$ go vet ./...'; go vet ./... 2>&1; echo "# exit: $?"
  echo '$ gofmt -l .   (no output = formatted)'; gofmt -l .
  echo '$ cat go.mod'; cat go.mod
  echo '$ go list -deps -f "{{if not .Standard}}{{.ImportPath}}{{end}}" ./...'
  go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d'
  echo '$ go list -deps ./... | grep -E "^net(/|$)"   (no output = no net package)'
  go list -deps ./... | grep -E '^net(/|$)'; echo "# grep exit: $? (1 = no match)"
} > "$OUT/deps-vet-fmt.log"
echo "collected in $OUT at $COMMIT"
