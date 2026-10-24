#!/usr/bin/env bash
# Collects the H17 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h17/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H17 builds on (main).
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h17}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git merge-base HEAD main)}
SCRUB=$(git log --format=%h -1 --grep="^refactor: name no person in the reviewer's H16")
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
[ -n "$SCRUB" ] || { echo "no scrub commit found"; exit 1; }

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 2. handoff version, built here, and what Go records in the binary.
{
  header "go build, then handoff version and --version, and go version -m"
  go build -o "$SCRATCH/handoff" ./cmd/handoff && "$SCRATCH/handoff" version && "$SCRATCH/handoff" --version
  go version -m "$SCRATCH/handoff" | grep -E '^\s+(path|mod)\s' | sed "s|$SCRATCH|<scratch>|"
} > "$OUT/version.log" 2>&1

# 3. The sweep of every tracked file, on main and here; the renaming changes
# names and paths only, in the reviewer's H16 evidence alone; and no hash in
# the tree is of the bytes it changed.
sweep() {
  python3 -I -c '
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
}
{
  header "H14's sweep at $BASE (main) and here; git diff --numstat, --name-only and the words put in by $SCRUB; H14's hash sweep"
  echo "## the sweep at $BASE (main)"
  (mkdir "$SCRATCH/base" && git archive "$BASE" | tar -x -C "$SCRATCH/base" && cd "$SCRATCH/base" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | sweep)
  echo "## the sweep here"
  git ls-files -z | sweep
  echo "## $SCRUB"
  git diff --numstat "$SCRUB^" "$SCRUB" | awk '{a+=$1; d+=$2; n++} END {print n " files; " a " lines in, " d " lines out"}'
  git diff --name-only "$SCRUB^" "$SCRUB"
  git diff --word-diff=porcelain "$SCRUB^" "$SCRUB" | grep '^+' | grep -v '^+++' | sed 's/^+//' | sort | uniq -c | sort -rn
  echo "## hashes"
  python3 -I evidence/h14/stale-hashes.py "$SCRUB^" "$SCRUB" "$SCRUB^" "$COMMIT"
} > "$OUT/names.log" 2>&1

# 4. The README, before and after.
{
  header "wc -w README.md at $BASE (main) and here; its sections"
  echo "words at $BASE: $(git show "$BASE:README.md" | wc -w | tr -d ' ')"
  echo "words here: $(wc -w < README.md | tr -d ' ')"
  echo "## sections here"
  grep '^## ' README.md
} > "$OUT/readme.log" 2>&1

# 5. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1

# 6. Round 2 (row 82): the version test and handoff version in a plain clone,
# not a linked worktree: with no tag, at a v0.1.0 tag, and one commit after.
{
  header "git clone --no-local of this commit; then go build, handoff version and go test -run TestH17 with no tag, at a v0.1.0 tag, and one commit after it"
  git clone -q --no-local "$(git rev-parse --show-toplevel)" "$SCRATCH/clone" && git -C "$SCRATCH/clone" checkout -q --detach "$COMMIT"
  for step in "no tag" "at v0.1.0" "one commit after v0.1.0"; do
    case $step in
      "at v0.1.0") git -C "$SCRATCH/clone" tag v0.1.0 ;;
      "one commit after v0.1.0") git -C "$SCRATCH/clone" -c user.name=t -c user.email=t@t commit -q --allow-empty -m after ;;
    esac
    echo "## $step"
    (cd "$SCRATCH/clone" && go build -o "$SCRATCH/clone-handoff" ./cmd/handoff && "$SCRATCH/clone-handoff" version &&
      go test -count=1 -v -run TestH17 ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|_test\.go:[0-9]+:')
  done
} > "$OUT/clone-version.log" 2>&1
