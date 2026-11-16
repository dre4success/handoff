# attach-probe.py <scratch dir> <job id> <session id>: `claude attach <job id>` through a pseudo-terminal, to
# give the idle background session of run 5 one turn, so that it writes its transcript. Then the folders
# its transcript names, and this build's `handoff role implementer` as that session from P/proj.
import os, pty, re, select, sys, time, signal, json, subprocess, glob
D, job, sid = sys.argv[1], sys.argv[2], sys.argv[3]
P, proj, LOG = D + "/P", D + "/P/proj", D + "/log/runs.jsonl"
env = {k: os.environ[k] for k in ("HOME", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", PATH=D + "/shim:" + os.environ["PATH"], HANDOFF_TEST_NOTIFY=D + "/alerts.txt", HANDOFF_TEST_AWAKE=D + "/awake.txt")
def sc(s): return str(s).replace(proj, "<P/proj>").replace(P, "<P>")
def stamp(): return time.strftime("%H:%M:%S", time.gmtime())
n0 = sum(1 for _ in open(LOG))
pid, fd = pty.fork()
if pid == 0:
    os.chdir(P); os.execvpe("claude", ["claude", "attach", job], env)
screen = b""
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        try: r, _, _ = select.select([fd], [], [], 0.05)
        except (OSError, ValueError): time.sleep(0.05); continue
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: time.sleep(0.05)
pump(8)
for ch in "Reply with the word DELTA only.": os.write(fd, ch.encode()); pump(0.02)
pump(0.6); os.write(fd, b"\r")
t = time.time(); stop = None
while time.time() - t < 90 and not stop:
    pump(0.5)
    for l in open(LOG).read().splitlines()[n0:]:
        r = json.loads(l); i = r.get("in") or {}
        if i.get("hook_event_name") == "Stop" and i.get("session_id") == sid: stop = r
print(stamp(), "## attached to the background session and gave it one turn; its Stop hook ran in:", sc(stop["pwd"]) if stop else "(no Stop seen)", "| CLAUDE_PROJECT_DIR=" + sc(stop["pdir"]) if stop else "")
pump(2)
try: os.kill(pid, signal.SIGTERM)
except OSError: pass
time.sleep(1)
try: os.waitpid(pid, os.WNOHANG)
except OSError: pass
for p in sorted(glob.glob(os.environ["HOME"] + "/.claude/projects/*/" + sid + ".jsonl")):
    n = 0; first = None; cw = []; firstmsg = None
    for l in open(p, errors="replace"):
        n += 1
        try: e = json.loads(l)
        except ValueError: continue
        c = e.get("cwd")
        if c and first is None: first = (n, c, e.get("type"), (e.get("attachment") or {}).get("type") or e.get("subtype"))
        if c and (not cw or cw[-1] != c): cw.append(c)
    print(stamp(), "## its transcript: %d lines; the first cwd is on line %s (a %s line%s): %s; the folders it names, in order: %s" % (n, first[0] if first else "-", first[2] if first else "-", ", " + str(first[3]) if first and first[3] else "", sc(first[1]) if first else "-", " > ".join(sc(c) for c in cw)))
r = subprocess.run([D + "/../bin31/handoff", "role", "implementer"], cwd=proj, capture_output=True, text=True, env=dict(env, CLAUDE_CODE_SESSION_ID=sid, CLAUDE_PID=str(os.getpid())))
w = [l for l in r.stdout.splitlines() if l.startswith("handoff: warning")]
print(stamp(), "## this build's handoff role implementer, as that session, from P/proj:", (r.stdout.splitlines() or [r.stderr.strip()])[0], "|", "WARNED" if w else "no warning")
