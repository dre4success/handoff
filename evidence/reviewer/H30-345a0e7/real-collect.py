# real-collect.py <scratch dir> <Claude Code's transcript folder for <scratch dir>/proj>
# Puts the real runs' saved timelines and what their transcripts end with into one log.
# Session ids are cut to eight characters; no path is printed.
import json, os, re, sys, glob, time
D, T = sys.argv[1], sys.argv[2]
def short(s): return re.sub(r"\b([0-9a-f]{8})-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", r"\1", s)
def lines(p): return [json.loads(l) for l in open(p, errors="replace") if l.strip()]
def ends(sid, n=6):
    p = os.path.join(T, sid + ".jsonl")
    if not os.path.exists(p): return "  (no transcript)"
    ls = lines(p); out = []
    ci = [i for i, e in enumerate(ls) if e.get("type") == "continued-in"]
    out.append("  transcript of %s: %d lines; continued-in lines: %d%s" % (sid[:8], len(ls), len(ci), "; lines after the last one: %d (%d bytes)" % (len(ls) - 1 - ci[-1], sum(len(json.dumps(e)) for e in ls[ci[-1] + 1:])) if ci else ""))
    for e in ls[ci[-1]:ci[-1] + 1] if ci else []:
        out.append("    " + short(json.dumps(e)))
    out.append("    its last lines' types: " + ", ".join(str(e.get("type")) for e in ls[-n:]))
    return "\n".join(out)
def sessions(run):
    seen = []
    for p in sorted(glob.glob(os.path.join(D, run, "in", "*")), key=os.path.getmtime):
        if p.endswith(".out"): continue
        try: sid = json.load(open(p))["session_id"]
        except Exception: continue
        if sid not in seen: seen.append(sid)
    return seen
def section(label, run, what):
    print("\n## %s. %s" % (label, what))
    for l in open(os.path.join(D, run, "timeline.txt")): print(short(l.rstrip()))
    for sid in sessions(run): print(ends(sid))
print("## R1. /background while a command runs; hookwrap-verbose.sh, which logs before it runs handoff (about 0.2 s)")
txt = open(os.path.join(D, "run0", "hooks.log")).read()
for b in txt.split("--- ")[1:]:
    head, _, rest = b.partition("\n")
    try: j = json.loads(rest.split("\n")[0] + ("" if rest.split("\n")[0].rstrip().endswith("}") else ""))
    except Exception: j = {}
    m = re.search(r'"session_id":"([0-9a-f]{8})', rest); ev = re.search(r'"hook_event_name":"(\w+)"', rest); src = re.search(r'"source":"(\w+)"', rest)
    if m: print("%s session %s: %s%s: hook %s" % (head.split()[0], m.group(1), ev.group(1) if ev else "?", " (%s)" % src.group(1) if src else "", head.split()[1]))
    else: print("%s (the same event's other hook, %s; the two wrote to the log at once)" % (head.split()[0], head.split()[1]))
    for l in rest.split("\n"):
        if l.startswith("  binding as"): print("  " + short(l))
old, new = "0c43ed37-79a9-4864-b85e-3e32b5ba8795", "814ad498-0d27-4abf-b549-f542c1394fc7"
print("# from the driver's log: the old session's process was reaped at", [l.split()[0] for l in open(os.path.join(D, "run0", "driver-tool.log")) if "has ended" in l][0])
print("# from the new session's transcript: what its SessionStart brief hook printed, and what woke it later")
for e in lines(os.path.join(T, new + ".jsonl")):
    s = json.dumps(e); ts = (e.get("timestamp") or "")[11:23]
    if e.get("type") == "attachment" and ("has no role" in s or "you are implementer" in s) and ts > "22:50:30":
        print("  %s %s: the brief hook's output: %s" % (ts, e.get("type"), "YOU ARE IMPLEMENTER" if "you are implementer" in s else "NO ROLE"))
    if ts > "22:51:40" and e.get("type") in ("user", "system") and ("new for implementer" in s or e.get("subtype") == "stop_hook_summary"):
        print("  %s %s %s%s" % (ts, e.get("type"), e.get("subtype", ""), ": carries \"handoff: new for implementer: #1 note W1 reviewer → implementer\"" if "new for implementer" in s else ""))
print("# the reviewer's note was posted at 22:51:51 (handoff note, in the scratch record); the session's next Stop hook ran at 22:52:09.895")
print(ends(old)); print(ends(new, 4))
section("R2", "tool1", "/background while a command runs")
section("R3", "tool2", "the same again")
section("R4", "idle1", "/background with the turn over and the session's listener waiting")
section("R5", "idle2", "the same again")
section("R6", "tool3", "/background while a command runs, a third time; its background session is kept for R8")
section("R7", "fork1", "/fork with the turn over; the old session is closed three minutes later")
section("R8", "resume1", "claude --resume of R6's old session, which is given implementer again, has one turn and is closed")
print()
print(short(open(os.path.join(D, "resume1", "replay-stale.txt")).read().strip()))
