# A throwaway Antigravity CLI session through a pseudo-terminal, in a scratch project whose
# .agents/hooks.json runs dump.sh. Usage: python3 -I agy-probe.py <project> <hooks log> <screen out>
import os, pty, re, select, sys, time, signal, json
proj, log, scr = sys.argv[1:4]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
MODEL = "gemini-3.8-flash-low"
screen = b""
def start(args):
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(proj)
        os.execvpe("agy", ["agy", "--model", MODEL, "--dangerously-skip-permissions"] + args, env)
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
def mark(s):
    with open(log, "a") as f: f.write("## %s %s\n" % (time.strftime("%H:%M:%S", time.gmtime()), s))
def alive(pid):
    try: return os.waitpid(pid, os.WNOHANG) == (0, 0)
    except ChildProcessError: return False
def stop(pid, fd):
    for _ in range(2):
        if not alive(pid): break
        os.write(fd, b"\x03"); pump(fd, 1.5)
    t = time.time()
    while alive(pid) and time.time() - t < 8: pump(fd, 0.5)
    if alive(pid):
        mark("still running after two Ctrl-C; SIGTERM"); os.kill(pid, signal.SIGTERM); pump(fd, 2)
        try: os.waitpid(pid, 0)
        except ChildProcessError: pass
def conv():
    for l in open(log):
        if l.startswith("{"):
            d = json.loads(l); i = d.get("in") or {}
            if i.get("conversationId"): return i["conversationId"]
pid, fd = start([])
pump(fd, 8)
mark("screen at start: " + " | ".join(l.strip() for l in plain().splitlines() if l.strip())[-400:])
if re.search(r"trust", plain(), re.I):
    mark("trust prompt seen; Enter"); os.write(fd, b"\r"); pump(fd, 4)
mark("turn 1")
say(fd, "First say the secret word you were given at the start, if any. Then run this shell command: env | grep -E '^(ANTIGRAVITY|AGY|GEMINI)' | sed 's/=.*//' ; echo CONV=$ANTIGRAVITY_CONVERSATION_ID ; pwd   and reply with the word and the command's output, nothing else.")
pump(fd, 130)  # the first Stop hook holds the turn 70 s, then continues it
mark("after turn 1 and the held stop")
cid = conv(); mark("conversation %s; exit" % cid)
stop(pid, fd)
open(scr, "w").write(plain())
if cid:
    mark("resume with --conversation")
    pid, fd = start(["--conversation", cid])
    pump(fd, 10)
    if re.search(r"trust", plain()[-2000:], re.I):
        os.write(fd, b"\r"); pump(fd, 4)
    mark("turn 2 after resume")
    say(fd, "Run this shell command: echo CONV=$ANTIGRAVITY_CONVERSATION_ID   and reply with its output only.")
    pump(fd, 40)
    mark("exit")
    stop(pid, fd)
open(scr, "w").write(plain())
