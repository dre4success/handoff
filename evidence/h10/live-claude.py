"""End-to-end check of H10a's Claude Code hooks with a real, throwaway session.

usage: python3 -I live-claude.py <project> <handoff binary> <alerts file> <log file>  (live-claude.sh runs it)

The project has a handoff record (roles claude, codex) and the settings that
`handoff settings claude --as claude` printed, in .claude/settings.local.json.
Alerts go to the alerts file (HANDOFF_TEST_NOTIFY), never to the screen.
"""
import glob, json, os, pty, re, select, shutil, signal, subprocess, sys, time

proj, H, alerts, logpath = sys.argv[1:5]
log = open(logpath, "a", buffering=1)
def note(msg):
    log.write(f"{time.strftime('%H:%M:%S', time.gmtime())}Z {msg}\n")

env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=alerts)
henv = dict(env)  # for handoff run from here, as the other agent or as a probe

def handoff(*args, role=None):
    e = dict(henv)
    if role:
        e["HANDOFF_ROLE"] = role
    r = subprocess.run([H, *args], cwd=proj, env=e, capture_output=True, text=True, timeout=60)
    return r.returncode, r.stdout, r.stderr

pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj)
    os.execvpe("claude", ["claude", "--model", "haiku",
                          "Say READY, then the role your handoff brief gives you, and nothing else."], env)

screen = b""
def pump(secs):
    global screen
    end = time.time() + secs
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.2)
        if r:
            try:
                chunk = os.read(fd, 65536)
            except OSError:
                return
            screen = (screen + chunk)[-200000:]
def plain():
    return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")

# Claude Code keeps this session's transcript under its project's real path,
# every character but a letter or digit made "-". Only this run's folder.
transcripts = os.path.expanduser("~/.claude/projects/" + re.sub(r"[^A-Za-z0-9]", "-", os.path.realpath(proj)))
def records():
    out = []
    for p in glob.glob(os.path.join(transcripts, "*.jsonl")):
        for line in open(p, errors="replace"):
            try:
                out.append(json.loads(line))
            except ValueError:
                pass
    return out
def texts():
    return [c["text"] for d in records() if d.get("type") == "assistant"
            for c in (d.get("message", {}).get("content") or []) if isinstance(c, dict) and c.get("type") == "text"]
def commands():
    return [(d.get("timestamp"), c.get("input", {}).get("command", "")) for d in records() if d.get("type") == "assistant"
            for c in (d.get("message", {}).get("content") or []) if isinstance(c, dict) and c.get("type") == "tool_use"]
def until(what, test, secs):
    end = time.time() + secs
    while time.time() < end:
        pump(1)
        if test():
            note(f"seen: {what}")
            return True
    note(f"NOT seen within {secs}s: {what}")
    return False

note(f"claude started, pid {pid}")
until("the trust dialog", lambda: "Entertoconfirm" in plain().replace(" ", ""), 30)
if "Itrustthisfolder" in plain().replace(" ", ""):
    note("trust dialog: choosing 'Yes, I trust this folder'")
    os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r")
if until("READY from the model", lambda: any("READY" in t for t in texts()), 90):
    note("model said: " + json.dumps([t for t in texts() if "READY" in t]))
    pump(4)
    code, out, err = handoff("wait", "--for", "claude", "--timeout", "3s")
    note(f"a second wait for claude while the session idles: exit {code}; stderr {err.strip()!r}")
    code, out, err = handoff("post", "--as", "codex", "--item", "T1", "--to", "claude", "--commit", "HEAD",
                             "--title", "T1: add a.txt", "--body", os.path.join(proj, "req.md"))
    note(f"codex posts a request: exit {code}; " + " ".join(x for x in (out.strip(), err.strip()) if x))
    posted = time.time()
    if until("the session runs handoff show or claim", lambda: any(re.search(r"handoff (show|claim)", c) for _, c in commands()), 120):
        note(f"woke and acted {time.time() - posted:.1f}s after the post")
    if until("a reply by claude in the record", lambda: any(handoff("show", str(i))[1].startswith(f"#{i} reply") for i in range(2, 7)), 180):
        note(f"replied {time.time() - posted:.0f}s after the post")
    note("the record after the request: " + json.dumps([handoff("show", str(i))[1].split("\n")[0] for i in range(2, 7) if handoff("show", str(i))[0] == 0]))
note("commands the session ran: " + json.dumps(commands()))
note("assistant texts: " + json.dumps(texts()))
os.kill(pid, signal.SIGTERM)
pump(3)
try:
    os.kill(pid, signal.SIGKILL)
except ProcessLookupError:
    pass
os.waitpid(pid, 0)
time.sleep(2)
code, out, err = handoff("wait", "--for", "claude", "--timeout", "2s")
note(f"a wait for claude after the session ended: exit {code}; stdout {out.strip()!r}; stderr {err.strip()!r}")
leftover = subprocess.run(["pgrep", "-fl", H + " hook"], capture_output=True, text=True).stdout.strip()
note(f"hook processes left: {leftover or '(none)'}")
shutil.rmtree(transcripts, ignore_errors=True)  # this throwaway session's transcript; what matters is above
note("done")
