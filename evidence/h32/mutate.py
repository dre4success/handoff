# Each of H32's conditions taken out in turn; the H32 tests must fail each time.
# Usage: python3 -I evidence/h32/mutate.py   (from the repository's root; restores every file)
import subprocess, sys
M = [
 ("C1 an Antigravity command is an agent's session", "cmd/handoff/session.go", 'return session{ID: os.Getenv("ANTIGRAVITY_CONVERSATION_ID")}', 'return session{}'),
 ("C2 the CLI is known as antigravity", "cmd/handoff/brief.go", 'case os.Getenv("ANTIGRAVITY_CONVERSATION_ID") != "":\n\t\treturn "antigravity"', 'case false:\n\t\treturn "antigravity"'),
 ("C3 Antigravity's input names the session", "cmd/handoff/hook.go", 'in.Session, in.Event, in.agy = in.Conversation, in.EventName, true', 'in.Event, in.agy = in.EventName, true'),
 ("C4 the project is the agent's folder", "cmd/handoff/hook.go", '\t\t\ta.cwd = in.Cwd\n', ''),
 ("C5 only an absolute folder", "cmd/handoff/hook.go", 'if filepath.IsAbs(in.Cwd) {', 'if in.Cwd != "" {'),
 ("C6 listen answers with a continue", "cmd/handoff/hook.go", 'map[string]string{"decision": "continue", "reason": says.String()}', 'map[string]string{"reason": says.String()}'),
 ("C7 listen is Antigravity's for its input", "cmd/handoff/hook.go", '\t\tif in.agy {\n\t\t\treturn a.agyListen(args[1:], in)\n\t\t}\n', ''),
 ("C8 the brief is a system message", "cmd/handoff/hook.go", '\t\tif in.agy {\n\t\t\treturn a.agyBrief(args[1:], in)\n\t\t}\n', ''),
 ("C9 nothing to say is {}", "cmd/handoff/hook.go", 'if err != nil || says.Len() == 0 {', 'if err != nil {'),
 ("C10 no listening at the limit", "cmd/handoff/hook.go", 'if in.Continuations >= agyContinuations {', 'if in.Continuations > agyContinuations {'),
 ("C11 the operator is told at the limit", "cmd/handoff/hook.go", '\t\ta.tellOperator(fmt.Sprintf("handoff can\'t wake', '\t\t_ = (fmt.Sprintf("handoff can\'t wake'),
 ("C12 settings knows antigravity", "cmd/handoff/settings.go", '&& pos[0] != "antigravity") {', ') {'),
 ("C13 settings fixes no role for it", "cmd/handoff/settings.go", '\t\tif *as != "" {\n\t\t\treturn 0, errors.New("--as fixes a role for Claude Code or Codex', '\t\tif false {\n\t\t\treturn 0, errors.New("--as fixes a role for Claude Code or Codex'),
 ("C14 its user hooks are in ~/.gemini/config", "cmd/handoff/setup.go", 'return filepath.Join(home, ".gemini", "config")', 'return filepath.Join(home, ".gemini")'),
 ("C15 its project hooks are in .agents/", "cmd/handoff/setup.go", 'filepath.Join(root, ".agents", "hooks.json")}', 'filepath.Join(root, ".agent", "hooks.json")}'),
 ("C16 a hook turned off doesn't listen", "cmd/handoff/setup.go", 'if h.Enabled != nil && !*h.Enabled {', 'if false {'),
 ("C17 only handoff's listen counts", "cmd/handoff/setup.go", 'if strings.Contains(c.Command, "hook listen") {', 'if c.Command != "" {'),
 ("C18 its brief by hand says not to wait", "cmd/handoff/brief.go", '"antigravity": fmt.Sprintf("don\'t run `%s wait`. Instead, %s", h, idle),', ''),
 ("C19 its brief offers its hooks", "cmd/handoff/brief.go", 'for _, c := range []string{"claude", "codex", "antigravity"} {', 'for _, c := range []string{"claude", "codex"} {'),
 ("C20 the operator's brief offers only the other two", "cmd/handoff/brief.go", 'if (cli == "" && c != "antigravity" || cli == c)', 'if (cli == "" || cli == c)'),
]
def run():
    r = subprocess.run(["go", "test", "-count=1", "-timeout", "120s", "-run", "TestH32", "./cmd/handoff/"], capture_output=True, text=True)
    fails = sorted({l.split()[2] for l in r.stdout.splitlines() if l.startswith("--- FAIL")})
    build = "build failed" in r.stdout + r.stderr or "[setup failed]" in r.stdout
    return r.returncode, fails, build
code, _, _ = run()
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
        code, fails, build = run()
    finally:
        open(path, "w").write(src)
    print("%s: %s" % (name, "DOESN'T BUILD" if build else ("fails " + ", ".join(fails)) if code != 0 else "PASSES (not held)"))
