#!/usr/bin/env bash
# The reviewer's H19 round-2 evidence. Run from a checkout of the commit under review:
#   OUT=<this directory> bash collect.sh <h14-patterns.txt>
# The patterns file is H14's, kept outside the repository. Alerts and sleep
# holds go to scratch files. Nothing is installed; no module version is
# fetched. Paths of the scratch folder, this checkout and the repository are
# shown as <scratch>, <worktree> and <repo>.
set -u
OUT=${OUT:?}
PATTERNS=$(cd "$(dirname "${1:?h14-patterns.txt}")" && pwd -P)/$(basename "$1")
COMMIT=$(git rev-parse HEAD)
BASE=e832f00bdef94a3be2bfcf2f934e4816b5bd20c2
ROUND1=63a9d7b06ab928297b160418d786decbaf788cff
MAIN=$(git rev-parse main)
WT=$(pwd -P)
REPO=$(cd "$(git rev-parse --git-common-dir)/.." && pwd -P)
OBJ=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/objects
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
scrub() { sed "s|$SCRATCH|<scratch>|g; s|$WT|<worktree>|g; s|$REPO|<repo>|g; s|$HOME|~|g"; }
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

# 1. Build, vet, gofmt, no net, no dependencies; the branch's shape.
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go list -deps ./... | grep net; go.mod; git log/diff --stat"
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go list -deps ./... matching net: [$(go list -deps ./... | grep -E '(^|/)net(/|$)' | tr '\n' ' ')]"
  echo "## go.mod: $(grep -c '^require' go.mod) require lines; $(grep '^go ' go.mod)"
  echo "## commits on round 1 ($ROUND1):"; git log --oneline $ROUND1..HEAD; echo "## round 1 is an ancestor of HEAD: $(git merge-base --is-ancestor $ROUND1 HEAD && echo yes || echo no)"
  echo "## the base is an ancestor of HEAD: $(git merge-base --is-ancestor $BASE HEAD && echo yes || echo no); of main ($MAIN): $(git merge-base --is-ancestor $BASE $MAIN && echo yes || echo "no (H18 is not merged yet)")"
  echo "## main since the branch point ($(git merge-base HEAD $MAIN)): $(git diff --name-only "$(git merge-base HEAD $MAIN)" $MAIN | tr '\n' ' ')"
  echo "## files changed, round 1 to HEAD:"; git diff --stat $ROUND1 HEAD
  echo "## the evidence commit alone:"; git diff --stat HEAD^ HEAD | tail -1
  echo "## the no-base rule in cmd/handoff/cmds.go:"; grep -n -B2 -A12 'len(ps) == 0' cmd/handoff/cmds.go
} 2>&1 | scrub > "$OUT/build-vet-deps.log"

# 2. The probes.
OUT="$OUT" bash "$OUT/loop2.sh"

# 3. A read-only merge dry run, and H14's sweep of this commit, of main, and
#    of the merged tree with the reviewer's evidence in it.
cat > "$SCRATCH/sweep.py" <<'PY'
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
PY
{
  header "git merge-tree --write-tree main HEAD (objects to a temporary directory); H14's sweep over git archive of HEAD, of main, and of the merged tree plus evidence/reviewer/H19-${COMMIT:0:7}/"
  before=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  mkdir "$SCRATCH/obj"; MERGED=$(GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git merge-tree --write-tree "$MAIN" "$COMMIT" 2>/dev/null); echo "## merge-tree exit $? (0: clean); merged tree $MERGED"
  after=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  echo "## objects in the repository before $before, after $after; in the temporary directory: $(find "$SCRATCH/obj" -type f | wc -l | tr -d ' ')"
  for rev in "$COMMIT" "$MAIN" "$MERGED"; do
    echo "## $rev"; mkdir "$SCRATCH/t"
    GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git archive "$rev" | tar -x -C "$SCRATCH/t"
    if [ "$rev" = "$MERGED" ]; then mkdir -p "$SCRATCH/t/evidence/reviewer/H19-${COMMIT:0:7}" && cp "$OUT"/* "$SCRATCH/t/evidence/reviewer/H19-${COMMIT:0:7}/" && echo "(with the reviewer's evidence so far: $(ls "$OUT" | tr '\n' ' '))"; fi
    (cd "$SCRATCH/t" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | python3 -I "$SCRATCH/sweep.py" "$PATTERNS")
    rm -rf "$SCRATCH/t"
  done
} 2>&1 | scrub > "$OUT/merge-and-sweep.log"

# 4. Every test under the race detector, in this checkout.
{
  header "go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS|panic)'"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS|panic)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} 2>&1 | scrub > "$OUT/go-test-race.log"
{
  echo "# tests: $(grep -c '^--- PASS' "$OUT/go-test-race.log") pass, $(grep -c '^--- FAIL' "$OUT/go-test-race.log") fail; H19's: $(grep -E '^--- .*H19' "$OUT/go-test-race.log" | sed 's/ (.*//' | tr '\n' ' ')"
  echo "# alerts sent to the scratch file during this collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} >> "$OUT/go-test-race.log"
