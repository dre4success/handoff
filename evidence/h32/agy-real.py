# A real run of this branch's Antigravity hooks, as `handoff settings antigravity` prints them, in a
# throwaway project. The session takes the reviewer role; a request posted from outside wakes it; Esc
# frees it to take a message; it is closed, resumed, and woken again.
# Usage: python3 -I agy-real.py <project> <handoff binary> <log> <screen out>
import os, pty, re, select, sys, time, signal, json, subprocess, glob
proj, hb, log, scr = sys.argv[1:5]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=log + ".told")
screen = b""
def mark(s):
    with open(log, "a") as f: f.write("## %s %s\n" % (time.strftime("%H:%M:%S", time.gmtime()), s))
def start(args):
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(proj); os.execvpe("agy", ["agy", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions"] + args, env)
    return pid, fd
def pump(fd, s):
    global screen
    end = time.time() + s
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.2)
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: return
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")
def say(fd, s):
    for ch in s: os.write(fd, ch.encode()); pump(fd, 0.02)
    pump(fd, 0.5); os.write(fd, b"\r")
def binding():
    out = []
    for p in glob.glob(os.path.join(proj, ".handoff", "sessions", "*.json")):
        b = json.load(open(p)); out.append("%s %s host=%s" % (b["role"], b["session"][:8], b.get("host")))
    return out
def hb_run(*args, role=None):
    e = dict(env); e.pop("ANTIGRAVITY_CONVERSATION_ID", None)
    if role: e["HANDOFF_ROLE"] = role
    r = subprocess.run([hb] + list(args), cwd=proj, env=e, capture_output=True, text=True)
    return (r.stdout + r.stderr).strip().splitlines()[-1:] or [""]
def seen(word, since): return word in plain()[since:]
def stop(pid, fd):
    os.write(fd, b"\x03"); pump(fd, 1.5); os.write(fd, b"\x03"); pump(fd, 4)
    try:
        if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(fd, 2); os.waitpid(pid, 0)
    except ChildProcessError: pass
body = os.path.join(os.path.dirname(log), "probe-body.md"); open(body, "w").write("A probe request.\n")
commit = subprocess.run(["git", "rev-parse", "w1"], cwd=proj, capture_output=True, text=True).stdout.strip()
pid, fd = start([])
pump(fd, 8)
if re.search(r"trust", plain(), re.I): os.write(fd, b"\r"); pump(fd, 4)
mark("agy pid %d; turn 1: take the role" % pid)
say(fd, "You're the reviewer in handoff. Run: %s role reviewer   Then, for this test only: when handoff brings you work, run %s show <id> for it and reply with the word GOT and the entry's title, nothing else, and don't claim or review it. Now end your turn." % (hb, hb))
pump(fd, 45)
mark("brief at the first message: %s" % ("seen" if "has no role in it" in plain() or "you are reviewer" in plain() else "not seen"))
mark("bindings: %s" % binding())
n = len(plain())
mark("post a request to the reviewer from outside: %s" % hb_run("post", "--item", "W1", "--to", "reviewer", "--commit", commit, "--title", "probe one", "--body", body, role="implementer"))
pump(fd, 45)
mark("woken with it: %s; GOT: %s" % (seen("new for reviewer", n), seen("GOT", n)))
mark("bindings: %s" % binding())
n = len(plain())
mark("while it listens: Esc, then a message"); pump(fd, 5); os.write(fd, b"\x1b"); pump(fd, 2)
say(fd, "Reply with the word HELLO only."); pump(fd, 25)
mark("HELLO answered: %s" % seen("HELLO", n + 40))
cid = None
for p in glob.glob(os.path.join(proj, ".handoff", "sessions", "*.json")):
    cid = json.load(open(p))["session"]
mark("close; resume conversation %s" % (cid or "?")[:8]); stop(pid, fd)
pid, fd = start(["--conversation", cid])
pump(fd, 10)
if re.search(r"trust", plain()[-1500:], re.I): os.write(fd, b"\r"); pump(fd, 4)
mark("resumed: agy pid %d; a message so its turn ends once" % pid)
say(fd, "Reply with the word READY only, then end your turn."); pump(fd, 25)
mark("bindings: %s" % binding())
n = len(plain())
mark("post a second request: %s" % hb_run("post", "--item", "W2", "--to", "reviewer", "--commit", commit, "--title", "probe two", "--body", body, role="implementer"))
pump(fd, 45)
mark("woken with it: %s; GOT: %s" % (seen("new for reviewer", n), seen("GOT", n)))
mark("operator told: %s" % (open(log + ".told").read().strip() if os.path.exists(log + ".told") else "nothing"))
stop(pid, fd)
open(scr, "w").write(plain())
