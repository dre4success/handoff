# Probe 2: how many times a Stop hook may continue a turn in a row, and what typing does while one holds.
# Usage: python3 -I agy-probe2.py <project> <hooks log> <mode file> <screen out>
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
def say(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.02)
    pump(0.5); os.write(fd, b"\r")
def mark(s):
    with open(log, "a") as f: f.write("## %s %s\n" % (time.strftime("%H:%M:%S", time.gmtime()), s))
pump(8)
if re.search(r"trust", plain(), re.I): os.write(fd, b"\r"); pump(4)
open(modef, "w").write("count\n"); mark("mode count; turn 1")
say("Reply with the word ALPHA only.")
pump(120)
mark("words seen: %s" % sorted(set(re.findall(r"PING\d+", plain()))))
open(modef, "w").write("hold 60\n"); mark("mode hold 60; turn 2")
say("Reply with the word BRAVO only.")
pump(15)
mark("typing while the stop holds"); say("Reply with the word TYPED only."); pump(80)
mark("words now: TYPED %s, PONG %s" % ("TYPED" in plain().split("Reply with the word TYPED only.")[-1], "PONG" in plain()))
os.write(fd, b"\x03"); pump(1.5); os.write(fd, b"\x03"); pump(4)
try:
    if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(2); os.waitpid(pid, 0)
except ChildProcessError: pass
open(scr, "w").write(plain())
