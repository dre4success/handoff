# Run a command with a pseudo-terminal on stdin and stdout, as the operator
# types it, and print what it wrote.
import os, pty, sys
pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
out = b""
while True:
    try:
        b = os.read(fd, 4096)
    except OSError:
        break
    if not b:
        break
    out += b
os.waitpid(pid, 0)
sys.stdout.write(out.decode(errors="replace").replace("\r\n", "\n"))
