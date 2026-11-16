# Each of H31's conditions, as of round 3, taken out in turn; the H31 tests must fail each time.
# Usage: python3 -I evidence/h31/mutate.py   (from the repository's root; restores every file)
import subprocess, sys
M = [
 ("C1 role warns when taking a role", "cmd/handoff/session.go", 'fmt.Fprint(a.stdout, "\\n\\n")\n\ta.warnOutside(a.stdout, s, "")', 'fmt.Fprint(a.stdout, "\\n\\n")'),
 ("C2 role, asked, warns", "cmd/handoff/session.go", 'role, project)\n\t\ta.warnOutside(a.stdout, s, "")', 'role, project)'),
 ("C3 brief by hand warns", "cmd/handoff/brief.go", '\ta.warnOutside(a.stdout, callerSession(), "")\n', ''),
 ("C4 the start brief warns", "cmd/handoff/hook.go", '\ta.warnOutside(a.stdout, hookSession(in), in.Transcript)\n', ''),
 ("C5 a Claude Code hook's run folder is CLAUDE_PROJECT_DIR", "cmd/handoff/outside.go", 'if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" && s.PID > 0 {', 'if d := ""; d != "" && s.PID > 0 {'),
 ("C6 CLAUDE_PROJECT_DIR only for Claude Code", "cmd/handoff/outside.go", 'if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" && s.PID > 0 {', 'if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {'),
 ("C7 a Claude Code command's run folder is its process's", "cmd/handoff/outside.go", 'if s.PID > 0 {\n\t\treturn processFolder(s.PID), "claude"', 'if false {\n\t\treturn processFolder(s.PID), "claude"'),
 ("C8 lsof's folder line", "cmd/handoff/outside.go", 'strings.CutPrefix(line, "n")', 'strings.CutPrefix(line, "p")'),
 ("C9 a hook's rollout only when it names the session", "cmd/handoff/outside.go", 'if transcript == "" || !rolloutOf(transcript, s.ID) {', 'if transcript == "" {'),
 ("C10 Codex's words", "cmd/handoff/outside.go", '\tif cli == "codex" {\n\t\twhen, resume', '\tif cli == "" {\n\t\twhen, resume'),
 ("C11 a path a shell would split is quoted", "cmd/handoff/outside.go", 'if strings.ContainsAny(root, ', 'if false && strings.ContainsAny(root, '),
 ("C12 the folder must be absolute", "cmd/handoff/outside.go", 'if !filepath.IsAbs(dir) {', 'if dir == "" {'),
 ("C13 no repository is outside", "cmd/handoff/outside.go", 'if gitx.NoRepository(err) {', 'if false {'),
 ("C14 any other git failure says nothing", "cmd/handoff/outside.go", 'return "", cli // git can\'t say', 'return dir, cli // git can\'t say'),
 ("C15 the same record is inside", "cmd/handoff/outside.go", 'if samePath(rec, a.dir) {', 'if rec == "" {'),
 ("C16 only a line whose own type is turn_context", "cmd/handoff/outside.go", 'e.Type == "turn_context" && e.Payload.Cwd != ""', 'e.Payload.Cwd != ""'),
 ("C17 a second, longer read", "cmd/handoff/outside.go", 'range []int64{1 << 20, 16 << 20}', 'range []int64{1 << 20}'),
 ("C18 16 MiB at most", "cmd/handoff/outside.go", 'range []int64{1 << 20, 16 << 20}', 'range []int64{1 << 20, 32 << 20}'),
 ("C19 a file is opened without waiting", "cmd/handoff/outside.go", 'os.O_RDONLY|syscall.O_NONBLOCK', 'os.O_RDONLY|0*syscall.O_NONBLOCK'),
 ("C20 the latest rollout counts", "cmd/handoff/outside.go", 'st.ModTime().After(at)', 'at.IsZero()'),
 ("C21 an id is never a pattern", "cmd/handoff/outside.go", 'if !plainID.MatchString(id) {', 'if false {'),
 ("C22 Codex's rollouts are found", "cmd/handoff/outside.go", '"sessions", "*", "*", "*", "rollout-*-"', '"sessions", "*", "*", "rollout-*-"'),
 ("C23 an Antigravity session isn't looked for (round 3)", "cmd/handoff/outside.go", 'if sessionCLI() == "antigravity" {', 'if false {'),
 ("C24 a spaced turn_context is found (round 3)", "cmd/handoff/outside.go", 'marker := []byte(`"turn_context"`)', 'marker := []byte(`"type":"turn_context"`)'),
]
def run():
    r = subprocess.run(["go", "test", "-count=1", "-timeout", "90s", "-run", "TestH31", "./cmd/handoff/"], capture_output=True, text=True)
    fails = sorted({l.split()[2] for l in r.stdout.splitlines() if l.startswith("--- FAIL")})
    build = "[build failed]" in r.stdout + r.stderr or "[setup failed]" in r.stdout
    return r.returncode, fails, "panic: test timed out" in r.stdout + r.stderr, build
code, _, _, _ = run()
print("unchanged: exit %d" % code)
for name, path, *pairs in M:
    src = open(path).read()
    out = src
    for old, new in zip(pairs[::2], pairs[1::2]):
        if out.count(old) != 1:
            print("%s: the text to change was not found once" % name); sys.exit(1)
        out = out.replace(old, new)
    open(path, "w").write(out)
    try:
        code, fails, timeout, build = run()
    finally:
        open(path, "w").write(src)
    print("%s: %s%s" % (name, "DOESN'T BUILD" if build else ("fails " + ", ".join(fails)) if code != 0 else "PASSES (not held)", " (timed out)" if timeout else ""))
