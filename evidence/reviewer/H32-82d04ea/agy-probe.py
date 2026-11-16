# agy-probe.py <scratch dir>: one throwaway interactive Antigravity session through a pseudo-terminal,
# in <scratch dir>/proj, a git repository after `handoff init` whose .agents/hooks.json is what this
# build's `handoff settings antigravity` printed, with shim/handoff in the build's place. The shim notes
# each run and runs the build. shim/handoff-base runs the base's build (2967c9a) the same way.
#   1. the session takes the reviewer role and ends its turn; the Stop hook listens
#   2. Esc; it runs the operator's commands: fyi, away, back, decide, handoff on its own, and note --as
#      operator, with this build; then fyi with the base's build
#   3. a request posted from outside wakes it
#   4. Esc while it listens: is the listener gone? A message is answered; then a second request wakes it
#      in the same process
import os, pty, re, select, sys, time, signal, json, subprocess, glob
D = sys.argv[1]
proj, LOG, SHIM, BASE, BIN = D + "/proj", D + "/log/runs.jsonl", D + "/shim/handoff", D + "/shim/handoff-base", D + "/../bin32/handoff"
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=D + "/alerts.txt", HANDOFF_TEST_AWAKE=D + "/awake.txt")
def scrub(s):
    s = str(s)
    for a, b in ((SHIM + "-base", "handoff-base"), (SHIM, "handoff"), (proj, "<proj>"), (D, "<scratch>"), (os.environ["HOME"], "~")):
        s = s.replace(a, b).replace(os.path.realpath(a), b)
    return re.sub(r"\S*(claude-501|scratchpad)\S*", "<a scratch path>", s)
def stamp(): return time.strftime("%H:%M:%S", time.gmtime())
def say(*a): print(stamp(), scrub(" ".join(str(x) for x in a)), flush=True)
screen = b""
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj); os.execvpe("agy", ["agy", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions"], env)
def pump(s):
    global screen
    end = time.time() + s
    while time.time() < end:
        try: r, _, _ = select.select([fd], [], [], 0.05)
        except (OSError, ValueError): time.sleep(0.05); continue
        if r:
            try: screen += os.read(fd, 65536)
            except OSError: time.sleep(0.05)
def plain(): return re.sub(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b.", b"", screen).decode(errors="replace")
def typ(s):
    for ch in s: os.write(fd, ch.encode()); pump(0.02)
    pump(0.6); os.write(fd, b"\r")
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
    global shown
    rs = runs()
    for r in rs[shown:]:
        i = r.get("in") or {}
        names = r["names"].split()
        line = "%s  %-12s %-34s stdin a terminal: %-3s ANTIGRAVITY_CONVERSATION_ID: %-3s (of %d ANTIGRAVITY_ names) parent is agy: %s" % (r["t"], r["bin"], r["args"][:34], r["tty"], "yes" if "ANTIGRAVITY_CONVERSATION_ID" in names else "no", len(names), r["ppid"] == pid)
        if i: line += " | %s executionNum=%s stdin cwd=%s, ran in %s" % (i.get("hookEventName"), i.get("executionNum", "-"), scrub(i.get("cwd")), scrub(r["pwd"]))
        print(line, flush=True)
        if not i:
            time.sleep(0.3)
            try: code = open(r["f"] + ".code").read().strip()
            except OSError: code = "?"
            try: out = (open(r["f"] + ".out").read() + open(r["f"] + ".err").read()).strip().splitlines()
            except OSError: out = []
            print("            exit %s: %s" % (code, scrub(out[0] if out else "(nothing)")[:170]), flush=True)
    shown = len(rs)
def wait_for(pred, secs, what, since):
    t = time.time()
    while time.time() - t < secs:
        hit = [r for r in runs()[since:] if pred(r)]
        if hit: return hit[-1]
        pump(0.3)
    say("## did not see:", what); return None
def stop_hook(r): return (r.get("in") or {}).get("hookEventName") == "Stop"
def ran(args): return lambda r: not r.get("in") and r["args"].startswith(args)
def listeners():
    r = subprocess.run(["pgrep", "-f", "bin32/handoff hook listen"], capture_output=True, text=True)
    return sorted(int(x) for x in r.stdout.split())
def binding():
    out = []
    for p in glob.glob(proj + "/.handoff/sessions/*.json"):
        b = json.load(open(p)); out.append("%s has %s (host %s)" % (b["session"][:8], b["role"], "is agy" if b.get("host") == pid else b.get("host") or b.get("pid") or "none"))
    return "; ".join(out) or "none"
def hb(*args, role):
    e = dict(env, HANDOFF_ROLE=role)
    r = subprocess.run([BIN] + list(args), cwd=proj, env=e, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return scrub(((r.stdout + r.stderr).strip().splitlines() or [""])[0])
body = D + "/body.md"; open(body, "w").write("A probe request. Nothing to review.\n")
commit = subprocess.run(["git", "rev-parse", "w1"], cwd=proj, capture_output=True, text=True).stdout.strip()
n0 = len(runs()); shown = n0
pump(8)
if re.search(r"trust", plain(), re.I): say("## Antigravity asks whether to trust the folder; Enter"); os.write(fd, b"\r"); pump(4)
say("## 1. agy runs; the session is told its role")
typ("You're the reviewer in handoff. Run this exact shell command: %s role reviewer   Then, for this test only: whenever handoff brings you work, run %s show <id> for it and reply with the word GOT and the entry's title, nothing else, and don't claim or review it. Now end your turn." % (SHIM, SHIM))
wait_for(ran("role reviewer"), 90, "handoff role reviewer", n0); wait_for(stop_hook, 60, "the first Stop hook", n0); pump(3); show()
say("## binding:", binding(), "| listeners of this build running:", len(listeners()))
say("## 2. Esc, then the operator's commands, run by the agent")
n = len(runs()); os.write(fd, b"\x1b"); pump(2)
cmds = ["%s fyi hello" % SHIM, "%s away gone" % SHIM, "%s back" % SHIM, "%s decide W1 --approve" % SHIM, "%s" % SHIM, "%s note --as operator --to implementer --item W1 --title hello --body %s" % (SHIM, body), "%s fyi hello" % BASE]
typ("Run each of these %d shell commands, one at a time, in this order, each one even if an earlier one fails, and change nothing in them. Then reply with the word DONE only. " % len(cmds) + " ".join("(%d) %s" % (i + 1, c) for i, c in enumerate(cmds)))
wait_for(lambda r: r["bin"] == "handoff-base" and not r.get("in"), 180, "the last command", n); wait_for(stop_hook, 60, "the turn's Stop hook", n); pump(6); show()
say("## the record's entries now:", hb("log", role="reviewer") if False else subprocess.run(["python3", "-I", "-c", "import json,sys; print('; '.join('#%s %s %s>%s' % (e.get('id'), e.get('kind'), e.get('from'), e.get('to')) for e in map(json.loads, open(sys.argv[1]))))", proj + "/.handoff/channel.jsonl"], capture_output=True, text=True).stdout.strip())
n = len(runs()); wait_for(stop_hook, 60, "a Stop hook after the fyi was delivered", n); pump(3); show()
say("## 3. a request posted from outside:", hb("post", "--item", "W1", "--to", "reviewer", "--commit", commit, "--title", "probe one", "--body", body, role="implementer"))
n = len(runs()); wait_for(ran("show"), 90, "handoff show, run by the woken session", n); wait_for(stop_hook, 60, "the Stop hook after it", n); pump(3); show()
say("## binding:", binding(), "| listeners running:", listeners() and len(listeners()))
say("## 4. Esc while it listens")
before = listeners(); os.write(fd, b"\x1b"); pump(3); after = listeners()
say("## listeners before Esc: %d; 3 s after: %d (the same process still there: %s)" % (len(before), len(after), bool(set(before) & set(after))))
n = len(runs()); typ("Reply with the word HELLO only."); wait_for(stop_hook, 60, "the Stop hook after HELLO", n); pump(3); show()
say("## HELLO on the screen:", "HELLO" in plain()[-3000:], "| listeners running:", len(listeners()), "(a new process: %s)" % bool(set(listeners()) - set(before)))
say("## a second request:", hb("post", "--item", "W2", "--to", "reviewer", "--commit", commit, "--title", "probe two", "--body", body, role="implementer"))
n = len(runs()); wait_for(ran("show"), 90, "handoff show, run by the session woken a second time", n); wait_for(stop_hook, 60, "the Stop hook after it", n); pump(3); show()
say("## binding:", binding(), "| listeners running:", len(listeners()))
cid = None
for p in glob.glob(proj + "/.handoff/sessions/*.json"): cid = json.load(open(p))["session"]
print(cid, file=open(D + "/conversation.txt", "a"))
os.write(fd, b"\x1b"); pump(1.5); os.write(fd, b"\x03"); pump(1.5); os.write(fd, b"\x03"); pump(4)
try:
    if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(2); os.waitpid(pid, 0)
except ChildProcessError: pass
time.sleep(2); say("## agy closed; listeners running:", len(listeners()))
open(D + "/screen.txt", "w").write(plain())
# the conversation's own transcript: the kinds of step, in order, with handoff's words where they are
for t in glob.glob(os.environ["HOME"] + "/.gemini/antigravity-cli/brain/%s/.system_generated/logs/transcript_full.jsonl" % cid):
    say("## the conversation's transcript, its steps in order:")
    for l in open(t, errors="replace"):
        try: e = json.loads(l)
        except ValueError: continue
        s = json.dumps(e)
        kind = e.get("type") or e.get("stepType") or e.get("kind") or "?"
        m = re.search(r"(handoff: new for reviewer|has no role in it|handoff brief: you are reviewer|never in an agent's session|told reviewer|GOT[^\"\\]{0,30}|HELLO|DONE)", s)
        print("    %s%s" % (kind, ": " + m.group(1) if m else ""), flush=True)
say("## done")
