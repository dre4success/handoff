# Probe 4: does a Stop hook with timeout 86400 hold past 10 minutes? It holds 660 s, then continues.
# Usage: python3 -I agy-probe4.py <project> <hooks log> <mode file> <screen out>
import os, pty, re, select, sys, time, signal
proj, log, modef, scr = sys.argv[1:5]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
screen = b""
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj); os.execvpe("agy", ["agy", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions"], env)
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.2)
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: return
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")
def typ(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.02)
    pump(0.5)
def mark(s):
    with open(log, "a") as f: f.write("## %s %s\n" % (time.strftime("%H:%M:%S", time.gmtime()), s))
pump(8)
if re.search(r"trust", plain(), re.I): os.write(fd, b"\r"); pump(4)
open(modef, "w").write("hold 660\n"); mark("run L: hold 660 s, timeout 86400")
typ("Reply with the word ALPHA only."); os.write(fd, b"\r"); pump(700)
mark("seen: " + ", ".join(w for w in ("ALPHA", "PONG") if re.search(r"\b%s\b" % w, plain().replace("Reply with the word %s only." % w, ""))))
os.write(fd, b"\x03"); pump(1.5); os.write(fd, b"\x03"); pump(4)
try:
    if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(2); os.waitpid(pid, 0)
except ChildProcessError: pass
open(scr, "w").write(plain())
