# on-this-mac.py: read-only. What round 2 reads on this Mac. Prints counts and sizes only: no path, no id, no text.
import glob, json, os, subprocess, sys
home = os.path.expanduser("~")
print("## Codex: how far from a rollout's end its last turn_context is")
files = glob.glob(home + "/.codex/sessions/*/*/*/rollout-*.jsonl")
far = []; none = 0; spaced = 0; sizes = []
for p in files:
    size = os.path.getsize(p); sizes.append(size); off = 0; last = None; exact = 0; parsed = 0
    with open(p, "rb") as f:
        for raw in f:
            if b"turn_context" in raw[:200]:
                try: e = json.loads(raw)
                except ValueError: e = {}
                if e.get("type") == "turn_context":
                    parsed += 1; last = off
                    if b'"type":"turn_context"' in raw: exact += 1
            off += len(raw)
    if last is None: none += 1
    else: far.append(size - last)
    spaced += parsed - exact
far.sort()
print("rollouts: %d; sizes in bytes: least %s, most %s" % (len(files), min(sizes) if sizes else "-", max(sizes) if sizes else "-"))
print("with no turn_context: %d" % none)
print("bytes from the last turn_context to the end: least %s, most %s; over 1 MiB: %d; over 16 MiB: %d" % (far[0] if far else "-", far[-1] if far else "-", sum(1 for x in far if x > 1 << 20), sum(1 for x in far if x > 16 << 20)))
print("turn_context lines not written exactly as \"type\":\"turn_context\": %d" % spaced)
print()
print("## Claude Code: is each running session's process in the session's folder? (claude agents --json, then lsof)")
try:
    d = json.loads(subprocess.run(["claude", "agents", "--json"], capture_output=True, text=True).stdout)
except Exception: d = []
n = same = nopid = 0; kinds = {}
for a in d:
    pid = a.get("pid")
    if not pid: nopid += 1; continue
    out = subprocess.run(["/usr/sbin/lsof", "-a", "-p", str(pid), "-d", "cwd", "-Fn"], capture_output=True, text=True).stdout
    cwd = next((l[1:] for l in out.splitlines() if l.startswith("n")), None)
    ok = cwd is not None and os.path.realpath(cwd) == os.path.realpath(str(a.get("cwd")))
    n += 1; same += ok; k = "%s, %s" % (a.get("kind"), "same" if ok else "different"); kinds[k] = kinds.get(k, 0) + 1
print("sessions listed with a process: %d; process in the folder the listing names: %d; listed with no process: %d; by kind: %s" % (n, same, nopid, kinds))
