#!/usr/bin/env bash
# Collects the H19 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h19/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. BASE is what H19 builds on: H18's branch.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
OUT=${OUT:-evidence/h19}
COMMIT=$(git rev-parse HEAD)
BASE=${BASE:-$(git rev-parse h18-git-and-init)}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID

# 1. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# Built from H18 (before) and from here (after).
mkdir "$SCRATCH/base-src" && git archive "$BASE" | tar -x -C "$SCRATCH/base-src"
(cd "$SCRATCH/base-src" && go build -o "$SCRATCH/before" ./cmd/handoff)
go build -o "$SCRATCH/after" ./cmd/handoff

# 2. The briefs, before and after, for the implementer and the reviewer, in a
# project made by init with no flags.
{
  header "handoff brief for implementer and reviewer, built from $BASE (H18) and from here"
  mkdir "$SCRATCH/briefs" && cd "$SCRATCH/briefs" && git init -q -b main && "$SCRATCH/after" init >/dev/null
  for bin in before after; do
    for role in implementer reviewer; do
      echo "## $bin: $role"
      HANDOFF_ROLE=$role "$SCRATCH/$bin" brief | sed -n '/^When something/,/^Rules/p'
    done
  done
  cd - >/dev/null
} 2>&1 | sed "s|$SCRATCH/before|handoff|g; s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g" > "$OUT/briefs.log"

# 3. A new project, as the operator met it: git init, files, no commit; init;
# the implementer commits on a branch and asks for a review. Before and after.
# Then, after: a second commit with no --base is refused; with one it posts.
newproject() { # newproject <bin>
  local dir="$SCRATCH/new-$1"
  mkdir "$dir" && cd "$dir" && git init -q -b main && echo "the work" > work.txt
  "$SCRATCH/$1" init >/dev/null
  git switch -q -c w1 && git add work.txt && git commit -q -m "the first work"
  echo "## $1: post the project's first commit, main with no commit"
  echo "please review" | HANDOFF_ROLE=implementer "$SCRATCH/$1" post --item W1 --to reviewer --commit HEAD --title "W1 first work" --body -; echo "# exit $?"
  cd - >/dev/null
}
{
  header "a new project with no commit: the implementer commits on a branch and posts it; built from $BASE (H18) and from here"
  newproject before
  newproject after
  cd "$SCRATCH/new-after"
  echo "## after: show the request: no base line"
  "$SCRATCH/after" show 1
  first=$(git rev-parse HEAD) && echo more >> work.txt && git commit -q -am "more work"
  echo "## after: a second commit, no --base"
  echo "later" | HANDOFF_ROLE=implementer "$SCRATCH/after" post --item W2 --to reviewer --commit HEAD --title "W2" --body -; echo "# exit $?"
  echo "## after: the same, with --base the first commit"
  echo "later" | HANDOFF_ROLE=implementer "$SCRATCH/after" post --item W2 --to reviewer --commit HEAD --base "$first" --title "W2" --body -; echo "# exit $?"
  cd - >/dev/null
} 2>&1 | sed "s|$SCRATCH/before|handoff|g; s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g" > "$OUT/new-project.log"

# 3b. Round 2 (row 86): the reviewer asks for changes on that first request;
# the fix goes on top and supersedes it, with no --base. Built from round 1
# (R1) and from here.
R1=${R1:-63a9d7b}
mkdir "$SCRATCH/r1-src" && git archive "$R1" | tar -x -C "$SCRATCH/r1-src"
(cd "$SCRATCH/r1-src" && go build -o "$SCRATCH/round1" ./cmd/handoff)
round2() { # round2 <bin>
  local dir="$SCRATCH/r2-$1"
  mkdir "$dir" && cd "$dir" && git init -q -b main && echo "the work" > work.txt
  "$SCRATCH/$1" init >/dev/null
  git switch -q -c w1 && git add work.txt && git commit -q -m "the first work"
  echo "please review" | HANDOFF_ROLE=implementer "$SCRATCH/$1" post --item W1 --to reviewer --commit HEAD --title "W1 first work" --body - >/dev/null
  HANDOFF_ROLE=reviewer "$SCRATCH/$1" claim 1 >/dev/null
  echo "1. fix this" | HANDOFF_ROLE=reviewer "$SCRATCH/$1" reply 1 --verdict CHANGES --commit HEAD --body - >/dev/null
  echo fixed >> work.txt && git commit -q -am "the fix"
  echo "## $1: round 2, the fix on top, --supersedes 1, no --base"
  echo "fixed" | HANDOFF_ROLE=implementer "$SCRATCH/$1" post --item W1 --to reviewer --commit HEAD --supersedes 1 --title "W1 round 2" --body -; echo "# exit $?"
  cd - >/dev/null
}
{
  header "a new project's first request, round 2: CHANGES, then the fix on top supersedes it; built from round 1 ($R1) and from here"
  round2 round1
  round2 after
  echo "## after: show the round-2 request: no base line"
  (cd "$SCRATCH/r2-after" && "$SCRATCH/after" show 4)
} 2>&1 | sed "s|$SCRATCH/round1|handoff|g; s|$SCRATCH/after|handoff|g; s|$SCRATCH|<scratch>|g" > "$OUT/round-2.log"

# 4. The sweep of every tracked file here.
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

# 5. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## alerts sent during collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} > "$OUT/deps-vet-fmt.log" 2>&1
