# on-this-mac.py: read-only. What the transcripts on this Mac say about "the cwd of the
# first line that names one". Prints counts and sizes only: no path, no id, no text.
import glob, json, os, sys, collections
home = os.path.expanduser("~")
def first_cwd(path, cap_lines=None):
    # (line number of the first line with a top-level or payload cwd, bytes before it, the cwd, longest line before it)
    n = 0; seen = 0; longest = 0
    try:
        with open(path, "rb") as f:
            for raw in f:
                n += 1
                longest = max(longest, len(raw))
                if b'"cwd"' in raw:
                    try: e = json.loads(raw)
                    except ValueError: e = {}
                    c = e.get("cwd") or (e.get("payload") or {}).get("cwd") if isinstance(e, dict) else None
                    if c: return n, seen, c, longest, e
                seen += len(raw)
                if cap_lines and n >= cap_lines: break
    except OSError:
        return None
    return n, seen, None, longest, None
print("## Claude Code: ~/.claude/projects/*/<id>.jsonl")
files = glob.glob(home + "/.claude/projects/*/*.jsonl")
dirs = len(glob.glob(home + "/.claude/projects/*/"))
print("project folders: %d; transcripts: %d" % (dirs, len(files)))
none = 0; lines = []; byts = []; later_diff = 0; byid = collections.defaultdict(list); types = collections.Counter(); folder_mismatch = 0
for p in files:
    r = first_cwd(p)
    if r is None: continue
    n, seen, c, longest, e = r
    byid[os.path.basename(p)].append(p)
    if c is None: none += 1; continue
    lines.append(n); byts.append(seen); types[e.get("type")] += 1
    # is the transcript filed under the folder named after its first cwd?
    enc = "".join(ch if ch.isalnum() else "-" for ch in c)
    if os.path.basename(os.path.dirname(p)) != enc: folder_mismatch += 1
lines.sort(); byts.sort()
def q(a, f): return a[min(len(a) - 1, int(len(a) * f))] if a else None
print("with no cwd on any line: %d (their sizes: %s)" % (none, "all under 64 KiB" if all(os.path.getsize(p) < 65536 for p in files if (first_cwd(p) or (0,0,1))[2] is None) else "some larger"))
print("the first line naming a cwd is line: median %s, 99%% %s, most %s; over 200: %d" % (q(lines, .5), q(lines, .99), lines[-1] if lines else None, sum(1 for x in lines if x > 200)))
print("bytes before it: median %s, 99%% %s, most %s; over 4 MiB: %d" % (q(byts, .5), q(byts, .99), byts[-1] if byts else None, sum(1 for x in byts if x > 4 << 20)))
print("type of that line: %s" % dict(types.most_common(6)))
print("transcripts not filed under the folder their first cwd encodes to: %d" % folder_mismatch)
print("ids with more than one transcript: %d" % sum(1 for v in byid.values() if len(v) > 1))
# how many transcripts name more than one cwd at all (the session moved)
moved = 0; back = 0
for p in files[:4000]:
    seen = []
    try:
        for raw in open(p, "rb"):
            if b'"cwd"' not in raw[:2000] and b'"cwd"' not in raw: continue
            try: c = json.loads(raw).get("cwd")
            except Exception: c = None
            if c and (not seen or seen[-1] != c): seen.append(c)
    except OSError: continue
    if len(set(seen)) > 1: moved += 1
print("transcripts naming more than one cwd: %d" % moved)
print()
print("## Codex: ~/.codex/sessions/<y>/<m>/<d>/rollout-<time>-<id>.jsonl")
files = glob.glob(home + "/.codex/sessions/*/*/*/rollout-*.jsonl")
days = len(glob.glob(home + "/.codex/sessions/*/*/*/"))
print("day folders: %d; rollouts: %d" % (days, len(files)))
first_types = collections.Counter(); sizes = []; nocwd = 0; metas = collections.Counter(); ids = collections.Counter(); idmatch = 0; differs = 0; over = 0
for p in files:
    try:
        with open(p, "rb") as f: raw = f.readline()
    except OSError: continue
    sizes.append(len(raw))
    try: e = json.loads(raw)
    except ValueError: e = {}
    first_types[e.get("type")] += 1
    pl = e.get("payload") or {}
    if not pl.get("cwd"): nocwd += 1
    sid = pl.get("id") or ""
    if sid and os.path.basename(p).endswith("-" + sid + ".jsonl"): idmatch += 1
    ids[os.path.basename(p)[-41:]] += 1
    # count session_meta lines, and whether a later turn_context names another cwd
    m = 0; cw = set()
    try:
        for raw2 in open(p, "rb"):
            if raw2.startswith(b'{"timestamp"') or raw2.startswith(b'{"type"'):
                head = raw2[:300]
                if b'"session_meta"' in head: m += 1
                if b'"turn_context"' in head:
                    try: cw.add((json.loads(raw2).get("payload") or {}).get("cwd"))
                    except Exception: pass
    except OSError: pass
    metas[m] += 1
    if pl.get("cwd") and cw and cw != {pl.get("cwd")}: differs += 1
sizes.sort()
print("type of the first line: %s" % dict(first_types))
print("first line with no payload.cwd: %d" % nocwd)
print("first line's size in bytes: median %s, most %s; over 64 KiB: %d; over 4 MiB: %d" % (q(sizes, .5), sizes[-1] if sizes else None, sum(1 for x in sizes if x > 65536), sum(1 for x in sizes if x > 4 << 20)))
print("file named after the id in its first line: %d of %d" % (idmatch, len(files)))
print("session_meta lines per rollout: %s" % dict(sorted(metas.items())))
print("ids with more than one rollout: %d" % sum(1 for v in ids.values() if v > 1))
print("rollouts whose turns name a cwd other than the first line's: %d" % differs)
print()
print("## Claude Code: a continued-in line (/background) whose two transcripts are both here")
files = {os.path.basename(p)[:-6]: p for p in glob.glob(home + "/.claude/projects/*/*.jsonl")}
def uu(ls):
    out = []
    for x in ls:
        try: j = json.loads(x)
        except ValueError: continue
        if isinstance(j, dict) and j.get("uuid"): out.append(j["uuid"])
    return out
def fc(ls):
    for x in ls:
        try: j = json.loads(x)
        except ValueError: continue
        if isinstance(j, dict) and j.get("cwd"): return j["cwd"]
pairs = 0
for sid, p in sorted(files.items()):
    try: lines = open(p, errors="replace").read().splitlines()
    except OSError: continue
    for l in lines[-400:]:
        if '"continued-in"' not in l: continue
        try: e = json.loads(l)
        except ValueError: continue
        new = e.get("continuedInSessionId")
        if e.get("type") != "continued-in" or new not in files: continue
        pairs += 1
        nl = open(files[new], errors="replace").read().splitlines()
        a, b = uu(lines), uu(nl)
        print("  old %d lines, new %d lines; lines of the old one in the new one (by uuid): %d; the new one begins with the old one's first message: %s; same first cwd: %s" % (len(lines), len(nl), len(set(a) & set(b)), b[:1] == a[:1], fc(lines) == fc(nl)))
print("pairs: %d" % pairs)
