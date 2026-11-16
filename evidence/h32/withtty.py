# Runs a command with a pseudo-terminal on its stdin and stdout, as Antigravity runs its agent's commands,
# and prints what it wrote and its exit status. Usage: python3 -I withtty.py <command> [args...]
import os, pty, sys
pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
out = b""
while True:
    try:
        b = os.read(fd, 65536)
    except OSError:
        break
    if not b:
        break
    out += b
_, st = os.waitpid(pid, 0)
sys.stdout.write(out.decode(errors="replace").replace("\r", ""))
print("[exit %d; stdin was a terminal]" % os.waitstatus_to_exitcode(st))
