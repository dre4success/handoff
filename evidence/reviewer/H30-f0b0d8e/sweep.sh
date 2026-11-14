#!/usr/bin/env bash
# The reviewer's name sweep for H30 round 2 (f0b0d8e). H14's patterns file was in an
# earlier session's scratch folder and is gone, so these patterns are rebuilt
# from §17 rows 77 to 79 and kept outside the repository, since they name
# what was removed. Two are wider than H14's (any home path, any email
# address), so the count at main, which H14's sweep found clean, is the
# baseline: the commit must add nothing to it. Run as
#   R=<repository> C=<commit> OUT=<reviewer's evidence folder> bash sweep.sh <patterns file>
set -u
R=${R:?}; C=${C:?}; OUT=${OUT:?}; PATTERNS=${1:?patterns}
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
MAIN=$(git -C "$R" rev-parse main)
mkdir -p "$T/main" "$T/commit" "$T/merged" "$T/objects"
git -C "$R" archive "$MAIN" | tar -x -C "$T/main"
git -C "$R" archive "$C" | tar -x -C "$T/commit"
sweep() { (cd "$1" && find . -type f -not -path './.git/*' -print0 | python3 -I -c '
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
' "$PATTERNS"); }
printf '# command: git merge-tree --write-tree main <commit>, objects to a scratch directory; the name sweep over main, the commit, the merged tree and the reviewer'"'"'s evidence folder\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$C"
echo "## main is $MAIN; the request's base is $(git -C "$R" rev-parse 2336ecd); commits on main since the base:"
git -C "$R" log --oneline --no-decorate 2336ecd.."$MAIN"; git -C "$R" diff --stat 2336ecd "$MAIN" | tail -2
echo "## loose objects in the repository before: $(git -C "$R" count-objects | cut -d' ' -f1)"
TREE=$(GIT_OBJECT_DIRECTORY="$T/objects" GIT_ALTERNATE_OBJECT_DIRECTORIES="$(git -C "$R" rev-parse --absolute-git-dir | sed 's|/worktrees/.*||')/objects" git -C "$R" merge-tree --write-tree "$MAIN" "$C"); echo "## merge-tree of main and the commit: exit $?"; echo "$TREE"
echo "## loose objects in the repository after: $(git -C "$R" count-objects | cut -d' ' -f1)"
GIT_OBJECT_DIRECTORY="$T/objects" GIT_ALTERNATE_OBJECT_DIRECTORIES="$(git -C "$R" rev-parse --absolute-git-dir)/objects" git -C "$R" archive "$(echo "$TREE" | head -1)" | tar -x -C "$T/merged"
echo "## files that differ between main and the merged tree:"; diff -rq "$T/main" "$T/merged" | sed "s|$T/||g"
echo "## files that differ between the commit and the merged tree:"; diff -rq "$T/commit" "$T/merged" | sed "s|$T/||g"
for d in main commit merged; do echo; echo "## sweep of $d"; sweep "$T/$d"; done
echo; echo "## added lines in git diff 2336ecd <commit> that match any pattern:"
git -C "$R" diff 2336ecd "$C" | grep '^+' | grep -v '^+++' | python3 -I -c '
import re, sys
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
text = sys.stdin.read().split("\n")
for label, rx in pats:
    r = re.compile(rx)
    print(f"{label}: {sum(1 for l in text if r.search(l))} lines")
' "$PATTERNS"
echo; echo "## sweep of the reviewer's evidence folder"; sweep "$OUT"
