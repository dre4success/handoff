# mutate.py: takes each H30 condition out in turn, runs the H30 tests, and
# puts the file back. Run from the repository root: python3 -I mutate.py
import subprocess, shutil
M = [
 ("the hook's transcript is named after its session", "cmd/handoff/session.go", '|| filepath.Base(transcript) != s.ID+".jsonl" {', '{'),
 ("exactly one binding qualifies", "cmd/handoff/session.go", "if len(from) != 1 {\n\t\treturn \"\", false, false", "if len(from) == 0 {\n\t\treturn \"\", false, false"),
 ("the line's type is continued-in", "cmd/handoff/session.go", '&& e.Type == "continued-in" {', '{'),
 ("the last such line wins", "cmd/handoff/session.go", 'if json.Unmarshal(line, &e) == nil && e.Type == "continued-in" {\n\t\t\tnext, at = e.Next, e.At', 'if json.Unmarshal(line, &e) == nil && e.Type == "continued-in" && next == "" {\n\t\t\tnext, at = e.Next, e.At'),
 ("no line older than the role", "cmd/handoff/session.go", '|| at.Before(since) {', '|| (false && at.Before(since)) {'),
 ("the hook waits for the old process", "cmd/handoff/session.go", 'return 5 * time.Second', 'return 0'),
 ("a running session keeps its role", "cmd/handoff/session.go", 'if b.running() {\n\t\treturn "", true, false', 'if false {\n\t\treturn "", true, false'),
 ("ids are plain names", "cmd/handoff/session.go", 'if !plainID.MatchString(id) {\n\t\t\treturn false', 'if id == "" {\n\t\t\treturn false'),
 ("an fyi for the session it went on from", "internal/wait/wait.go", '&& !slices.Contains(o.Was, e.Session) {', '&& !slices.Contains([]string(nil), e.Session) {'),
 ("a named pipe is never waited on", "cmd/handoff/session.go", 'os.O_RDONLY|syscall.O_NONBLOCK', 'os.O_RDONLY'),
 ("a hook finds the role another hook of its session moved", "cmd/handoff/session.go", "if b, ok := a.bound(s); ok {\n\t\treturn b.Role, false, true", "if b, ok := a.bound(s); ok && false {\n\t\treturn b.Role, false, true"),
 ("the move itself", "cmd/handoff/session.go", 'if in.Transcript != "" {\n\t\tif role, ok := a.moveContinued', 'if false {\n\t\tif role, ok := a.moveContinued'),
]
for label, path, old, new in M:
    src = open(path).read()
    assert src.count(old) == 1, label
    open(path, "w").write(src.replace(old, new))
    try:
        r = subprocess.run(["go", "test", "./cmd/handoff", "-run", "TestH30", "-count=1"], capture_output=True, text=True, timeout=600)
        failed = sorted({l.split()[2] for l in r.stdout.splitlines() if l.strip().startswith("--- FAIL")})
        if r.returncode and not failed: failed = [r.stdout.strip().splitlines()[-1] if r.stdout.strip() else r.stderr.strip()[:200]]
        print(f"{label}: {'FAIL ' + ', '.join(failed) if r.returncode else 'all pass (not held)'}")
    finally:
        open(path, "w").write(src)
