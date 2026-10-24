#!/usr/bin/env bash
# The reviewer's H17 round-2 evidence. Run from a checkout of the commit under review:
#   OUT=<this directory> bash collect.sh <h14-scrub.py> <h14-patterns.txt>
# The script and the patterns file are H14's, kept outside the repository.
# Alerts and sleep holds go to scratch files. The race suite is collected
# separately (go-test-race.log), in a plain clone.
set -u
OUT=${OUT:?}
SCRUBPY=$(cd "$(dirname "${1:?h14-scrub.py}")" && pwd -P)/$(basename "$1")
PATTERNS=$(cd "$(dirname "${2:?h14-patterns.txt}")" && pwd -P)/$(basename "$2")
COMMIT=$(git rev-parse HEAD)
BASE=f1fcd1738894e9dcd28c7ac4b2fed5bf7aa87b04
ROUND1=cbb09c235ffb661bf0c73f8726800fda9ed309d3
SCRUB=d9dd08e
PREV=$(git rev-parse HEAD^)
REPO=$(cd "$(git rev-parse --git-common-dir)/.." && pwd -P)
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
T='TestH17VersionNamesTheBuild'
vtest() { go test -count=1 -run "$T" ./cmd/handoff 2>&1 | grep -E 'version_test|^(ok|FAIL|---)' | cut -c1-160; }

# 1. Build, vet, gofmt, no net, no dependencies; what round 2 adds.
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go list -deps ./... | grep net; go.mod; git log/diff --stat"
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go list -deps ./... matching net: [$(go list -deps ./... | grep -E '(^|/)net(/|$)' | tr '\n' ' ')]"
  echo "## go.mod: $(grep -c '^require' go.mod) require lines; $(grep '^go ' go.mod)"
  echo "## commits on top of round 1 ($ROUND1):"; git log --oneline $ROUND1..HEAD
  echo "## round 1 is an ancestor: $(git merge-base --is-ancestor $ROUND1 HEAD && echo yes || echo no); base of the branch: $(git merge-base main HEAD)"
  echo "## files changed after $PREV, where the implementer's evidence was collected:"; git diff --stat $PREV HEAD
  echo "## the test's regex now: $(grep -o 'MustCompile(`[^`]*`)' cmd/handoff/version_test.go)"
} > "$OUT/build-vet-deps.log" 2>&1

# 2. handoff version and its test: in this worktree, and in a plain clone untagged, dirty, at a tag, dirty at a tag, after a tag; go install via a file proxy; from /.
{
  header "go build, handoff version and go test -run $T, in this worktree and in a plain clone in each state; go install …@v0.1.0 via a file:// module proxy"
  echo "## this checkout (a linked worktree): $(go build -o "$SCRATCH/here" ./cmd/handoff && "$SCRATCH/here" version)"; vtest
  git clone -q --no-hardlinks "$REPO" "$SCRATCH/clone" && cd "$SCRATCH/clone" && git checkout -q "$COMMIT"
  echo "## a plain clone, no tag: $(go build -o "$SCRATCH/b" ./cmd/handoff && "$SCRATCH/b" version)"; vtest
  echo x >> README.md
  echo "## no tag, dirty tree: $(go build -o "$SCRATCH/b" ./cmd/handoff && "$SCRATCH/b" version)"; vtest
  git checkout -q -- README.md; git tag v0.1.0
  echo "## v0.1.0 on this commit: $(go build -o "$SCRATCH/b" ./cmd/handoff && "$SCRATCH/b" version)"; vtest
  echo x >> README.md
  echo "## at the tag, dirty tree: $(go build -o "$SCRATCH/b" ./cmd/handoff && "$SCRATCH/b" version)"; vtest
  git checkout -q -- README.md; git tag -d v0.1.0 >/dev/null; git tag v0.1.0 "$PREV"
  echo "## one commit after v0.1.0 (tag on $PREV): $(go build -o "$SCRATCH/b" ./cmd/handoff && "$SCRATCH/b" version)"; vtest
  git tag -d v0.1.0 >/dev/null; git tag v0.1.0
  P="$SCRATCH/proxy/github.com/dre4success/handoff/@v" && mkdir -p "$P"
  git archive --format=zip --prefix=github.com/dre4success/handoff@v0.1.0/ -o "$P/v0.1.0.zip" v0.1.0
  git show v0.1.0:go.mod > "$P/v0.1.0.mod"; echo '{"Version":"v0.1.0","Time":"2026-10-24T21:00:00Z"}' > "$P/v0.1.0.info"; echo v0.1.0 > "$P/list"
  (cd / && GOBIN="$SCRATCH/gobin" GOPROXY="file://$SCRATCH/proxy" GOSUMDB=off GOFLAGS= go install github.com/dre4success/handoff/cmd/handoff@v0.1.0 2>&1)
  echo "## go install …@v0.1.0 through a file:// proxy of the tag: $("$SCRATCH/gobin/handoff" version); --version: $("$SCRATCH/gobin/handoff" --version)"
  go version -m "$SCRATCH/gobin/handoff" | grep -E '^\s+(path|mod)'
  echo "## from /, outside any project: $(cd / && "$SCRATCH/gobin/handoff" version); exit $?"
  cd "$OLDPWD"
} > "$OUT/version.log" 2>&1

# 3. The renaming still reproduces; no stale hash.
{
  header "python3 -I h14-scrub.py <worktree at $BASE> against $SCRUB; python3 -I evidence/h14/stale-hashes.py $SCRUB^ $SCRUB $SCRUB^ $COMMIT"
  git worktree add -q --detach "$SCRATCH/base" "$BASE"
  python3 -I "$SCRUBPY" "$SCRATCH/base"
  (cd "$SCRATCH/base" && git add -A && git diff --cached | grep -v '^index ' > "$SCRATCH/reproduced.diff")
  git diff "$SCRUB^" "$SCRUB" | grep -v '^index ' > "$SCRATCH/committed.diff"
  cmp -s "$SCRATCH/reproduced.diff" "$SCRATCH/committed.diff" && echo "## the reproduced diff is identical to $SCRUB's; tree $(cd "$SCRATCH/base" && git write-tree) = $(git rev-parse "$SCRUB^{tree}")" || { echo "## DIFFERS"; diff "$SCRATCH/reproduced.diff" "$SCRATCH/committed.diff"; }
  git worktree remove --force "$SCRATCH/base"
  python3 -I evidence/h14/stale-hashes.py "$SCRUB^" "$SCRUB" "$SCRUB^" "$COMMIT"
} > "$OUT/scrub.log" 2>&1

# 4. The sweep, counts only: here, on main (with the reviewer's round-1 evidence), and on the tree the merge would make.
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
  header "H14's sweep over git archive of this commit, of main ($MAIN), and of the tree git merge-tree makes of the two (objects in a temporary directory)"
  echo "## main since the base changed: $(git diff --name-only $BASE $MAIN | tr '\n' ' ')"
  before=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  mkdir "$SCRATCH/obj"; MERGED=$(GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git merge-tree --write-tree "$MAIN" "$COMMIT" 2>/dev/null); echo "## merge-tree exit $? (0: clean); merged tree $MERGED"
  after=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  echo "## objects in the repository before $before, after $after; in the temporary directory: $(find "$SCRATCH/obj" -type f | wc -l | tr -d ' ')"
  for rev in "$COMMIT" "$MAIN" "$MERGED"; do
    echo "## $rev"; mkdir "$SCRATCH/t"
    GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git archive "$rev" | tar -x -C "$SCRATCH/t" && (cd "$SCRATCH/t" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | python3 -I "$SCRATCH/sweep.py" "$PATTERNS")
    rm -rf "$SCRATCH/t"
  done
} > "$OUT/sweep-and-merge.log" 2>&1

# 5. The README's two changed lines, and the hooks each CLI gets.
{
  header "wc -w README.md; grep the two lines; handoff settings claude|codex in a scratch project"
  echo "## words at $ROUND1: $(git show "$ROUND1:README.md" | wc -w | tr -d ' '); here: $(wc -w < README.md | tr -d ' ')"
  echo "## permission prompt line(s):"; grep -n 'permission prompt' README.md
  echo "## worktree line(s):"; grep -n -i 'worktree' README.md
  H="$SCRATCH/here"; mkdir "$SCRATCH/p" && cd "$SCRATCH/p" && git init -q -b main && echo x > f && git add f && GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t git commit -qm base
  "$H" init >/dev/null 2>&1
  echo "## hooks in settings claude: $("$H" settings claude 2>&1 | grep -oE '"(SessionStart|Stop|Notification)"' | sort -u | tr '\n' ' ')"
  echo "## hooks in settings codex:  $("$H" settings codex 2>&1 | grep -oE '"(SessionStart|Stop|Notification)"' | sort -u | tr '\n' ' ')"
  cd "$OLDPWD"
  echo "# alerts sent to the scratch file during this collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} > "$OUT/readme.log" 2>&1
