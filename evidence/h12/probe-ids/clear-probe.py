import os, pty, re, select, sys, time, signal, shutil
proj = sys.argv[1]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh")
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj)
    os.execvpe("claude", ["claude", "--model", "haiku", "Reply with the word ALPHA only."], env)
screen = b""
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.2)
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: return
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace").replace(" ", "")
pump(6)
if "Itrustthisfolder" in plain():
    os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r")
pump(15)
print("ALPHA seen:", "ALPHA" in plain())
for ch in "/clear": os.write(fd, ch.encode()); pump(0.1)
pump(1); os.write(fd, b"\r"); pump(6)
for ch in "Reply with the word BRAVO only.": os.write(fd, ch.encode()); pump(0.03)
pump(1); os.write(fd, b"\r"); pump(15)
print("BRAVO seen:", "BRAVO" in plain())
print("claude pid:", pid)
os.kill(pid, signal.SIGTERM); pump(2)
try: os.kill(pid, signal.SIGKILL)
except ProcessLookupError: pass
os.waitpid(pid, 0)
shutil.rmtree(os.path.expanduser("~/.claude/projects/" + re.sub(r"[^A-Za-z0-9]", "-", os.path.realpath(proj))), ignore_errors=True)
