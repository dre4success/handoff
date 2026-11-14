# collect-real.py <scratch dir> <Claude Code's projects folder>: puts the real runs'
# timelines, the reviewer's notes, the old transcript's continued-in line and what the new
# session was told, into one log. Session ids are cut to eight characters; no path is printed.
import json, os, re, sys, glob
W, P = sys.argv[1], sys.argv[2]
def short(s): return re.sub(r"\b([0-9a-f]{8})-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", r"\1", s)
runs = [("run-idle-28b27b9", "R4's case, at 28b27b9: /background with the turn over and the listener waiting"),
        ("run-idle", "R4's case, at 9dbb4ff: /background with the turn over and the listener waiting"),
        ("run-tool", "R2's case, at 9dbb4ff: /background typed while a 45 s command runs")]
for run, what in runs:
    D = os.path.join(W, run)
    print("\n## %s: %s" % (run, what))
    for l in open(os.path.join(D, "timeline.txt")): print(short(l.rstrip()))
    if os.path.exists(os.path.join(D, "note.log")):
        for l in open(os.path.join(D, "note.log")): print(short(l.rstrip()) + "   (anything handoff said is on this line)")
    T = glob.glob(os.path.join(P, "*h30r2-" + run.split("-28b")[0] + "-proj"))
    ids = []
    for l in open(os.path.join(D, "timeline.txt")):
        m = re.search(r"session \d \(([0-9a-f]{8})\)", l)
        if m and m.group(1) not in ids: ids.append(m.group(1))
    for f in sorted(glob.glob(os.path.join(T[0], "*.jsonl"))) if T else []:
        sid = os.path.basename(f)[:8]
        if sid not in ids: continue
        ls = [json.loads(x) for x in open(f) if x.strip()]
        ci = [e for e in ls if e.get("type") == "continued-in"]
        print("  transcript of %s: %d lines; its continued-in line: %s" % (sid, len(ls), short(json.dumps(ci[-1])) if ci else "none"))
        if ids.index(sid) == 1:
            for e in ls:
                s = json.dumps(e); ts = (e.get("timestamp") or "")[11:23]
                if e.get("type") in ("user", "attachment") and ts and ("you are implementer" in s or "has no role" in s or "new for implementer" in s):
                    print("    %s %s: %s" % (ts, e.get("type"), "the brief: YOU ARE IMPLEMENTER" if "you are implementer" in s else "the brief: NO ROLE" if "has no role" in s else "handoff: new for implementer: #1 note W1 reviewer"))
