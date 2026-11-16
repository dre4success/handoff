import os, pty, re, select, sys, time, signal, json, subprocess
P, log = sys.argv[1], sys.argv[2]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
def start(args):
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(P)
        os.execvpe("claude", ["claude", "--model", "haiku", "--allowedTools", "Bash(cd:*)", "Bash(pwd)", "Bash(pwd:*)"] + args, env)
    return pid, fd
screen = b""
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
    for ch in s: os.write(fd, ch.encode()); pump(fd, 0.03)
    pump(fd, 0.5); os.write(fd, b"\r")
def stop(pid, fd):
    os.write(fd, b"\x03"); pump(fd, 0.5); os.write(fd, b"\x03"); pump(fd, 3)
    try:
        if os.waitpid(pid, os.WNOHANG) == (0, 0):
            os.kill(pid, signal.SIGTERM); pump(fd, 2); os.waitpid(pid, 0)
    except ChildProcessError: pass
def pcwd(pid):
    r = subprocess.run(["lsof", "-a", "-p", str(pid), "-d", "cwd", "-Fn"], capture_output=True, text=True)
    return [l[1:] for l in r.stdout.splitlines() if l.startswith("n")]
def mark(s):
    with open(log, "a") as f: f.write("## " + s + "\n")
pid, fd = start([])
pump(fd, 6)
if "trustthisfolder" in plain().replace(" ", ""):
    os.write(fd, b"\x1b[B"); pump(fd, 0.8); os.write(fd, b"\r")
pump(fd, 4)
mark("turn 1: reply only"); say(fd, "Reply with the word ALPHA only."); pump(fd, 15)
mark("turn 2: cd proj"); say(fd, "Run the shell command: cd proj   Then reply with the word BRAVO only."); pump(fd, 25)
mark("process cwd of claude: %s" % pcwd(pid))
mark("turn 3: pwd"); say(fd, "Run the shell command: pwd   Then reply with its output only."); pump(fd, 20)
sid = None
for l in open(log):
    if l.startswith("{"): sid = json.loads(l)["in"]["session_id"]; break
mark("session %s; exit" % sid); stop(pid, fd)
mark("resume from P")
pid, fd = start(["--resume", sid])
pump(fd, 10)
mark("process cwd of resumed claude: %s" % pcwd(pid))
mark("turn 4 after resume: pwd"); say(fd, "Run the shell command: pwd   Then reply with its output only."); pump(fd, 20)
mark("exit"); stop(pid, fd)
print("\n".join(l.rstrip() for l in plain().splitlines() if l.strip())[-1500:])
