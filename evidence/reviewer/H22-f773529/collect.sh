#!/usr/bin/env bash
# The reviewer's collector for H22 (f773529): build, vet, gofmt, dependencies,
# the commits since the base, a read-only merge dry run against main, and
# H14's sweep over this commit, main, the merged tree and this folder. Run as
#   W=<worktree at f773529> OUT=<this dir> bash collect.sh <patterns file>
# The patterns file is H14's, outside the repository. Nothing is written to
# the repository: merge-tree's objects go to a scratch object directory.
set -u
W=${W:?worktree}; OUT=${OUT:?out}; PATTERNS=${1:?patterns}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
cd "$W"
COMMIT=$(git rev-parse HEAD); MAIN=$(git rev-parse main); BASE=b6d164ec671b3b8cf2077e24947a5e496ce071f3
export LC_ALL=C GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
header() { printf '# command: %s\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$COMMIT"; }
T=$(mktemp -d); T=$(cd "$T" && pwd -P); trap 'rm -rf "$T"' EXIT
clean() { sed "s|$T|<scratch>|g; s|$W|<worktree>|g; s|$HOME|~|g"; }

{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod; go list -deps; commits and diffstat since the base $BASE"
  go version
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base:"; git log --oneline --no-decorate "$BASE..HEAD"
  echo "## diffstat:"; git diff --stat "$BASE..HEAD"
  echo "## main is now $MAIN; commits on main since the base:"; git log --oneline --no-decorate "$BASE..$MAIN"
  echo "## what main adds since the base touches:"; git diff --stat "$BASE..$MAIN"
} 2>&1 | clean > "$OUT/build-vet-deps.log"

sweep() { # dir: H14's sweep over every file under dir
  (cd "$1" && find . -type f -not -path './.git/*' -print0 | python3 -I -c '
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
' "$PATTERNS")
}

{
  header "git merge-tree --write-tree main HEAD, objects to a scratch directory; H14's sweep over HEAD, main, the merged tree and the reviewer's evidence folder"
  COMMON=$(git rev-parse --path-format=absolute --git-common-dir)
  echo "## loose objects in the repository before:"; git count-objects -v | grep -E '^(count|in-pack):'
  mkdir -p "$T/objects"
  TREE=$(GIT_OBJECT_DIRECTORY="$T/objects" GIT_ALTERNATE_OBJECT_DIRECTORIES="$COMMON/objects" git merge-tree --write-tree "$MAIN" HEAD); code=$?
  echo "## merge-tree of main ($MAIN) and HEAD: exit $code"; echo "$TREE"
  echo "## loose objects in the repository after:"; git count-objects -v | grep -E '^(count|in-pack):'
  for rev in HEAD main; do mkdir -p "$T/$rev" && git archive "$rev" | tar -x -C "$T/$rev"; done
  mkdir -p "$T/merged" && GIT_OBJECT_DIRECTORY="$T/objects" GIT_ALTERNATE_OBJECT_DIRECTORIES="$COMMON/objects" git archive "$(echo "$TREE" | head -1)" | tar -x -C "$T/merged"
  echo "## files that differ between main and the merged tree:"; (cd "$T" && diff -rq main merged | sed 's|^Only in merged: |+ |; s|^Files main/\(.*\) and merged/.* differ|M \1|; s|^Only in main: |- |')
  echo "## the merged tree's evidence/h22 and evidence/reviewer folders:"; (cd "$T/merged" && ls evidence/h22 evidence/reviewer)
  for d in HEAD main merged; do echo "## sweep of $d ($( [ $d = HEAD ] && echo "$COMMIT" || { [ $d = main ] && echo "$MAIN" || echo "tree $(echo "$TREE" | head -1)"; }))"; sweep "$T/$d"; done
  echo "## sweep of the reviewer's evidence folder, $OUT"; sweep "$OUT"
} 2>&1 | clean > "$OUT/merge-and-sweep.log"
echo "collect done"
