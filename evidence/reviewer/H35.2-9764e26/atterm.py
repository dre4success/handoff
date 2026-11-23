# Run a command with a pseudo-terminal on stdin and stdout, answer each "[y/N]"
# it asks with the next letter of the first argument ("y", "n", "yy", "yn", ...),
# and print what it wrote. If it asks more questions than there are answers,
# the command is killed after 5 s and that is said.
import os, pty, sys, time, select, signal
answers = list(sys.argv[1]); pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[2], sys.argv[2:])
out = b""; seen = 0; t0 = time.time(); killed = False
while True:
    r, _, _ = select.select([fd], [], [], 0.2)
    if not r:
        if out.count(b"[y/N]") > seen + 0 and not answers and time.time() - t0 > 5:
            os.kill(pid, signal.SIGKILL); killed = True; break
        continue
    try:
        b = os.read(fd, 4096)
    except OSError:
        break
    if not b:
        break
    out += b
    while out.count(b"[y/N]") > seen and answers:
        os.write(fd, (answers.pop(0) + "\n").encode()); seen += 1; t0 = time.time()
os.waitpid(pid, 0)
sys.stdout.write(out.decode(errors="replace").replace("\r\n", "\n"))
if killed: print("(atterm: an unanswered question; killed)")
