# real-probe.py <scratch dir> <clear|background|inside|plain>
# A throwaway interactive Claude Code session (Haiku) through a pseudo-terminal, started in
# <scratch dir>/P, which is no git repository. <scratch dir>/P/proj is one, after `handoff init`.
# <scratch dir>/shim is first on the session's PATH, so the hooks in the user's settings, which
# run `handoff` by name, run shim/handoff: it notes each run and runs the build under review.
# Every mode: the session runs `cd proj`, then `handoff role implementer`, and says what it was told.
#   plain:      then it is closed, and resumed from P
#   clear:      then /compact, then /clear, then `handoff role`; it is closed, and the new id is resumed from P
#   background: then /background; a note is sent to implementer; the new session is asked nothing more
#   compact-background: then /compact, then a command that takes 40 s, and /background is typed while it
#               runs; when the new session's turn is over, this build's `handoff role implementer` is run
#               as the new session from P/proj
#   born-inside: instead of the above, the session starts in P/proj and takes the role; it is closed, and
#               resumed with `claude --resume <id>` from P; it runs `cd proj` and `handoff role`
#   inside:     then it is closed, and resumed with `claude --resume <id>` from P/proj; `handoff role` again;
#               closed, and resumed once more from P
import os, pty, re, select, sys, time, signal, json, subprocess, glob
D, mode = sys.argv[1], sys.argv[2]
P, proj, LOG, SHIM = D + "/P", D + "/P/proj", D + "/log/runs.jsonl", D + "/shim/handoff"
env = {k: os.environ[k] for k in ("HOME", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", PATH=D + "/shim:" + os.environ["PATH"],
           HANDOFF_TEST_NOTIFY=D + "/alerts.txt", HANDOFF_TEST_AWAKE=D + "/awake.txt")
home = os.environ["HOME"]
def enc(p): return "".join(ch if ch.isalnum() else "-" for ch in p)
def scrub(s):
    s = str(s)
    for a, b in ((proj, "<P/proj>"), (P, "<P>"), (D, "<scratch>"), (enc(proj), "<P/proj, as a folder name>"), (enc(P), "<P, as a folder name>"), (home, "~")):
        s = s.replace(a, b).replace(os.path.realpath(a), b)
    return s
def stamp(): return time.strftime("%H:%M:%S", time.gmtime())
def say(*a): print(stamp(), scrub(" ".join(str(x) for x in a)), flush=True)
screen = b""; pid = fd = None; ended = True
def start(args, cwd):
    global pid, fd, screen, ended
    screen = b""; ended = False
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(cwd)
        os.execvpe("claude", ["claude", "--model", "haiku", "--allowedTools", "Bash(cd:*)", "Bash(pwd)", "Bash(sleep:*)", "Bash(%s:*)" % SHIM] + args, env)
    say("## started: claude", " ".join(args), "in", cwd)
def reap():
    global ended
    if not ended:
        try:
            if os.waitpid(pid, os.WNOHANG) != (0, 0): ended = True; say("## the session's process has ended")
        except ChildProcessError: ended = True
    return not ended
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        reap()
        if ended: time.sleep(0.05); continue
        try: r, _, _ = select.select([fd], [], [], 0.05)
        except (OSError, ValueError): time.sleep(0.05); continue
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: time.sleep(0.05)
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")
def typ(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.02)
    pump(0.6); os.write(fd, b"\r")
def tail(n=10):
    # the screen's last lines from the session's last reply on, with the scratch folder's name taken out
    lines = [l.rstrip() for l in plain()[-4000:].splitlines() if l.strip()]
    at = max([i for i, l in enumerate(lines) if l.lstrip().startswith("\u23fa") and len(l.strip()) > 8] or [max(0, len(lines) - n)])
    print("\n".join("    | " + re.sub(r"\S*(claude-501|scratchpad|real31)\S*", "<a scratch path>", scrub(l))[:230] for l in lines[at:at + n]), flush=True)
def runs():
    out = []
    try:
        for l in open(LOG):
            try: out.append(json.loads(l))
            except ValueError: pass
    except OSError: pass
    return out
shown = 0
def show():
    # every run of handoff since the last call: who ran it, where, and what a brief or a command printed
    global shown
    rs = runs()
    for r in rs[shown:]:
        i = r.get("in") or {}
        line = "%s  handoff %-22s ran in %-9s CLAUDE_PROJECT_DIR=%-4s" % (r["t"], r["args"][:22], scrub(r["pwd"]), scrub(r["pdir"]) or "-")
        if i: line += " %s%s session=%s stdin cwd=%s transcript in %s" % (i.get("hook_event_name"), "(" + i["source"] + ")" if i.get("source") else "", i.get("session_id", "")[:8], scrub(i.get("cwd")), scrub(os.path.basename(os.path.dirname(i.get("transcript_path", "")))))
        print(line, flush=True)
        if r["args"].startswith("hook brief") or not i:
            try: out = open(r["f"] + ".out").read()
            except OSError: out = ""
            w = [l for l in out.splitlines() if l.startswith("handoff: warning")]
            first = (out.strip().splitlines() or ["(nothing)"])[0]
            print("            printed: %s%s" % (scrub(first)[:150], "\n            WARNED: " + scrub(w[0]) if w else "\n            no warning"), flush=True)
    shown = len(rs)
def wait_for(pred, secs, what, since=0):
    t = time.time()
    while time.time() - t < secs:
        hit = [r for r in runs()[since:] if pred(r)]
        if hit: return hit[-1]
        pump(0.3)
    say("## did not see:", what); return None
def ev(name, source=None, sid=None, other=None):
    def p(r):
        i = r.get("in") or {}
        return (r["args"].startswith("hook brief") if name == "SessionStart" else True) and i.get("hook_event_name") == name and (source is None or i.get("source") == source) and (sid is None or i.get("session_id") == sid) and (other is None or i.get("session_id") != other)
    return p
def transcripts(sid):
    for p in sorted(glob.glob(home + "/.claude/projects/*/" + sid + ".jsonl")):
        n = 0; first = None; cw = []
        for l in open(p, errors="replace"):
            n += 1
            try: c = json.loads(l).get("cwd")
            except Exception: c = None
            if c and first is None: first = (n, c)
            if c and (not cw or cw[-1] != c): cw.append(c)
        say("## transcript of %s in the folder %s: %d lines; the first cwd is on line %s: %s; the folders it names, in order: %s" % (sid[:8], os.path.basename(os.path.dirname(p)), n, first[0] if first else "-", first[1] if first else "-", " > ".join(cw)))
def binding():
    out = []
    for p in glob.glob(proj + "/.handoff/sessions/*.json"):
        b = json.load(open(p)); out.append("%s has %s (pid %s%s)" % (b["session"][:8], b["role"], "running" if b.get("pid") and alive(b["pid"]) else "ended", ", was " + ",".join(x[:8] for x in b["was"]) if b.get("was") else ""))
    return "; ".join(out) or "none"
def alive(p):
    try: os.kill(p, 0); return True
    except OSError: return False
def close():
    if reap():
        typ("/exit"); t = time.time()
        while time.time() - t < 12 and reap(): pump(0.1)
    if reap(): os.kill(pid, signal.SIGTERM); pump(3)
    if reap(): os.kill(pid, signal.SIGKILL); pump(1)
    pump(1.5)
def trust():
    pump(6)
    if "trust" in plain().lower() and "folder" in plain().lower():
        say("## the folder-trust question is shown; answering yes"); os.write(fd, b"\x1b[B"); pump(0.8); os.write(fd, b"\r"); pump(3)
def turn(text, sid, secs=90):
    n = len(runs()); typ(text)
    wait_for(ev("Stop", sid=sid), secs, "the turn's Stop", n); pump(2)

if mode == "born-inside":
    n0 = len(runs()); shown = n0
    start([], proj); trust(); first = wait_for(ev("SessionStart"), 40, "the session's SessionStart", n0)
    sid = first["in"]["session_id"]; say("## session", sid); show()
    turn("Run this exact shell command: %s role implementer   Then reply with the word ALPHA only." % SHIM, sid, 120); show(); say("## binding:", binding()); transcripts(sid)
    close(); show(); n = len(runs())
    start(["--resume", sid], P); trust(); c = wait_for(ev("SessionStart"), 40, "a SessionStart of the session resumed from P", n); pump(3); show()
    rid = c["in"]["session_id"] if c else sid; say("## resumed from P: the session is", rid, "(the same id: %s)" % (rid == sid), "| binding:", binding())
    turn("Run the shell command: cd proj   Then reply with the word BRAVO only.", rid); show()
    turn("Run this exact shell command: %s role   Then reply with the word CHARLIE only." % SHIM, rid, 120); show(); transcripts(rid); say("## binding:", binding())
    close(); show(); say("## done"); sys.exit(0)
n0 = len(runs()); shown = n0
start([], P); trust()
first = wait_for(ev("SessionStart"), 40, "the session's SessionStart", n0)
if not first: tail(); close(); sys.exit(1)
sid = first["in"]["session_id"]
say("## session", sid); show()
turn("Run the shell command: cd proj   Then reply with the word ALPHA only.", sid); show()
turn("Run this exact shell command: %s role implementer   Then say, in two short sentences, what the warning in its output means and what the operator should do." % SHIM, sid, 120)
show(); say("## what the session said:"); tail(12); say("## binding:", binding()); transcripts(sid)

if mode == "plain":
    close(); show(); n = len(runs())
    start(["--resume", sid], P); trust(); wait_for(ev("SessionStart", sid=sid), 40, "the resumed session's SessionStart", n); pump(3); show()
    say("## binding after the resume from P:", binding())
    r = subprocess.run([D + "/../bin31/handoff", "note", "--as", "reviewer", "--to", "implementer", "--item", "H1", "--title", "are you there", "--body", "-"], input="A note.\n", cwd=proj, capture_output=True, text=True, env=dict(env, HANDOFF_ROLE=""))
    say("## a note to implementer from the reviewer's side:", r.returncode, (r.stdout + r.stderr).strip().replace("\n", " | ")[:400])
    turn("Run the shell command: pwd   Then reply with its output only.", sid); show(); close(); show()

if mode == "clear":
    n = len(runs()); typ("/compact"); c = wait_for(ev("SessionStart", source="compact"), 120, "a SessionStart after /compact", n); pump(3); show()
    n = len(runs()); typ("/clear"); c = wait_for(ev("SessionStart", source="clear"), 40, "a SessionStart after /clear", n); pump(3); show()
    new = c["in"]["session_id"] if c else sid
    say("## after /clear the session is", new, "| binding:", binding()); transcripts(new)
    turn("Run this exact shell command: %s role   Then reply with the word BRAVO only." % SHIM, new); show(); transcripts(new)
    close(); show(); n = len(runs())
    start(["--resume", new], P); trust(); wait_for(ev("SessionStart", sid=new), 40, "the resumed session's SessionStart", n); pump(3); show()
    say("## binding after the resume from P:", binding()); close(); show()

if mode == "background":
    n = len(runs()); typ("/background"); t0 = time.time()
    while time.time() - t0 < 40 and reap(): pump(0.1)
    c = wait_for(ev("SessionStart", other=sid), 40, "the new session's SessionStart", n); pump(4); show()
    new = c["in"]["session_id"] if c else None
    say("## after /background the session is", new, "| binding:", binding())
    if new: transcripts(new)
    r = subprocess.run([D + "/../bin31/handoff", "note", "--as", "reviewer", "--to", "implementer", "--item", "H1", "--title", "are you there", "--body", "-"], input="Reply to this note with one word.\n", cwd=proj, capture_output=True, text=True, env=dict(env, HANDOFF_ROLE=""))
    say("## a note to implementer:", r.returncode, (r.stdout + r.stderr).strip().replace("\n", " | ")[:400])
    n = len(runs()); wait_for(ev("Stop", other=sid), 120, "a Stop of the new session", n); pump(3); show()
    say("## binding:", binding())
    if new:
        transcripts(new); print(new, file=open(D + "/bg-session.txt", "a"))

if mode == "compact-background":
    n = len(runs()); typ("/compact"); wait_for(ev("SessionStart", source="compact"), 120, "a SessionStart after /compact", n); pump(4); show()
    n = len(runs()); typ("Run this exact shell command in the foreground, not in the background, and wait for it to finish: sleep 40   Then reply with the word DELTA only.")
    pump(14); typ("/background"); t0 = time.time()
    while time.time() - t0 < 40 and reap(): pump(0.1)
    c = wait_for(ev("SessionStart", other=sid), 40, "the new session's SessionStart", n); pump(2); show()
    new = c["in"]["session_id"] if c else None
    say("## after /background the session is", new, "| binding:", binding())
    if new:
        print(new, file=open(D + "/bg-session.txt", "a"))
        wait_for(ev("Stop", sid=new), 120, "the new session's Stop", n); pump(3); show(); transcripts(sid); transcripts(new)
        r = subprocess.run([D + "/../bin31/handoff", "role", "implementer"], cwd=proj, capture_output=True, text=True, env=dict(env, CLAUDE_CODE_SESSION_ID=new, CLAUDE_PID=str(os.getpid())))
        w = [l for l in r.stdout.splitlines() if l.startswith("handoff: warning")]
        say("## this build's handoff role implementer, as the new session, from P/proj:", (r.stdout.splitlines() or [r.stderr.strip()])[0], "|", "WARNED" if w else "no warning")

if mode == "inside":
    close(); show(); n = len(runs())
    start(["--resume", sid], proj); trust(); c = wait_for(ev("SessionStart"), 40, "a SessionStart of the session resumed from P/proj", n); pump(3); show(); tail(8)
    if c:
        rid = c["in"]["session_id"]; say("## resumed from P/proj: the session is", rid, "(the same id: %s)" % (rid == sid)); transcripts(rid)
        turn("Run this exact shell command: %s role   Then reply with the word CHARLIE only." % SHIM, rid); show(); transcripts(rid)
        say("## binding:", binding()); close(); show(); n = len(runs())
        start(["--resume", rid], P); trust(); wait_for(ev("SessionStart"), 40, "a SessionStart of the session resumed from P again", n); pump(3); show(); tail(6); transcripts(rid); close(); show()
    else:
        close()
say("## done")
