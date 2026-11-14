# timeline.py <scratch dir> <driver log>: one run's hooks in order, from the files hookwrap.sh kept
import json, os, sys, glob, time, re
D, drv = sys.argv[1], sys.argv[2]
def ts(t): return time.strftime("%H:%M:%S", time.gmtime(t)) + ".%03d" % (t * 1000 % 1000)
ev = []
names = {}
for p in glob.glob(D + "/in/*"):
    if p.endswith(".out"): continue
    st = os.stat(p); kind = p.rsplit(".", 1)[1]
    try: j = json.load(open(p))
    except Exception: continue
    sid = j.get("session_id", "?")
    names.setdefault(sid, None)
    what = "%s%s: handoff hook %s" % (j.get("hook_event_name"), " (%s)" % j["source"] if j.get("source") else (" (%s)" % j["reason"] if j.get("reason") else ""), kind) if kind != "end" else "SessionEnd (%s)" % j.get("reason")
    out = ""
    if os.path.exists(p + ".out"):
        o = open(p + ".out").read()
        out = "  -> brief says: " + ("YOU ARE " + re.search(r"you are (\S+)", o).group(1).upper() if "you are " in o else "this session has NO ROLE" if "has no role" in o else repr(o[:60]))
    ev.append((st.st_mtime, sid, what + out))
for l in open(drv):
    m = re.match(r"(\d\d):(\d\d):(\d\d)\.(\d\d\d) (.*)", l)
    if m and ("has ended" in l or "pressed Enter" in l or "handoff role" in l or "listener" in l or "binding" in l):
        day = time.gmtime(min(e[0] for e in ev)) if ev else time.gmtime()
        import calendar
        t = calendar.timegm((day.tm_year, day.tm_mon, day.tm_mday, int(m.group(1)), int(m.group(2)), int(m.group(3)))) + int(m.group(4)) / 1000
        ev.append((t, None, "DRIVER: " + m.group(5)))
for p in glob.glob(D + "/rec/.handoff/sessions/*.json"):
    b = json.load(open(p)); st = os.stat(p)
    ev.append((st.st_mtime, b["session"], "the binding was written: this session has %s (process %s)" % (b["role"], b.get("pid"))))
ev.sort(key=lambda e: e[0])
order = []
for _, sid, _ in ev:
    if sid and sid not in order: order.append(sid)
label = {sid: "session %d (%s)" % (i + 1, sid[:8]) for i, sid in enumerate(order)}
for t, sid, what in ev:
    for full in order: what = what.replace(full, label[full])
    print(ts(t), (label[sid] + ": " if sid else "") + what)
