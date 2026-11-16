# The reviewer's R4 and R6 with this build: a session begun in P (or P/proj), closed, and resumed from
# the other; each run, the session runs handoff role from P/proj into a file.
# Usage: python3 -I resume-probe.py <P> <handoff binary> <out folder> <log>
import os, pty, re, select, sys, time, signal, json
P, hb, outd, log = sys.argv[1:5]
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY="1")
screen = b""
def start(folder, args):
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(folder); os.execvpe("claude", ["claude", "--model", "haiku", "--dangerously-skip-permissions"] + args, env)
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
def dialogs(fd):
    # The folder-trust dialog defaults to "No, exit"; the bypass-permissions one to "No". Both: Down, Enter.
    for _ in range(2):
        pump(fd, 5)
        t = plain()[-3000:].replace(" ", "")
        if "trustthisfolder" in t or "BypassPermissionsmode" in t and "Yes,Iaccept" in t:
            os.write(fd, b"\x1b[B"); pump(fd, 0.8); os.write(fd, b"\r"); pump(fd, 2)
def stop(pid, fd):
    os.write(fd, b"\x03"); pump(fd, 0.5); os.write(fd, b"\x03"); pump(fd, 3)
    try:
        if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(fd, 2); os.waitpid(pid, 0)
    except ChildProcessError: pass
def mark(s):
    with open(log, "a") as f: f.write("## %s %s\n" % (time.strftime("%H:%M:%S", time.gmtime()), s))
def role(fd, args, out):
    say(fd, "Run exactly this shell command: cd %s/proj && %s role %s > %s 2>&1   Then reply with the word DONE only." % (P, hb, args, out))
    pump(fd, 30)
proj = os.path.join(P, "proj")
for case, first, then, name in (("R4", P, proj, "implementer"), ("R6", proj, P, "reviewer")):
    pid, fd = start(first, []); dialogs(fd)
    mark("%s: begun in %s; takes %s from proj" % (case, first, name)); role(fd, name, "%s/%s-1.txt" % (outd, case))
    sid = None
    for f in [f for f in os.listdir(os.path.join(proj, ".handoff", "sessions")) if f.endswith(".json")]:
        b = json.load(open(os.path.join(proj, ".handoff", "sessions", f)))
        if b["role"] == name: sid = b["session"]
    mark("%s: session %s; closed; resumed from %s; handoff role from proj" % (case, sid, then)); stop(pid, fd)
    if not sid: continue
    pid, fd = start(then, ["--resume", sid]); dialogs(fd)
    role(fd, "", "%s/%s-2.txt" % (outd, case))
    stop(pid, fd)
open(os.path.join(outd, "screen.txt"), "w").write(plain())
