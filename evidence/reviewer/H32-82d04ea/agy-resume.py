# agy-resume.py <scratch dir> <conversation id>: the conversation of agy-limit.py, which has the reviewer
# role, is opened again with `agy --conversation <id>` and nothing is typed. Does anything listen, and what
# is a sender told? Then `agy -p --conversation <id>` (print mode) for the same conversation: does it end?
import os, pty, re, select, sys, time, signal, json, subprocess, glob
D, cid = sys.argv[1], sys.argv[2]
proj, LOG, BIN = D + "/proj", D + "/log/runs.jsonl", D + "/../bin32/handoff"
env = {k: os.environ[k] for k in ("HOME", "PATH", "USER", "LANG") if k in os.environ}
env.update(TERM="xterm-256color", SHELL="/bin/zsh", HANDOFF_TEST_NOTIFY=D + "/alerts.txt", HANDOFF_TEST_AWAKE=D + "/awake.txt")
def scrub(s): return re.sub(r"\S*(claude-501|scratchpad)\S*", "<a scratch path>", str(s).replace(proj, "<proj>").replace(D, "<scratch>"))
def say(*a): print(time.strftime("%H:%M:%S", time.gmtime()), scrub(" ".join(str(x) for x in a)), flush=True)
def nruns(): return sum(1 for _ in open(LOG))
def listeners(): return len(subprocess.run(["pgrep", "-f", "bin32/handoff hook listen"], capture_output=True, text=True).stdout.split())
def note(title):
    b = D + "/body.md"; open(b, "w").write("Probe note: %s (%s).\n" % (title, time.time()))
    r = subprocess.run([BIN, "note", "--to", "reviewer", "--item", "W1", "--title", title, "--body", b], cwd=proj, env=dict(env, HANDOFF_ROLE="implementer"), capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return (r.stdout + r.stderr).strip().replace("\n", " | ")
n0 = nruns()
pid, fd = pty.fork()
if pid == 0:
    os.chdir(proj); os.execvpe("agy", ["agy", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions", "--conversation", cid], env)
def pump(s):
    end = time.time() + s
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.1)
        if r:
            try: os.read(fd, 65536)
            except OSError: time.sleep(0.1)
say("## 1. agy --conversation <id>, nothing typed"); pump(20)
say("## 20 s on: runs of handoff by its hooks: %d | listeners running: %d" % (nruns() - n0, listeners()))
say("## a note to reviewer; all the sender is told:", note("after reopening"))
pump(15); say("## 15 s later: runs of handoff: %d | listeners running: %d" % (nruns() - n0, listeners()))
os.write(fd, b"\x03"); pump(1.5); os.write(fd, b"\x03"); pump(4)
try:
    if os.waitpid(pid, os.WNOHANG) == (0, 0): os.kill(pid, signal.SIGTERM); pump(2); os.waitpid(pid, 0)
except ChildProcessError: pass
say("## 2. agy -p --conversation <id> \"Reply with the word READY only.\", given 60 s")
n1 = nruns(); t = time.time()
p = subprocess.Popen(["agy", "-p", "Reply with the word READY only.", "--model", "gemini-3.8-flash-low", "--dangerously-skip-permissions", "--conversation", cid], cwd=proj, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
try:
    out, _ = p.communicate(timeout=60); say("## it ended after %.0f s, exit %s; its last line: %s" % (time.time() - t, p.returncode, (out.strip().splitlines() or [""])[-1][:120]))
except subprocess.TimeoutExpired:
    say("## still running after 60 s | runs of handoff by its hooks: %d | listeners running: %d" % (nruns() - n1, listeners()))
    say("## a note to reviewer:", note("to the print run"))
    try: out, _ = p.communicate(timeout=45); say("## it ended %.0f s after it began, exit %s; its output's lines: %s" % (time.time() - t, p.returncode, " / ".join(l.strip()[:80] for l in out.strip().splitlines()[-4:])))
    except subprocess.TimeoutExpired:
        p.kill(); p.communicate(); say("## still running 45 s after the note; killed")
time.sleep(2); say("## listeners running:", listeners()); say("## done")
