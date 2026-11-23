#!/usr/bin/env python3
"""H33 review probe: throwaway Claude Code sessions (Haiku) in a scratch project.

C1  a print-mode run takes the reviewer role (does a print-mode run fire the start hook?)
C2  the same session resumed in a terminal, nothing typed: does it listen, and does a
    request wake it?
C3  a session started in a terminal and closed with nothing typed: is there anything
    to resume?
"""
import json, os, pty, re, select, signal, subprocess, sys, time, uuid

S = os.path.dirname(os.path.abspath(__file__))
PROJ = os.path.join(S, "proj")
RUNS = os.path.join(S, "log", "runs.jsonl")
SHIM = os.path.join(S, "shim", "handoff")
HOME = os.environ["HOME"]
BIN = os.path.join(HOME, "go", "bin", "handoff")  # the installed build, main at 09ece7d
ENV = {
    "HOME": HOME, "USER": os.environ.get("USER", ""), "TERM": "xterm-256color",
    "LANG": "en_US.UTF-8", "SHELL": "/bin/zsh",
    "PATH": os.path.join(S, "shim") + ":" + HOME + "/.local/bin:" + HOME + "/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
}
HENV = dict(ENV, HANDOFF_TEST_NOTIFY=os.path.join(S, "alerts.txt"), HANDOFF_TEST_AWAKE=os.path.join(S, "awake.txt"))
ANSI = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()][A-Za-z0-9]|\x1b[=>]")


def say(msg):
    print(time.strftime("%H:%M:%S", time.gmtime()), msg, flush=True)


def runs():
    try:
        with open(RUNS) as f:
            return [json.loads(l) for l in f if l.strip()]
    except FileNotFoundError:
        return []


def short(r):
    i = r.get("in") or {}
    return "handoff %-28s %s %s session=%s" % (r["args"], i.get("hook_event_name", ""), i.get("source", ""), (i.get("session_id") or "")[:8])


def new_runs(seen):
    all_ = runs()
    for r in all_[seen:]:
        say("   " + short(r))
    return len(all_)


def listeners(top):
    # `handoff hook listen` processes below the session's process
    out = subprocess.run(["ps", "-axo", "pid=,ppid=,args="], capture_output=True, text=True).stdout
    rows = [l.split(None, 2) for l in out.splitlines() if len(l.split(None, 2)) == 3]
    parent = {r[0]: r[1] for r in rows}
    def below(pid):
        for _ in range(12):
            if pid == str(top):
                return True
            pid = parent.get(pid, "")
        return False
    return [r for r in rows if r[2].rstrip().endswith("hook listen") and below(r[0])]


class Term:
    def __init__(self, args):
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            os.chdir(PROJ)
            os.execvpe(args[0], args, ENV)
        self.buf = ""

    def pump(self, secs):
        end = time.time() + secs
        while time.time() < end:
            r, _, _ = select.select([self.fd], [], [], 0.3)
            if r:
                try:
                    self.buf += os.read(self.fd, 65536).decode("utf-8", "replace")
                except OSError:
                    return False
        return True

    def text(self):
        return re.sub(r"[ \t]+", " ", ANSI.sub("", self.buf))

    def send(self, s):
        os.write(self.fd, s.encode())

    def alive(self):
        try:
            return os.waitpid(self.pid, os.WNOHANG) == (0, 0)
        except ChildProcessError:
            return False

    def close(self):
        for s in ("\x1b", "\x03", "\x03"):
            try:
                self.send(s)
            except OSError:
                break
            self.pump(1)
        if self.alive():
            os.kill(self.pid, signal.SIGTERM)
            time.sleep(2)
        if self.alive():
            os.kill(self.pid, signal.SIGKILL)
            time.sleep(1)


def trust(t):
    # a new folder asks whether to trust it; the default is No, so Down, then Enter
    for _ in range(20):
        t.pump(1)
        if "trust" in t.text().lower():
            say("   (the folder's trust question: answered Yes, with Down and Enter)")
            t.send("\x1b[B")
            t.pump(1)
            t.send("\r")
            t.buf = ""
            return


def c1(sid):
    say("## C1 print mode: claude -p --session-id %s, told to run `handoff role reviewer`" % sid[:8])
    seen = len(runs())
    p = subprocess.Popen(
        ["claude", "-p", "--model", "haiku", "--session-id", sid, "--allowedTools", "Bash", "--",
         "Run this exact command with the Bash tool and nothing else: %s role reviewer\nThen answer with the one word done." % SHIM],
        cwd=PROJ, env=ENV, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    end = time.time() + 150
    while time.time() < end and p.poll() is None:
        time.sleep(2)
        if any(r["args"] == "hook listen" and (r.get("in") or {}).get("hook_event_name") == "Stop" for r in runs()[seen:]):
            break
    seen = new_runs(seen)
    say("## C1 the run is %s; stopping it (its Stop hook listens, so it would not end)" % ("still running" if p.poll() is None else "over, exit %s" % p.returncode))
    if p.poll() is None:
        p.terminate()
    try:
        out = p.communicate(timeout=10)[0]
    except subprocess.TimeoutExpired:
        p.kill(); out = p.communicate()[0]
    say("## C1 printed: %r" % out.strip()[:200])
    b = os.path.join(PROJ, ".handoff", "sessions", sid + ".json")
    say("## C1 binding: %s" % (re.sub(r'"pid":\d+', '"pid":<n>', open(b).read().strip()) if os.path.exists(b) else "none"))
    time.sleep(3)


def c2(sid):
    say("## C2 the same session in a terminal: claude --resume %s, nothing typed" % sid[:8])
    seen = len(runs())
    t = Term(["claude", "--resume", sid, "--model", "haiku", "--allowedTools", "Bash"])
    trust(t)
    t.pump(15)
    seen = new_runs(seen)
    ls = listeners(t.pid)
    say("## C2 15 s after the start, nothing typed: %d `handoff hook listen` of this probe running" % len(ls))
    b = os.path.join(PROJ, ".handoff", "presence", "reviewer.json")
    say("## C2 presence for reviewer: %s" % ("a file, how=%s" % json.load(open(b)).get("how") if os.path.exists(b) else "none"))
    r = subprocess.run([BIN, "post", "--as", "implementer", "--item", os.environ.get("ITEM", "W1"), "--to", "reviewer", "--commit", "HEAD",
                        "--title", "probe: are you listening", "--body", os.path.join(S, "body.md")],
                       cwd=PROJ, env=HENV, capture_output=True, text=True)
    say("## C2 a request from the implementer's side: exit %d | %s" % (r.returncode, (r.stdout + r.stderr).strip().replace("\n", " | ")))
    t.buf = ""
    woke = False
    for _ in range(18):
        t.pump(5)
        if any((r_.get("in") or {}).get("hook_event_name") == "Stop" for r_ in runs()[seen:]):
            woke = True
            break
    seen = new_runs(seen)
    say("## C2 %s" % ("the session took a turn with nothing typed (its Stop hook ran): it was woken" if woke else "no turn in 90 s"))
    say("## C2 `handoff hook listen` below the session now: %d" % len(listeners(t.pid)))
    tail = [l.strip() for l in t.text().splitlines() if l.strip()]
    say("## C2 the screen's last lines: %s" % " / ".join(tail[-6:])[:500])
    t.close()
    say("## C2 closed; process %s" % ("still alive" if t.alive() else "ended"))
    new_runs(seen)


def c3(sid):
    say("## C3 a new session in a terminal, nothing typed, then closed: claude --session-id %s" % sid[:8])
    seen = len(runs())
    t = Term(["claude", "--session-id", sid, "--model", "haiku"])
    trust(t)
    t.pump(12)
    seen = new_runs(seen)
    t.close()
    time.sleep(2)
    n = 0
    for root, _, files in os.walk(os.path.join(HOME, ".claude", "projects")):
        n += sum(1 for f in files if f == sid + ".jsonl")
    say("## C3 transcripts named for it under the CLI's projects folder: %d" % n)
    r = subprocess.run(["claude", "-p", "--model", "haiku", "--resume", sid, "Answer with the one word hello."],
                       cwd=PROJ, env=ENV, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=120)
    say("## C3 claude -p --resume of it: exit %d | %s" % (r.returncode, (r.stdout + r.stderr).strip().replace("\n", " | ")[:300]))
    new_runs(seen)


if __name__ == "__main__":
    which = sys.argv[1]
    sid = sys.argv[2]
    {"c1": c1, "c2": c2, "c3": c3}[which](sid)
    say("## done")
