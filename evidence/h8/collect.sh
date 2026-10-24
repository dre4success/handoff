#!/usr/bin/env bash
# Collects the H8 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h8/collect.sh
set -u
OUT=${OUT:-evidence/h8}
COMMIT=$(git rev-parse HEAD)
BASE=$(git merge-base HEAD main)
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
H=$SCRATCH/handoff

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The CLI-level H8 tests fail on main and pass here. The unit tests in
#    status, notify and wait use the new API, so on main they do not build.
RUN='TestH8'
{
  header "the H8 CLI tests, on a worktree of $BASE (main) with this commit's tests copied in, then on this commit"
  W="$SCRATCH/red"
  git worktree add -q --detach "$W" "$BASE"
  cp cmd/handoff/alert_test.go cmd/handoff/main_test.go "$W/cmd/handoff/"
  echo "## at $BASE (expected to fail)"
  (cd "$W" && go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:')
  git worktree remove --force "$W"
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test.go:'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log"

# 3. The trial's own record, as it stood while Codex was stuck: the first six
#    entries, ending with the reviewer's claim of #5. What does H8 tell the operator?
{
  header "replay of evidence/trial/record/channel.jsonl, entries #1 to #6, through this commit's binary"
  R="$SCRATCH/replay"
  mkdir -p "$R" && (cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base)
  (cd "$R" && "$H" init --roles implementer,reviewer --reviewers reviewer --main main >/dev/null)
  head -n 6 evidence/trial/record/channel.jsonl > "$R/.handoff/channel.jsonl"
  python3 -I - "$R/.handoff/channel.jsonl" "$R/.handoff/head.json" <<'PY'
import sys, json, hashlib
lines = open(sys.argv[1], 'rb').read().split(b'\n')[:-1]
last = hashlib.sha256(lines[-1]).hexdigest()
json.dump({"v": 1, "lines": len(lines), "last": last, "id": json.loads(lines[-1])["id"]}, open(sys.argv[2], 'w'))
print(f"replay record: {len(lines)} entries; the last is #{json.loads(lines[-1])['id']} {json.loads(lines[-1])['kind']} at {json.loads(lines[-1])['ts']}")
PY
  echo
  echo '$ handoff inbox --for operator'
  (cd "$R" && "$H" inbox --for operator 2>&1)
  echo
  echo '$ HANDOFF_TEST_NOTIFY=sent.txt handoff wait --for implementer --timeout 3s'
  echo '  (a fresh cursor, so #3 is delivered at once; the alert is passed on before it returns)'
  (cd "$R" && HANDOFF_TEST_NOTIFY="$SCRATCH/sent.txt" "$H" wait --for implementer --timeout 3s 2>&1; echo "# wait exit: $?")
  echo '$ cat sent.txt   (what the operator would have been shown)'
  cat "$SCRATCH/sent.txt"
  echo '$ HANDOFF_TEST_NOTIFY=sent.txt handoff wait --for implementer --timeout 3s   (a second wait, as during the stall: nothing new)'
  (cd "$R" && HANDOFF_TEST_NOTIFY="$SCRATCH/sent.txt" "$H" wait --for implementer --timeout 3s 2>&1; echo "# wait exit: $?")
  echo '$ wc -l sent.txt   (1 = told once)'
  wc -l < "$SCRATCH/sent.txt"
} > "$OUT/trial-replay.txt"

# 4. Repeats, and vet, format and dependencies.
{
  header "go test -count=5 -v -run 'TestH8|Alert|Quiet|Claim|Approve|Note|Superseded|Unreadable|Once|Racing|NewKey|Markers|Unsafe|Silence|WithoutNotify|Lock|Paused|Killed|Retried|Recovers|BacksOff' ./..."
  go test -count=5 -v -run 'TestH8|Alert|Quiet|Claim|Approve|Note|Superseded|Unreadable|Once|Racing|NewKey|Markers|Unsafe|Silence|WithoutNotify|Lock|Paused|Killed|Retried|Recovers|BacksOff' ./... 2>&1 | grep -E '^(--- |ok|FAIL)'
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
