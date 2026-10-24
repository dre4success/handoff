#!/usr/bin/env bash
# Collects the H15 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h15/collect.sh <patterns file>
# The patterns file is H14's, kept outside the repository since it names what
# was removed. AUTHOR_NAME and AUTHOR_EMAIL are what the public commit will
# carry; the log shows only whether the address is GitHub's noreply form.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
PATTERNS=$(cd "$(dirname "$PATTERNS")" && pwd -P)/$(basename "$PATTERNS")
ROOT=$(pwd -P)
OUT=${OUT:-evidence/h15}
COMMIT=$(git rev-parse HEAD)
SCRUB=$(git log --format=%h -1 --grep="^refactor: name no person in the reviewer")
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
[ -n "$SCRUB" ] || { echo "no scrub commit found"; exit 1; }

# The sweep of H14's evidence/h14/collect.sh, over the files listed on stdin.
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

# 1. The sweep, before the renaming and here.
{
  header "H14's sweep of every tracked file's contents and name, at $SCRUB^ and here"
  echo "## at $(git rev-parse --short "$SCRUB^"), main after H14"
  (mkdir "$SCRATCH/before" && git archive "$SCRUB^" | tar -x -C "$SCRATCH/before" && cd "$SCRATCH/before" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | sweep)
  echo "## here"
  git ls-files -z | sweep
} > "$OUT/sweep.log" 2>&1

# 2. The renaming changes words only, and only in the reviewer's evidence
# collected after H14's renaming.
{
  header "git diff --numstat, --name-only and --word-diff=porcelain $SCRUB^ $SCRUB"
  git diff --numstat "$SCRUB^" "$SCRUB" | awk '{a+=$1; d+=$2; n++} END {print n " files; " a " lines in, " d " lines out"}'
  echo "## the files"
  git diff --name-only "$SCRUB^" "$SCRUB"
  echo "## each word or phrase put in, with how often:"
  git diff --word-diff=porcelain "$SCRUB^" "$SCRUB" | grep '^+' | grep -v '^+++' | sed 's/^+//' | sort | uniq -c | sort -rn
} > "$OUT/scrub-diff.log" 2>&1

# 3. No hash in the tree is of bytes the renaming changed.
{
  header "python3 -I evidence/h14/stale-hashes.py $SCRUB^ $SCRUB $SCRUB^ $COMMIT"
  python3 -I evidence/h14/stale-hashes.py "$SCRUB^" "$SCRUB" "$SCRUB^" "$COMMIT"
  echo "# exit: $?"
} > "$OUT/stale-hashes.log" 2>&1

# 4. A dry run of the public export: this commit's tracked files, as the one
# commit of a new repository, made here and never pushed.
{
  header "git archive HEAD into a new repository; one commit; the sweep there; no remote"
  mkdir "$SCRATCH/export" && git archive HEAD | tar -x -C "$SCRATCH/export"
  (cd "$SCRATCH/export" && git init -q -b main && git add -A &&
    GIT_AUTHOR_NAME="${AUTHOR_NAME:?}" GIT_COMMITTER_NAME="$AUTHOR_NAME" GIT_AUTHOR_EMAIL="${AUTHOR_EMAIL:?}" GIT_COMMITTER_EMAIL="$AUTHOR_EMAIL" \
      git commit -q -m "handoff: two coding agents hand work to each other through a record the operator can read" &&
    echo "## commits: $(git rev-list --count HEAD)"
    echo "## files: $(git ls-files | wc -l | tr -d ' '), the same as here: $([ "$(git ls-files)" = "$(git -C "$ROOT" ls-files)" ] && echo yes || echo no)"
    echo "## trees match: $([ "$(git rev-parse HEAD^{tree})" = "$(git -C "$ROOT" rev-parse HEAD^{tree})" ] && echo yes || echo no)"
    echo "## author and committer email are GitHub noreply: $(git log -1 --format='%ae %ce' | grep -cE '^[0-9]+\+[^ ]+@users\.noreply\.github\.com [0-9]+\+[^ ]+@users\.noreply\.github\.com$')"
    echo "## remotes: [$(git remote)]"
    echo "## the sweep, in the export"
    git ls-files -z | sweep)
} > "$OUT/export-preview.log" 2>&1
