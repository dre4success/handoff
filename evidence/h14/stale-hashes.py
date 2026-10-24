"""Finds every hash, quoted anywhere in the tree, of bytes the renaming changed.

usage: python3 -I stale-hashes.py <before> <after> <rev to search>...

For each file the renaming changed, it takes the hashes of the file as it was
(sha256 and git's blob id) and of each line it changed (sha256 of the line,
with and without its newline, and of any {...} in it). Then it looks for each
one, whole or as a prefix of 12 or more hex digits, in every file of each
searched revision. It prints what it found, never the changed text itself.
"""
import hashlib, re, subprocess, sys

before, after, *search = sys.argv[1:]


def git(*a):
    return subprocess.run(["git", *a], capture_output=True, check=True).stdout


digests = {}
# -M10%: the renamed files changed enough that git pairs only two at its default.
fields = git("diff", "-M10%", "--name-status", "-z", before, after).split(b"\0")
i, pairs = 0, []
while i < len(fields) - 1:
    st = fields[i].decode()
    if st.startswith("R"):
        pairs.append((fields[i + 1].decode(), fields[i + 2].decode()))
        i += 3
    else:
        if st != "M":
            sys.exit(f"unpaired: {st} {fields[i + 1].decode()}")
        pairs.append((fields[i + 1].decode(), fields[i + 1].decode()))
        i += 2
for old_path, new_path in pairs:
    old = git("show", f"{before}:{old_path}")
    new = git("show", f"{after}:{new_path}")
    if old == new:
        continue
    digests[hashlib.sha256(old).hexdigest()] = f"the sha256 of {old_path}"
    digests[git("rev-parse", f"{before}:{old_path}").strip().decode()] = f"the blob id of {old_path}"
    kept = set(new.split(b"\n"))
    for n, line in enumerate(old.split(b"\n"), 1):
        if line in kept:
            continue
        for c in {line, line.rstrip(b"\r"), *(m.group(0) for m in re.finditer(rb"\{.*\}", line))}:
            digests[hashlib.sha256(c).hexdigest()] = f"the sha256 of {old_path} line {n}"
            digests[hashlib.sha256(c + b"\n").hexdigest()] = f"the sha256 of {old_path} line {n}"
print(f"{len(pairs)} files changed; {len(digests)} hashes of the bytes that changed")

for rev in search:
    names = [f for f in git("ls-tree", "-r", "--name-only", "-z", rev).decode().split("\0") if f]
    found = {}
    for f in names:
        for m in re.finditer(rb"[0-9a-f]{12,64}", git("show", f"{rev}:{f}")):
            h = m.group(0).decode()
            for d, what in digests.items():
                if d.startswith(h):
                    found.setdefault((f, what), set()).add(h[:12])
    print(f"## at {git('rev-parse', '--short', rev).decode().strip()}: {len(names)} files searched, {len(found)} quote one")
    for (f, what), hs in sorted(found.items()):
        print(f"{f} quotes {what} ({', '.join(sorted(hs))})")
