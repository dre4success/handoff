# withtty.py <command> [args...]: runs a command with a pseudo-terminal on its stdin, stdout and stderr,
# as Antigravity runs its agent's commands. When the command has written nothing for 2 s it is taken to
# be asking something: the answer is "n" once, then end-of-input. It prints whether the command asked,
# the last line it wrote, and its exit status.
import os, pty, sys, select, time, signal
pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
out = b""; last = time.time(); asked = 0; st = None; asking = ""
while True:
    r, _, _ = select.select([fd], [], [], 0.2)
    if r:
        try: b = os.read(fd, 65536)
        except OSError: b = b""
        if not b: break
        out += b; last = time.time()
    done, st = os.waitpid(pid, os.WNOHANG)
    if done: break
    st = None
    if time.time() - last > 2:
        if asked == 0:
            asking = [l for l in out.decode(errors="replace").replace("\r", "").splitlines() if l.strip()][-1:]
            os.write(fd, b"n\n")
        elif asked == 1: os.write(fd, b"\x04")
        else: os.kill(pid, signal.SIGKILL)
        asked += 1; last = time.time()
if st is None: _, st = os.waitpid(pid, 0)
lines = [l for l in out.decode(errors="replace").replace("\r", "").splitlines() if l.strip()]
code = os.waitstatus_to_exitcode(st)
if asked: print("it asked \"%s\"; answered n; then exit %d: %s" % ((asking[0] if asking else "")[-60:], code, (lines[-1] if lines else "")[:110]))
else: print("exit %d: %s" % (code, (lines[0] if lines else "(nothing)")[:150]))
