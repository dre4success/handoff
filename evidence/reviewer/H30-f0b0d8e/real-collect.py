# real-collect.py <scratch dir> <Claude Code's transcript folder for <scratch dir>/proj>
# Round 2: puts the real runs' saved timelines, each old transcript's continued-in line,
# and what was captured beside them, into one log. Session ids are cut to eight characters.
import json, os, re, sys, glob
D, T = sys.argv[1], sys.argv[2]
def short(s): return re.sub(r"\b([0-9a-f]{8})-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", r"\1", s)
def line(run):
    out = []; ids = []
    for l in open(os.path.join(D, run, "timeline.txt")):
        m = re.search(r"session \d \(([0-9a-f]{8})\)", l)
        if m and m.group(1) not in ids: ids.append(m.group(1))
    for f in sorted(glob.glob(os.path.join(T, "*.jsonl"))):
        if os.path.basename(f)[:8] not in ids: continue
        for x in open(f, errors="replace"):
            if '"type":"continued-in"' in x:
                try: e = json.loads(x)
                except ValueError: continue
                if e.get("type") == "continued-in": out.append("  the old transcript's line: " + short(json.dumps(e)))
    return out
def section(label, run, what, extra=()):
    print("\n## %s. %s" % (label, what))
    for l in open(os.path.join(D, run, "timeline.txt")): print(short(l.rstrip()))
    for l in line(run): print(l)
    for l in extra: print(l)
section("S1", "s1-tool", "/background while a command runs; its background session is kept for S2")
print("  the binding after (cat of the file): " + short(open(os.path.join(D, "s1-binding.txt")).read().strip()))
section("S2", "s2-resume", "claude --resume of S1's old session, which is given implementer again, has one turn and is closed")
print(); print(short(open(os.path.join(D, "s2-resume", "replay-stale.txt")).read().strip()))
section("S3", "s3-idle", "/background with the turn over and the session's listener waiting; 107 s later, a reviewer's note", [
  "23:46:46     the reviewer, in the scratch record: handoff note --as reviewer --to implementer ...: posted #2 note W2 reviewer → implementer: a note for the idle background session   (handoff said nothing else)",
  "23:47:05.468 session 2 (11d7c033): Stop: handoff hook listen      (timeline.py again, after the note: the background session was woken, had a turn, and listens again)"])
section("S4", "s4-idle-slow-end", "the same as S3, with a second SessionEnd hook in the session's settings: /bin/sleep 8")
