# Drive one agy session in a pty. Raw output -> out.raw; control lines from ctl.fifo:
#   TYPE <text>  (types text, then Enter)   KEY ENTER|ESC|CTRLC   QUIT
import os, pty, sys, time, select, json, subprocess
P = sys.argv[1]; proj = sys.argv[2]; prompt = sys.argv[3]; tag = sys.argv[4]
env = dict(os.environ)
for k in ("CLAUDE_CODE_SESSION_ID", "CLAUDE_PID", "CODEX_THREAD_ID", "HANDOFF_ROLE", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT"):
    env.pop(k, None)
env["HANDOFF_TEST_NOTIFY"] = os.path.join(P, "notify.log")
env["TERM"] = "xterm-256color"
settings = json.load(open(os.path.join(P, "before", "settings.json")))
trusted = settings.get("trustedWorkspaces", []) + [proj]
argv = ["agy", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions",
        "--log-file", os.path.join(P, f"agy-{tag}.log"),
        "--settings", json.dumps({"trustedWorkspaces": trusted}), "-i", prompt]
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj); os.execvpe("agy", argv, env)
os.set_blocking(fd, False)
import fcntl, termios, struct
fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 160, 0, 0))
fifo = os.path.join(P, "ctl.fifo")
if os.path.exists(fifo): os.remove(fifo)
os.mkfifo(fifo)
cfd = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK)
out = open(os.path.join(P, f"out-{tag}.raw"), "ab")
ev = open(os.path.join(P, "driver-events.log"), "a")
def log(s):
    ev.write(f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} {tag} {s}\n"); ev.flush()
log(f"started pid {pid}: {' '.join(argv[:6])} ...")
buf = b""
seen_trust = False
while True:
    r, _, _ = select.select([fd, cfd], [], [], 0.5)
    if fd in r:
        try:
            data = os.read(fd, 65536)
        except OSError:
            data = b""
        if not data:
            log("pty closed"); break
        out.write(data); out.flush()
        buf = (buf + data)[-20000:]
        low = buf.lower()
        if not seen_trust and (b"trust" in low and (b"folder" in low or b"workspace" in low)):
            seen_trust = True; time.sleep(1); os.write(fd, b"\r"); log("answered trust prompt with Enter")
    if cfd in r:
        line = os.read(cfd, 65536).decode()
        for cmd in line.splitlines():
            cmd = cmd.strip()
            if not cmd: continue
            log("ctl " + cmd[:200])
            if cmd.startswith("TYPE "):
                for ch in cmd[5:]:
                    os.write(fd, ch.encode()); time.sleep(0.01)
                time.sleep(0.3); os.write(fd, b"\r")
            elif cmd == "KEY ENTER": os.write(fd, b"\r")
            elif cmd == "KEY ESC": os.write(fd, b"\x1b")
            elif cmd == "KEY CTRLC": os.write(fd, b"\x03")
            elif cmd == "QUIT":
                os.write(fd, b"\x1b"); time.sleep(1); os.write(fd, b"\x03"); time.sleep(0.5); os.write(fd, b"\x03")
    try:
        wpid, status = os.waitpid(pid, os.WNOHANG)
        if wpid == pid:
            log(f"agy exited status {status}"); break
    except ChildProcessError:
        break
log("driver done")
