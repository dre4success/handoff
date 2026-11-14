# real-probe.py <scratch dir> <handoff binary> <tool|idle|fork|resume:<session id>>
# A throwaway interactive Claude Code session (Haiku) through a pseudo-terminal, in
# <scratch dir>/proj, whose .claude/settings.local.json runs hookwrap.sh at SessionStart,
# Stop, SessionEnd and at a permission prompt. hookwrap.sh logs what the hook was given
# and runs the build under review on it, in the scratch record <scratch dir>/rec.
#   tool:  the session is given implementer, ends one turn, starts a long command, and
#          /background is typed while the command runs
#   idle:  the same, but /background is typed with the turn over and its listener waiting
#   fork:  the same, with /fork
#   resume:<id>: `claude --resume <id>`; the resumed session is given implementer, has one turn, and is closed
# This driver reaps the session's process within about 5 ms of its end, as a shell would.
import os, pty, re, select, sys, time, signal, json, subprocess
D, BIN, mode = sys.argv[1], sys.argv[2], sys.argv[3]
proj, rec, log = D + "/proj", D + "/rec", None
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=D + "/alerts.txt", HANDOFF_TEST_AWAKE=D + "/awake.txt")
args = ["claude", "--model", "haiku", "--allowedTools", "Bash(python3:*)"]
if mode.startswith("resume:"):
    args += ["--resume", mode.split(":", 1)[1]]
t00 = time.time()
def stamp(): return time.strftime("%H:%M:%S", time.gmtime()) + ".%03d" % (time.time() * 1000 % 1000)
def say(*a): print(stamp(), *a, flush=True)
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj)
    os.execvpe("claude", args, env)
screen = b""; ended_at = None
def reap():
    global ended_at
    if ended_at is None:
        try:
            if os.waitpid(pid, os.WNOHANG) != (0, 0):
                ended_at = stamp(); say("the session's process", pid, "has ended and is reaped")
        except ChildProcessError:
            ended_at = stamp()
    return ended_at is None
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        reap()
        try: r, _, _ = select.select([fd], [], [], 0.004)
        except (OSError, ValueError): time.sleep(0.02); continue
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: time.sleep(0.02)
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")
def typ(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.03)
def enter(): pump(0.6); os.write(fd, b"\r")
def tail(n=900):
    lines = [l.rstrip() for l in plain()[-3000:].splitlines() if l.strip()]
    print("\n".join("    | " + l[:160] for l in lines[-14:]), flush=True)
def blocks():
    # the hook inputs hookwrap.sh kept, oldest first: (hook, input, the CLI's process)
    out = []
    for name in sorted(os.listdir(D + "/in"), key=lambda n: os.stat(D + "/in/" + n).st_mtime):
        if name.endswith(".out"): continue
        try: j = json.load(open(D + "/in/" + name))
        except Exception: continue
        out.append((name.rsplit(".", 1)[1], j, 0))
    return out
def listeners():
    r = subprocess.run(["pgrep", "-f", BIN + " hook listen"], capture_output=True, text=True)
    return len(r.stdout.split())
def binding():
    out = []
    for n in os.listdir(rec + "/.handoff/sessions"):
        if n.endswith(".json"):
            b = json.load(open(rec + "/.handoff/sessions/" + n)); out.append("%s has %s" % (b["session"], b["role"]))
    return "; ".join(out)
def wait_for(pred, secs, what):
    t = time.time()
    while time.time() - t < secs:
        hit = [b for b in blocks() if pred(b)]
        if hit: return hit[-1]
        pump(0.3)
    say("did not see:", what); return None
n0 = len(blocks())
pump(6)
if "trust" in plain().lower().replace(" ", "")[-4000:] and "folder" in plain().lower():
    say("the folder-trust question is shown; answering yes (down, Enter)")
    os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r"); pump(3)
first = wait_for(lambda b: b[1].get("hook_event_name") == "SessionStart" and blocks().index(b) >= n0, 40, "the session's SessionStart")
if not first:
    tail()
    if reap(): os.kill(pid, signal.SIGKILL)
    sys.exit(1)
sid, cpid = first[1]["session_id"], pid
say("session", sid, "CLAUDE_PID", cpid, "(the driver's child is", str(pid) + ")", "SessionStart source:", first[1].get("source"))
if mode.startswith("resume:"):
    r = subprocess.run([BIN, "role", "implementer"], cwd=rec, capture_output=True, text=True, env=dict(env, CLAUDE_CODE_SESSION_ID=sid, CLAUDE_PID=str(cpid)))
    say("handoff role implementer, as the resumed session:", r.returncode, (r.stdout.strip().splitlines() or [""])[0], r.stderr.strip())
    pump(4); typ("Reply with the word DELTA only."); enter()
    wait_for(lambda b: b[1].get("hook_event_name") == "Stop" and b[1]["session_id"] == sid, 90, "its Stop")
    pump(2); say("DELTA on screen:", "DELTA" in plain(), "| binding: %s | listeners of this build running: %d" % (binding(), listeners()))
    typ("/exit"); enter()
    t0 = time.time()
    while time.time() - t0 < 15 and reap(): pump(0.05)
    if reap(): os.kill(pid, signal.SIGTERM); pump(3)
    pump(1.5); say("after it closed: binding: %s | listeners of this build running: %d" % (binding(), listeners()))
    sys.exit(0)
r = subprocess.run([BIN, "role", "implementer"], cwd=rec, capture_output=True, text=True, env=dict(env, CLAUDE_CODE_SESSION_ID=sid, CLAUDE_PID=str(cpid)))
say("handoff role implementer, as that session:", r.returncode, (r.stdout.strip().splitlines() or [""])[0], r.stderr.strip())
pump(2); typ("Reply with the word ALPHA only."); enter()
stop1 = wait_for(lambda b: b[1].get("hook_event_name") == "Stop" and b[1]["session_id"] == sid, 90, "the first turn's Stop")
pump(2); say("first turn over:", bool(stop1), "| ALPHA on screen:", "ALPHA" in plain())
if mode == "tool":
    typ("Run this exact shell command in the foreground, not in the background, and wait for it to finish: python3 -c 'import time; time.sleep(45)'  Then reply with the word BRAVO only."); enter()
    pump(14); say("14 s into the second turn; the screen:"); tail()
cmd = "/fork" if mode == "fork" else "/background"
typ(cmd); pump(1.5); say("typed", cmd); tail(); os.write(fd, b"\r"); say("pressed Enter")
t0 = time.time()
while time.time() - t0 < 75 and reap(): pump(0.05)
say("the session's process still runs %.0f s after %s: %s" % (time.time() - t0, cmd, reap())); tail()
pump(3); say("3 s later: binding: %s | listeners of this build running: %d" % (binding(), listeners()))
nxt = wait_for(lambda b: b[1].get("hook_event_name") == "Stop" and b[1]["session_id"] != sid, 100, "a Stop of the new session")
pump(2.5); say("after the new session's first Stop (seen: %s): binding: %s | listeners of this build running: %d" % (bool(nxt), binding(), listeners()))
if reap():
    say("closing the session that still runs"); typ("/exit"); enter(); pump(6)
    if reap(): os.kill(pid, signal.SIGTERM); pump(3)
    if reap(): os.kill(pid, signal.SIGKILL); pump(1)
    pump(1.5); say("after it closed: binding: %s | listeners of this build running: %d" % (binding(), listeners()))
