# Run a command with a pseudo-terminal on stdin and stdout, answer its one
# question with "y", and print what it wrote. For the reviewer's probes of
# `handoff decide`, which asks "[y/N]" at a terminal.
import os, pty, sys, time
pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
out = b""; sent = False
while True:
    try:
        b = os.read(fd, 4096)
    except OSError:
        break
    if not b:
        break
    out += b
    if not sent and b"[y/N]" in out:
        os.write(fd, b"y\n"); sent = True
os.waitpid(pid, 0)
sys.stdout.write(out.decode(errors="replace").replace("\r\n", "\n"))
