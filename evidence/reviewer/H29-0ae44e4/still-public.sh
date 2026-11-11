#!/usr/bin/env bash
# What CLAUDE.md drops in H29, and where the commit's tree still says it.
# Counts only: the patterns are kept outside the repository, as the name
# sweep's are, and no matching line is printed. Run as
#   WT=<worktree at the commit> BASE=<base> bash still-public.sh <patterns file> > still-public.log
set -u
WT=${WT:?}; BASE=${BASE:?}; P=${1:?patterns}
printf '# command: for each thing CLAUDE.md drops, the lines that still say it in the tree at this commit, by folder; the lines evidence/h29/docs.log repeats\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(git -C "$WT" rev-parse HEAD)"
cd "$WT"
git ls-files -z | python3 -I -c '
import re, sys, collections
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
files = [f for f in sys.stdin.read().split("\0") if f]
def area(f):
    for a in ("CLAUDE.md", "README.md", "docs/proposals/", "docs/CHANNEL.md", "docs/prompts/", "docs/reviews/", "docs/", "evidence/h29/docs.log", "evidence/h29/", "evidence/", "cmd/", "internal/"):
        if f == a or f.startswith(a): return a
    return "other"
texts = {}
for f in files:
    try: texts[f] = open(f, encoding="utf-8", errors="surrogateescape").read().split("\n")
    except OSError: pass
print(f"## {len(files)} tracked files")
for label, rx in pats:
    r = re.compile(rx); c = collections.Counter()
    for f, ls in texts.items():
        n = sum(1 for l in ls if r.search(l))
        if n: c[area(f)] += n
    print(f"{label}: {sum(c.values())} lines" + ("" if not c else " (" + ", ".join(f"{a} {n}" for a, n in sorted(c.items())) + ")"))
' "$P"
echo
rm() { git diff "$BASE" HEAD -- CLAUDE.md | grep '^-' | grep -v '^--- ' | cut -c2- | grep -v '^$'; }
removed=$(rm | wc -l | tr -d ' ')
echo "## lines H29 removes from CLAUDE.md or rewrites, blank ones aside: $removed"
n=0; while IFS= read -r l; do [ -n "$l" ] && grep -q -F -x -- "-$l" evidence/h29/docs.log && n=$((n+1)); done < <(rm)
echo "## of those, lines that evidence/h29/docs.log repeats word for word: $n"
echo "## docs.log is new in this commit: [$(git diff --name-status "$BASE" HEAD -- evidence/h29/docs.log)]"
echo "## the README's More section points to: $(sed -n '/^## More/,$p' README.md | grep -o '`docs/[^`]*`' | tr '\n' ' ')"
