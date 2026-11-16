# Does a command the Antigravity agent runs have a terminal on stdin? (handoff decide refuses without one.)
# Usage: python3 -I agy-tty.py <folder> <screen out>
import os, pty, re, select, sys, time, signal
proj, scr = sys.argv[1:3]
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
pump(8)
if re.search(r"trust", plain(), re.I): os.write(fd, b"\r"); pump(4)
for ch in "Run this exact shell command and reply with its output only: python3 -c 'import os,sys; print(\"STDIN_TTY=%s STDOUT_TTY=%s\" % (os.isatty(0), os.isatty(1)))'":
    os.write(fd, ch.encode()); pump(0.02)
pump(0.5); os.write(fd, b"\r"); pump(45)
os.write(fd, b"\x03"); pump(1.5); os.write(fd, b"\x03"); pump(4)
try:
    if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(2); os.waitpid(pid, 0)
except ChildProcessError: pass
open(scr, "w").write(plain())
