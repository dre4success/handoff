#!/usr/bin/env bash
# Collects the H9 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h9/collect.sh
set -u
OUT=${OUT:-evidence/h9}
COMMIT=$(git rev-parse HEAD)
BASE=$(git merge-base HEAD main)
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT
# Alerts from the scratch projects go to a file, never to the operator's screen.
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt"
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
H=$SCRATCH/handoff

# 1. Every test, verbose, under the race detector.
{
  header "go test -count=1 -race -v ./..."
  go test -count=1 -race -v ./... 2>&1
  echo "# go test exit: $?"
} > "$OUT/go-test-race-v.log"

# 2. The H9 tests fail on main and pass here.
RUN='TestH9|TestDecideWithATerminal|TestCheck16'
{
  header "the H9 tests, on a worktree of $BASE (main) with this commit's tests copied in, then on this commit"
  W="$SCRATCH/red"
  git worktree add -q --detach "$W" "$BASE"
  cp cmd/handoff/decide_test.go cmd/handoff/main_test.go "$W/cmd/handoff/"
  echo "## at $BASE (expected to fail; TestCheck16 to pass)"
  (cd "$W" && go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)')
  git worktree remove --force "$W"
  echo
  echo "## at $COMMIT (expected to pass)"
  go test -count=1 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/regressions.log"

# 3. What the operator sees, at a real pseudo-terminal: two things wait, one approved
#    with a note, one skipped; then decide by item name, with no role.
R="$SCRATCH/demo"
mkdir -p "$R" && (cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m "H1 work")
(cd "$R" && "$H" init --roles claude,codex --reviewers claude,codex --main main >/dev/null)
SHA=$(cd "$R" && git rev-parse HEAD)
echo "please review" > "$SCRATCH/req.md"; echo "looks right" > "$SCRATCH/ok.md"; echo "staging only" > "$SCRATCH/ask.md"
(cd "$R" && "$H" post --as claude --item H1 --to codex --commit "$SHA" --title "H1: the login page" --body "$SCRATCH/req.md" >/dev/null &&
  "$H" reply --as codex 1 --verdict APPROVE --commit "$SHA" --body "$SCRATCH/ok.md" >/dev/null &&
  "$H" note --as claude --item H2 --to operator --title "May I deploy H2?" --body "$SCRATCH/ask.md" >/dev/null)
drive() { # $1 = directory; then pairs: what to wait for, what to type; then -- and the command
  python3 -I - "$@" <<'PY'
import os, pty, sys, time
d, rest = sys.argv[1], sys.argv[2:]
cut = rest.index("--")
pairs, cmd = rest[:cut], rest[cut + 1:]
os.chdir(d)
pid, fd = pty.fork()
if pid == 0:
    os.execvp(cmd[0], cmd)
shown = b""
def read_until(want):
    global shown
    deadline = time.time() + 20
    while want.encode() not in shown and time.time() < deadline:
        try:
            chunk = os.read(fd, 4096)
        except OSError:
            break
        if not chunk:
            break
        shown += chunk
for i in range(0, len(pairs), 2):
    read_until(pairs[i])
    os.write(fd, pairs[i + 1].encode() + b"\n")  # the terminal echoes it, as the operator would see
    time.sleep(0.2)
read_until("\0")  # to the end
_, status = os.waitpid(pid, 0)
print(shown.decode(errors="replace").replace("\r\n", "\n").rstrip())
print(f"# exit: {os.waitstatus_to_exitcode(status)}")
PY
}
{
  header "handoff on its own, and decide by item, at a pseudo-terminal, with no HANDOFF_ROLE"
  echo "A scratch project: claude asked codex to review H1 and codex approved it; claude asked the operator whether to deploy H2. Alerts go to a file here, not to the operator's screen."
  echo
  echo '$ handoff        (the operator approves H1 with a note, skips H2)'
  drive "$R" "[a/d/s]" a "Note (optional):" "merge it" "[a/d/s]" s -- env -u HANDOFF_ROLE "$H"
  echo
  echo '$ handoff decide H2 --deny --note "not this week"'
  drive "$R" "[y/N]" y -- env -u HANDOFF_ROLE "$H" decide H2 --deny --note "not this week"
  echo
  echo '$ handoff        (nothing left)'
  drive "$R" -- env -u HANDOFF_ROLE "$H"
  echo
  echo '$ handoff show 4 ; handoff show 5   (the two decisions as written)'
  (cd "$R" && "$H" show 4 && echo && "$H" show 5) 2>&1
  echo
  echo '$ echo a | handoff   (no terminal: it lists, writes nothing)'
  (cd "$R" && "$H" post --as claude --item H3 --to codex --commit "$SHA" --title "H3" --body "$SCRATCH/req.md" >/dev/null &&
    "$H" reply --as codex 6 --verdict APPROVE --commit "$SHA" --body "$SCRATCH/ok.md" >/dev/null &&
    echo a | env -u HANDOFF_ROLE "$H"; echo "# exit: $?"; "$H" inbox --for operator --quiet; echo "# inbox --quiet exit: $? (1 = H3 still waits)")
  echo
  echo '$ HANDOFF_ROLE=codex handoff   (an agent'"'"'s session: its inbox)'
  (cd "$R" && HANDOFF_ROLE=codex "$H"; echo "# exit: $?")
} > "$OUT/terminal-demo.txt" 2>&1

# 4. Repeats, and vet, format and dependencies.
{
  header "go test -count=5 -v -run '$RUN' ./cmd/handoff"
  go test -count=5 -v -run "$RUN" ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)'
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
