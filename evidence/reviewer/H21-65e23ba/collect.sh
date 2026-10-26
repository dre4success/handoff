#!/usr/bin/env bash
# The reviewer's H21 evidence, but for the probes (probes.sh), the live check
# (live-hidden.sh) and the race suite (go-test-race.log, run separately in
# the same worktree). Run from a checkout of the commit under review:
#   OUT=<this directory> bash collect.sh <h14-patterns.txt>
# The patterns file is H14's, kept outside the repository.
set -u
OUT=$(cd "${OUT:?}" && pwd -P)
PATTERNS=$(cd "$(dirname "${1:?h14-patterns.txt}")" && pwd -P)/$(basename "$1")
COMMIT=$(git rev-parse HEAD)
BASE=0f68be51b6e0b4ded2984af705b050c4c536d286
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt" LC_ALL=C
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export CLAUDE_CONFIG_DIR="$SCRATCH/claude-home" CODEX_HOME="$SCRATCH/codex-home"; mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"

# 1. Build, vet, gofmt, no net, no dependencies; what the branch adds.
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go list -deps ./... | grep net; go.mod; git log/diff --stat $BASE..HEAD"
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go list -deps ./... matching net: [$(go list -deps ./... | grep -E '(^|/)net(/|$)' | tr '\n' ' ')]"
  echo "## go.mod: $(grep -c '^require' go.mod) require lines; $(grep '^go ' go.mod)"
  echo "## the branch's commits on $BASE (main at the v0.1.1 release):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## base is an ancestor: $(git merge-base --is-ancestor $BASE HEAD && echo yes || echo no); merge base with main: $(git merge-base main HEAD)"
  echo "## files changed:"; git diff --stat $BASE HEAD | tail -25
  echo "## the implementer's evidence commit adds only evidence/h21/: $(git diff --name-only HEAD^ HEAD | grep -v '^evidence/h21/' | wc -l | tr -d ' ') other files"
  echo "## go test (no race) ./...:"; go test -count=1 ./... 2>&1 | tail -5
} > "$OUT/build-vet-deps.log" 2>&1

# 2. The merge dry run, and H14's sweep: here, on main, on the merged tree, and over the reviewer's evidence.
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
MAIN=$(git rev-parse main); OBJ=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/objects
{
  header "git merge-tree --write-tree main ($MAIN) HEAD, objects to a temporary directory; H14's sweep over git archive of HEAD, of main, of the merged tree, and over the reviewer's evidence folder"
  before=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  mkdir "$SCRATCH/obj"; MERGED=$(GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git merge-tree --write-tree "$MAIN" "$COMMIT" 2>/dev/null); echo "## merge-tree exit $? (0: clean); merged tree $MERGED; HEAD's tree $(git rev-parse 'HEAD^{tree}') (the same when main is the base)"
  after=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  echo "## objects in the repository before $before, after $after; in the temporary directory: $(find "$SCRATCH/obj" -type f | wc -l | tr -d ' ')"
  for rev in "$COMMIT" "$MAIN" "$MERGED"; do
    echo "## $rev"; mkdir "$SCRATCH/t"
    GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git archive "$rev" | tar -x -C "$SCRATCH/t" && (cd "$SCRATCH/t" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | python3 -I "$SCRATCH/sweep.py" "$PATTERNS")
    rm -rf "$SCRATCH/t"
  done
  echo "## the reviewer's evidence folder ($(basename "$OUT")), as it is now"
  (cd "$OUT" && find . -type f | sed 's|^\./||' | sort | tr '\n' '\0' | python3 -I "$SCRATCH/sweep.py" "$PATTERNS")
  echo "## alerts sent during this collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} > "$OUT/merge-and-sweep.log" 2>&1
