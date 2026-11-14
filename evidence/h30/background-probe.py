import os, pty, re, select, sys, time, signal, json, subprocess
proj, log = sys.argv[1], sys.argv[2]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj)
    os.execvpe("claude", ["claude", "--model", "haiku", "--allowedTools", "Bash(sleep:*)", "Reply with the word ALPHA only."], env)
screen = b""
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
    for ch in s: os.write(fd, ch.encode()); pump(0.03)
def alive():
    try: return os.waitpid(pid, os.WNOHANG) == (0, 0)
    except ChildProcessError: return False
pump(6)
if "trustthisfolder" in plain().replace(" ", ""):
    os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r")
pump(15)
print("ALPHA seen:", "ALPHA" in plain())
first = [json.loads(l) for l in open(log) if l.startswith("{")][0]
print("first session:", first["session_id"], "claude pid:", pid)
r = subprocess.run(["handoff", "role", "implementer"], cwd=proj, capture_output=True, text=True,
    env=dict(env, CLAUDE_CODE_SESSION_ID=first["session_id"], CLAUDE_PID=str(pid)))
print("handoff role:", r.returncode, r.stdout.strip().splitlines()[:1], r.stderr.strip())
typ("Run the shell command sleep 25, then reply with the word BRAVO only."); pump(0.5); os.write(fd, b"\r")
pump(8)
typ("/background"); pump(1.5); os.write(fd, b"\r")
t0 = time.time()
while time.time() - t0 < 50 and alive(): pump(1)
print("foreground alive after /background: %s (%.0fs)" % (alive(), time.time() - t0))
tail = plain()[-1500:]
print("screen tail:\n" + "\n".join(l.rstrip() for l in tail.splitlines() if l.strip())[-1200:])
pump(30)
if alive():
    os.kill(pid, signal.SIGTERM); pump(2)
    try: os.kill(pid, signal.SIGKILL)
    except ProcessLookupError: pass
    os.waitpid(pid, 0)
