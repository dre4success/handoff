# A session started in P, moved into P/proj with cd, then sent to the background: where is the
# background worker's process, and where do its hooks run? Usage: python3 -I bg-probe.py <P> <hooks log>
import os, pty, re, select, sys, time, signal, json, subprocess
P, log = sys.argv[1], sys.argv[2]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
pid, fd = pty.fork()
if pid == 0:
    os.chdir(P); os.execvpe("claude", ["claude", "--model", "haiku", "--allowedTools", "Bash(cd:*)", "Bash(sleep:*)"], env)
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
def say(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.03)
    pump(0.5); os.write(fd, b"\r")
def mark(s):
    with open(log, "a") as f: f.write("## " + s + "\n")
def alive():
    try: return os.waitpid(pid, os.WNOHANG) == (0, 0)
    except ChildProcessError: return False
def pcwd(p):
    r = subprocess.run(["lsof", "-a", "-p", str(p), "-d", "cwd", "-Fn"], capture_output=True, text=True)
    return [l[1:] for l in r.stdout.splitlines() if l.startswith("n")]
pump(6)
if "trustthisfolder" in plain().replace(" ", ""):
    os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r")
pump(4)
mark("turn 1: cd proj"); say("Run the shell command: cd proj   Then reply with the word BRAVO only."); pump(25)
mark("foreground claude pid %d, its folder %s" % (pid, pcwd(pid)))
mark("turn 2: sleep 20, then /background"); say("Run the shell command: sleep 20   Then reply with the word DONE only."); pump(5)
say("/background"); 
t = time.time()
while time.time() - t < 40 and alive(): pump(1)
mark("foreground ended: %s" % (not alive()))
pump(30)
for l in open(log):
    if l.startswith("{"):
        d = json.loads(l)
        if d["in"].get("source") == "fork" and d["cpid"]:
            mark("background worker pid %s, its folder %s" % (d["cpid"], pcwd(int(d["cpid"]))))
            break
if alive():
    os.kill(pid, signal.SIGTERM); pump(2)
    try: os.waitpid(pid, 0)
    except ChildProcessError: pass
