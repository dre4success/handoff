"""H12 live check: two real, throwaway Claude Code sessions (Haiku) in one
scratch project, each given its role in chat, as the operator would.

usage: python3 -I live-two-claude.py <project> <handoff binary> <alerts file> <log file>  (live-two-claude.sh runs it)

The project has a handoff record (roles implementer, reviewer) and the
settings `handoff settings claude` printed with no --as, so no role is fixed.
Alerts go to the alerts file (HANDOFF_TEST_NOTIFY), never to the screen.
"""
import glob, json, os, pty, re, select, shutil, signal, subprocess, sys, time

proj, H, alerts, logpath = sys.argv[1:5]
log = open(logpath, "a", buffering=1)
def note(msg):
    log.write(f"{time.strftime('%H:%M:%S', time.gmtime())}Z {msg}\n")

env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=alerts)

def handoff(*args):
    r = subprocess.run([H, *args], cwd=proj, env=dict(env), capture_output=True, text=True, timeout=60)
    return r.returncode, r.stdout, r.stderr

class Session:
    def __init__(self, name, prompt):
        self.name, self.screen = name, b""
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            os.chdir(proj)
            os.execvpe("claude", ["claude", "--model", "haiku", prompt], env)
        note(f"{name}: claude started, pid {self.pid}")
    def read(self):
        r, _, _ = select.select([self.fd], [], [], 0)
        if r:
            try:
                self.screen = (self.screen + os.read(self.fd, 65536))[-200000:]
            except OSError:
                pass
    def plain(self):
        return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", self.screen).decode(errors="replace").replace(" ", "")
    def stop(self):
        os.kill(self.pid, signal.SIGTERM)
        pump(2)
        try:
            os.kill(self.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        os.waitpid(self.pid, 0)

sessions = []
def pump(secs):
    end = time.time() + secs
    while time.time() < end:
        for s in sessions:
            s.read()
        time.sleep(0.2)

# Claude Code keeps transcripts under the project's real path, every
# character but a letter or digit made "-", one file per session id.
transcripts = os.path.expanduser("~/.claude/projects/" + re.sub(r"[^A-Za-z0-9]", "-", os.path.realpath(proj)))
def records(sid):
    out = []
    for line in open(os.path.join(transcripts, sid + ".jsonl"), errors="replace") if sid else []:
        try:
            out.append(json.loads(line))
        except ValueError:
            pass
    return out
def content(sid, kind):
    return [c for d in records(sid) if d.get("type") == "assistant"
            for c in (d.get("message", {}).get("content") or []) if isinstance(c, dict) and c.get("type") == kind]
def texts(sid):
    return [c["text"] for c in content(sid, "text")]
def commands(sid):
    return [c.get("input", {}).get("command", "") for c in content(sid, "tool_use")]

def bindings():
    out = {}
    for p in glob.glob(os.path.join(proj, ".handoff", "sessions", "*.json")):
        try:
            b = json.load(open(p))
            out[b["role"]] = b
        except (ValueError, KeyError):
            pass
    return out
def sid(role):
    return bindings().get(role, {}).get("session", "")

def until(what, test, secs):
    end = time.time() + secs
    while time.time() < end:
        pump(1)
        if test():
            note(f"seen: {what}")
            return True
    note(f"NOT seen within {secs}s: {what}")
    return False

def trust(s):
    if until(f"{s.name}: the trust dialog or the prompt", lambda: "Entertoconfirm" in s.plain() or "Itrustthisfolder" in s.plain() or sid(s.role), 30) and "Itrustthisfolder" in s.plain():
        note(f"{s.name}: trust dialog: choosing 'Yes, I trust this folder'")
        os.write(s.fd, b"\x1b[B"); pump(0.8); os.write(s.fd, b"\r")

ask = "The operator gives you your handoff role: {role}. Run exactly: {h} role {role}  Then reply READY-{tag} and nothing else."
for name, role, tag in (("opus-side", "implementer", "A"), ("fable-side", "reviewer", "B")):
    s = Session(name, ask.format(role=role, h=H, tag=tag))
    s.role = role
    sessions.append(s)
    trust(s)
    until(f"{name}: a binding for {role}", lambda: sid(role) != "", 90)
    until(f"{name}: READY-{tag}", lambda: any(f"READY-{tag}" in t for t in texts(sid(role))), 60)
pump(5)

b = bindings()
note("bindings: " + json.dumps({r: {"session": x["session"][:8] + "…", "pid is a claude process": x.get("pid", 0) in [s.pid for s in sessions]} for r, x in b.items()}))
note("the two sessions differ: " + str(len({x["session"] for x in b.values()}) == 2))
for role in ("implementer", "reviewer"):
    p = os.path.join(proj, ".handoff", "presence", role + ".json")
    note(f"listening for {role}: " + (open(p).read().strip() if os.path.exists(p) else "(nobody)"))

code, out, err = handoff("post", "--as", "implementer", "--item", "T1", "--to", "reviewer", "--commit", "HEAD",
                         "--title", "T1: add a.txt", "--body", os.path.join(proj, "req.md"))
note(f"a request to reviewer, posted from outside both sessions: exit {code}; " + " ".join(x for x in (out.strip(), err.strip()) if x))
posted = time.time()
rev, imp = sid("reviewer"), sid("implementer")
if until("the reviewer's session runs handoff show or claim", lambda: any(re.search(r"handoff (show|claim)", c) for c in commands(rev)), 120):
    note(f"the reviewer woke and acted {time.time() - posted:.1f}s after the post")
note("the implementer's session ran handoff since the post: " + str([c for c in commands(imp) if "role" not in c]))
if until("a reply by reviewer in the record", lambda: any(handoff("show", str(i))[1].startswith(f"#{i} reply") for i in range(2, 7)), 240):
    replied = time.time()
    note(f"replied {replied - posted:.0f}s after the post")
    if until("the implementer's session runs handoff show for the reply", lambda: any(re.search(r"handoff show", c) for c in commands(imp)), 120):
        note(f"the implementer woke and read it {time.time() - replied:.1f}s after the reply")
note("the record: " + json.dumps([handoff("show", str(i))[1].split("\n")[0] for i in range(1, 8) if handoff("show", str(i))[0] == 0]))
for role in ("implementer", "reviewer"):
    note(f"{role}'s session ran: " + json.dumps(commands(sid(role))))
    note(f"{role}'s session said: " + json.dumps(texts(sid(role))))
for s in sessions:
    s.stop()
time.sleep(2)
leftover = subprocess.run(["pgrep", "-fl", H + " hook"], capture_output=True, text=True).stdout.strip()
note(f"hook processes left: {leftover or '(none)'}")
shutil.rmtree(transcripts, ignore_errors=True)  # these throwaway sessions' transcripts; what matters is above
note("done")
