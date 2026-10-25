#!/usr/bin/env bash
# Collects the H18 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h18/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H18 builds on (main).
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h18}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 2. What a user sees, built from main and from here: init, inbox and brief
# in a folder outside any repository; then git init, and init; then each
# command init names, run through sh as printed. Paths are shown as <scratch>.
mkdir "$SCRATCH/base-src" && git archive "$BASE" | tar -x -C "$SCRATCH/base-src"
(cd "$SCRATCH/base-src" && go build -o "$SCRATCH/base-handoff" ./cmd/handoff)
go build -o "$SCRATCH/handoff" ./cmd/handoff
show() { # show <label> <binary>
  local dir="$SCRATCH/$1"
  mkdir "$dir"
  echo "## $1: outside a git repository"
  for c in init inbox brief; do
    (cd "$dir" && GIT_CEILING_DIRECTORIES="$SCRATCH" "$2" "$c" </dev/null; echo "# exit $?") 2>&1
  done
  echo "## $1: after git init -b main, init"
  (cd "$dir" && git init -q -b main && "$2" init </dev/null; echo "# exit $?") 2>&1
}
# Round 2 (row 84): a pointer to a repository that isn't there gets no hint:
# a worktree's .git file naming a missing path, and GIT_DIR naming one.
missing() { # missing <label> <binary>
  mkdir "$SCRATCH/$1-gone" "$SCRATCH/$1-nowhere"
  echo "gitdir: $SCRATCH/$1-main/.git/worktrees/gone" > "$SCRATCH/$1-gone/.git"
  echo "## $1: init in a worktree whose repository is gone"
  (cd "$SCRATCH/$1-gone" && GIT_CEILING_DIRECTORIES="$SCRATCH" "$2" init </dev/null; echo "# exit $?") 2>&1
  echo "## $1: init with GIT_DIR naming a missing path"
  (cd "$SCRATCH/$1-nowhere" && GIT_CEILING_DIRECTORIES="$SCRATCH" GIT_DIR="$SCRATCH/$1-nowhere/nowhere" "$2" init </dev/null; echo "# exit $?") 2>&1
}
{
  header "handoff init, inbox and brief outside a git repository, then git init and init, built from $BASE (main) and from here; init where the repository is missing; each command init names, through sh"
  show main "$SCRATCH/base-handoff"
  show here "$SCRATCH/handoff"
  missing main "$SCRATCH/base-handoff"
  missing here "$SCRATCH/handoff"
  echo "## here: each command init printed, after its name, run through sh"
  (cd "$SCRATCH/here" && rm -rf .handoff && "$SCRATCH/handoff" init) | sed -n -E 's/^  (Claude Code|Codex): *//p' | while read -r line; do
    (cd "$SCRATCH/here" && /bin/sh -c "$line" >"$SCRATCH/out.txt" 2>&1; echo "$line: exit $?, $(wc -l <"$SCRATCH/out.txt" | tr -d ' ') lines, first: $(head -1 "$SCRATCH/out.txt")")
  done
} 2>&1 | sed "s|$SCRATCH|<scratch>|g" > "$OUT/outputs.log"

# 3. The sweep of every tracked file here.
{
  header "H14's sweep of every tracked file here"
  git ls-files -z | python3 -I -c '
import re, sys
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
files = [f for f in sys.stdin.read().split("\0") if f]
print(f"{len(files)} files")
for label, rx in pats:
    r = re.compile(rx)
    lines = names = 0
    for f in files:
        names += bool(r.search(f))
        try:
            text = open(f, encoding="utf-8", errors="surrogateescape").read()
        except OSError:
            continue
        lines += sum(1 for l in text.split("\n") if r.search(l))
    print(f"{label}: {lines} lines, {names} file names")
' "$PATTERNS"
} > "$OUT/names.log" 2>&1

# 4. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1

echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)" >> "$OUT/deps-vet-fmt.log"
