# Usage: python3 -I release-sweep.py <patterns.tsv> <repo> <old-ref> <new-ref> <matches-out>
# Counts each pattern over every file of both trees (contents and file names),
# and writes every added line since old-ref that matches, for reading, to
# matches-out (outside the repository: it names what it found).
import re, subprocess, sys
pats_file, repo, old, new, out = sys.argv[1:6]
pats = [l.rstrip("\n").split("\t", 1) for l in open(pats_file) if l.strip()]
def git(*a):
    return subprocess.run(["git", "-C", repo, *a], check=True, capture_output=True).stdout
def tree(ref):
    names = git("ls-tree", "-r", "-z", "--name-only", ref).decode().split("\0")
    return [n for n in names if n]
for ref in (old, new):
    files = tree(ref)
    print(f"## {ref}: {len(files)} files")
    texts = {}
    for f in files:
        texts[f] = git("show", f"{ref}:{f}").decode("utf-8", "surrogateescape")
    for label, rx in pats:
        r = re.compile(rx)
        lines = sum(1 for t in texts.values() for l in t.split("\n") if r.search(l))
        names = sum(1 for f in files if r.search(f))
        print(f"{label}: {lines} lines, {names} file names")
diff = git("diff", "-M", old, new).decode("utf-8", "surrogateescape").split("\n")
cur = None
with open(out, "w") as w:
    print(f"## added lines in git diff {old} {new} that match:")
    for l in diff:
        if l.startswith("+++ "):
            cur = l[6:]
            continue
        if not l.startswith("+"):
            continue
        for label, rx in pats:
            if re.search(rx, l):
                w.write(f"{label}\t{cur}\t{l[1:][:300]}\n")
    w.flush()
counts = {}
for line in open(out):
    counts[line.split("\t")[0]] = counts.get(line.split("\t")[0], 0) + 1
for label, _ in pats:
    print(f"{label}: {counts.get(label, 0)} added lines")
