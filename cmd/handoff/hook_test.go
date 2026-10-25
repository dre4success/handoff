package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/wait"
)

// H10a: sessions keep themselves going (proposal §7, §8, §17 row 67). A
// real Claude Code session driving these hooks is in evidence/h10/.

// start runs handoff in the background as role, with stdin as given.
func (r *repo) start(t *testing.T, role, stdin string, extraEnv []string, args ...string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.dir
	e := env(extraEnv...)
	if role != "" {
		e = append(e, "HANDOFF_ROLE="+role)
	}
	cmd.Env = e
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd, &out, &errb
}

func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %s within 10 s", what)
		}
	}
}

// listening reports who presence/<role>.json names, once it names pid.
func (r *repo) listening(t *testing.T, role string, pid int) func() bool {
	return func() bool {
		p, err := wait.ReadPresence(filepath.Join(r.dir, ".handoff"), role)
		return err == nil && p.PID == pid
	}
}

// A wait holds its role's lock. A second wait for that role exits 4 at once
// and says who listens; another role's wait is unaffected; and a killed
// wait frees the lock.
func TestH10SecondWaitForARoleExits(t *testing.T) {
	r := newRepo(t)
	first, _, _ := r.start(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "30s")
	eventually(t, "presence for the first wait", r.listening(t, "reviewer", first.Process.Pid))
	began := time.Now()
	res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "20s")
	if res.code != 4 || !strings.Contains(res.err, fmt.Sprintf("already listening: wait for reviewer, pid %d", first.Process.Pid)) || time.Since(began) > 5*time.Second {
		t.Fatalf("second wait: exit %d after %s, stderr %q", res.code, time.Since(began), res.err)
	}
	if res := r.run(t, "", "", nil, "wait", "--for", "implementer", "--timeout", "1s"); res.code != 3 {
		t.Fatalf("another role's wait: exit %d, stderr %q", res.code, res.err)
	}
	first.Process.Kill()
	first.Wait()
	if res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "1s"); res.code != 3 {
		t.Fatalf("a wait after the first was killed: exit %d, stderr %q", res.code, res.err)
	}
}

// hook listen, as Claude Code's asyncRewake hook: an entry for its role
// wakes it, and it exits 2 with the entry for the model on stderr. With
// nothing new by its deadline it also exits 2, asking the session to end
// its turn, so listening never just lapses.
func TestH10HookListenWakesTheSession(t *testing.T) {
	r := newRepo(t)
	cmd, out, errb := r.start(t, "reviewer", `{"hook_event_name":"Stop","stop_hook_active":false}`, nil, "hook", "listen", "--timeout", "30s")
	eventually(t, "the hook listening", r.listening(t, "reviewer", cmd.Process.Pid))
	if p, _ := wait.ReadPresence(filepath.Join(r.dir, ".handoff"), "reviewer"); p.How != "hook" {
		t.Fatalf("presence %+v; want how hook", p)
	}
	posted := time.Now()
	req := r.request(t, "W1", r.commitA)
	code := exitCode(cmd.Wait())
	if woke := time.Since(posted); code != 2 || woke > 3*time.Second {
		t.Fatalf("hook: exit %d, %s after the post; stderr %q", code, woke, errb.String())
	}
	if e := errb.String(); out.Len() != 0 || !strings.Contains(e, "handoff: new for reviewer:\n#"+req+" request W1 implementer → reviewer") || !strings.Contains(e, "show <id>") {
		t.Fatalf("stdout %q, stderr %q", out.String(), e)
	}
	res := r.run(t, "reviewer", "{}", nil, "hook", "listen", "--timeout", "1s")
	if res.code != 2 || !strings.Contains(res.err, "nothing new for reviewer in 1s. Just end your turn") {
		t.Fatalf("the next listen, with nothing new: exit %d, stderr %q", res.code, res.err)
	}
}

// While a wait listens for the role, hook listen leaves it to that one:
// exit 0 at once, nothing printed, so the session is not woken.
func TestH10HookListenLeavesItToTheListener(t *testing.T) {
	r := newRepo(t)
	first, _, _ := r.start(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "30s")
	eventually(t, "presence for the wait", r.listening(t, "reviewer", first.Process.Pid))
	began := time.Now()
	res := r.run(t, "reviewer", "{}", nil, "hook", "listen", "--timeout", "20s")
	if res.code != 0 || res.out != "" || res.err != "" || time.Since(began) > 5*time.Second {
		t.Fatalf("hook listen: exit %d after %s, stdout %q, stderr %q", res.code, time.Since(began), res.out, res.err)
	}
}

// When the session that started it dies, the hook stops listening and its
// lock is free. sh stands in for Claude Code: it starts the hook and exits.
func TestH10HookListenStopsWithItsSession(t *testing.T) {
	r := newRepo(t)
	sh := exec.Command("/bin/sh", "-c", `"$0" hook listen --timeout 60s </dev/null >/dev/null 2>&1 & echo $!; sleep 1`, bin)
	sh.Dir, sh.Env = r.dir, env("HANDOFF_ROLE=reviewer")
	b, err := sh.Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("pid %q: %v", b, err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	if !r.listening(t, "reviewer", pid)() {
		t.Fatal("the hook was not listening while its session lived")
	}
	eventually(t, "the hook's exit after its session died", func() bool { return syscall.Kill(pid, 0) != nil })
	if res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "1s"); res.code != 3 {
		t.Fatalf("a wait after the hook stopped: exit %d, stderr %q", res.code, res.err)
	}
}

// A hook acts only for a session on duty here. With no role, outside a
// project, or as the operator, every hook exits 0 and prints nothing.
func TestH10HooksOffDuty(t *testing.T) {
	r := newRepo(t)
	outside := &repo{dir: t.TempDir()}
	for _, c := range []struct {
		r    *repo
		role string
	}{{r, ""}, {outside, "reviewer"}, {r, "operator"}} {
		for _, h := range []string{"listen", "brief", "notify"} {
			res := c.r.run(t, c.role, `{"notification_type":"permission_prompt","message":"x"}`, nil, "hook", h)
			if res.code != 0 || res.out != "" || res.err != "" {
				t.Fatalf("hook %s, role %q, in %s: exit %d, stdout %q, stderr %q", h, c.role, c.r.dir, res.code, res.out, res.err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(r.dir, ".handoff", "presence")); !os.IsNotExist(err) {
		t.Fatalf("a hook off duty took a lock: %v", err)
	}
}

// hook brief prints the brief, which reaches the model: the role, that a
// hook listens, the duties, and what waits. A reviewing role gets the review
// duty; a role that never reviews does not. `brief` on its own tells the
// session to run wait itself. A wrong role is said to the model.
func TestH10Brief(t *testing.T) {
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	res := r.run(t, "reviewer", `{"source":"compact"}`, nil, "hook", "brief")
	for _, want := range []string{"you are reviewer in ", "a hook listens for reviewer", "A request to you:", "needs my review:\n  #" + req + " request W1"} {
		if res.code != 0 || !strings.Contains(res.out, want) {
			t.Fatalf("hook brief: exit %d, missing %q in\n%s", res.code, want, res.out)
		}
	}
	res = r.run(t, "implementer", "", nil, "brief")
	if res.code != 0 || !strings.Contains(res.out, "wait --for implementer --timeout 25m`") || strings.Contains(res.out, "A request to you:") {
		t.Fatalf("brief for implementer: exit %d\n%s", res.code, res.out)
	}
	res = r.run(t, "nobody", "{}", nil, "hook", "brief")
	if res.code != 0 || !strings.Contains(res.out, "not on duty") || !strings.Contains(res.out, `unknown role "nobody"`) {
		t.Fatalf("hook brief, wrong role: exit %d, stdout %q", res.code, res.out)
	}
	r.refused(t, "", "for an agent's role", "brief", "--for", "operator")
}

// Every command the brief gives runs through a shell as printed, once its
// placeholders are filled in: show, claim, reply, a note back, and a
// question to the operator (GPT-6, H10a round 2: the question lacked --item;
// evidence/reviewer/H10a-d320393/ask-operator.txt).
func TestH10BriefCommandsRun(t *testing.T) {
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	body := r.body(t, "a question")
	fill := strings.NewReplacer("<id>", req, "<that sha>", r.commitA, "<file>", body, "<role>", "implementer",
		"<item>", "W1", "<title>", "Findings", "<question>", "A question", "APPROVE|CHANGES|QUESTION", "APPROVE")
	ran, last, brief := briefCommands(t, r, "reviewer", fill)
	if got := strings.Join(ran, " "); got != "show claim reply note note note" {
		t.Fatalf("ran %q from the brief:\n%s", got, brief)
	}
	if !strings.Contains(string(last), "note W1 reviewer → operator: A question") {
		t.Fatalf("the question to the operator: %s", last)
	}
}

// briefCommands runs, in order, every command in the brief that a session
// on duty as role gets, with fill's placeholders filled, through /bin/sh as
// that role; each must exit 0. It skips wait, and the `post ... --supersedes`
// hint, which isn't a full command. It returns each command's name, the
// last one's output, and the brief.
func briefCommands(t *testing.T, r *repo, role string, fill *strings.Replacer) (ran []string, last []byte, brief string) {
	t.Helper()
	brief = r.run(t, role, "{}", nil, "hook", "brief").out
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatal(err)
	}
	h := shellQuote(real) // as the brief names it
	for i, span := range strings.Split(brief, "`") {
		if i%2 == 0 || !strings.HasPrefix(span, h+" ") || strings.HasPrefix(span, h+" wait") || strings.Contains(span, "...") {
			continue
		}
		cmd := exec.Command("/bin/sh", "-c", fill.Replace(span))
		cmd.Dir, cmd.Env = r.dir, env("HANDOFF_ROLE="+role)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("the brief's `%s`: %v\n%s", span, err, out)
		}
		ran, last = append(ran, strings.Fields(strings.TrimPrefix(span, h+" "))[0]), out
	}
	return ran, last, brief
}

// hook notify tells the operator when a session on duty stops at a permission
// prompt, and at nothing else.
func TestH10HookNotify(t *testing.T) {
	r := newRepo(t)
	path, envs := sink(t)
	prompt := `{"notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}`
	if res := r.run(t, "reviewer", prompt, envs, "hook", "notify"); res.code != 0 {
		t.Fatalf("exit %d, stderr %q", res.code, res.err)
	}
	want := filepath.Base(r.dir) + ": reviewer is waiting for your permission: Claude needs your permission to use Bash"
	if got := sentTo(t, path); len(got) != 1 || got[0] != want {
		t.Fatalf("sent %q, want %q", got, want)
	}
	r.run(t, "reviewer", `{"notification_type":"idle_prompt","message":"waiting"}`, envs, "hook", "notify")
	r.run(t, "", prompt, envs, "hook", "notify")
	if got := sentTo(t, path); len(got) != 1 {
		t.Fatalf("sent %q; want only the permission prompt of the session on duty", got)
	}
}

// A listen that cannot work never wakes the session, since each wake ends
// in another Stop and would loop. It tells the operator, once, however many turns
// meet the same trouble.
func TestH10ListenTroubleIsToldOnce(t *testing.T) {
	r := newRepo(t)
	path, envs := sink(t)
	// A cursor nobody can read stands in for a record gone wrong.
	cursors := filepath.Join(r.dir, ".handoff", "cursors")
	if err := os.MkdirAll(cursors, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursors, "reviewer.json"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		// Exit 0 does not wake the session; what it printed is not shown to it.
		if res := r.run(t, "reviewer", "{}", envs, "hook", "listen", "--timeout", "5s"); res.code != 0 {
			t.Fatalf("hook listen in trouble: exit %d, stderr %q", res.code, res.err)
		}
	}
	if got := sentTo(t, path); len(got) != 1 || !strings.Contains(got[0], "reviewer's session stopped listening: cursor for reviewer") {
		t.Fatalf("sent %q; want the trouble once", got)
	}
}

// settings claude prints settings that parse: the role for every session,
// the three hooks, and rules that allow handoff but not decide, and keep
// hands off .handoff/. They name handoff as the brief does.
func TestH10SettingsClaude(t *testing.T) {
	r := newRepo(t)
	res := r.run(t, "", "", nil, "settings", "claude", "--as", "reviewer")
	at := strings.Index(res.out, "\n{")
	if res.code != 0 || at < 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", res.code, res.out, res.err)
	}
	type handler struct {
		Type        string   `json:"type"`
		Command     string   `json:"command"`
		Args        []string `json:"args"`
		AsyncRewake bool     `json:"asyncRewake"`
		Timeout     int      `json:"timeout"`
	}
	type group struct {
		Matcher string    `json:"matcher"`
		Hooks   []handler `json:"hooks"`
	}
	var s struct {
		Env         map[string]string  `json:"env"`
		Hooks       map[string][]group `json:"hooks"`
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(res.out[at+1:]), &s); err != nil {
		t.Fatalf("%v in\n%s", err, res.out)
	}
	h, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatal(err)
	}
	listen := handler{"command", h, []string{"hook", "listen"}, true, 86400}
	brief := handler{"command", h, []string{"hook", "brief"}, false, 0}
	notify := handler{"command", h, []string{"hook", "notify"}, false, 0}
	same := func(g []group, want group) bool {
		if len(g) != 1 || g[0].Matcher != want.Matcher || len(g[0].Hooks) != len(want.Hooks) {
			return false
		}
		for i := range want.Hooks {
			x, y := g[0].Hooks[i], want.Hooks[i]
			if x.Type != y.Type || x.Command != y.Command || strings.Join(x.Args, " ") != strings.Join(y.Args, " ") || x.AsyncRewake != y.AsyncRewake || x.Timeout != y.Timeout {
				return false
			}
		}
		return true
	}
	if s.Env["HANDOFF_ROLE"] != "reviewer" ||
		!same(s.Hooks["SessionStart"], group{"", []handler{brief, listen}}) ||
		!same(s.Hooks["Stop"], group{"", []handler{listen}}) ||
		!same(s.Hooks["Notification"], group{"permission_prompt", []handler{notify}}) ||
		strings.Join(s.Permissions.Allow, "|") != "Bash("+h+" *)" ||
		strings.Join(s.Permissions.Deny, "|") != "Bash("+h+" decide *)|Edit(/"+r.record(t)+"/**)" {
		t.Fatalf("settings:\n%s", res.out)
	}
	// Without --as, no role is fixed: each session takes the one the operator gives it
	// (row 73), and the hooks are the same.
	res = r.run(t, "", "", nil, "settings", "claude")
	s.Env = nil
	if err := json.Unmarshal([]byte(res.out[strings.Index(res.out, "\n{")+1:]), &s); err != nil || s.Env != nil ||
		!same(s.Hooks["SessionStart"], group{"", []handler{brief, listen}}) || !same(s.Hooks["Stop"], group{"", []handler{listen}}) ||
		!strings.Contains(res.out, "takes the role you give it") {
		t.Fatalf("settings claude, no --as: %v\n%s", err, res.out)
	}
	r.refused(t, "", "settings needs a CLI", "settings", "cursor", "--as", "reviewer")
	r.refused(t, "", `unknown role "operator"`, "settings", "claude", "--as", "operator")
}

// A hook stopped (as a sleeping Mac or a debugger stops it) while its
// session dies must not deliver when it resumes: nobody would read it, and
// the cursor would move past it. It exits, and the next listener gets the
// entry (GPT-6's probe, evidence/reviewer/H10a-ebc04f2/orphan.txt).
func TestH10ResumedOrphanLeavesTheEntry(t *testing.T) {
	r := newRepo(t)
	sh := exec.Command("/bin/sh", "-c", `"$0" hook listen --timeout 60s </dev/null >/dev/null 2>&1 & echo $!; sleep 30`, bin)
	sh.Dir, sh.Env = r.dir, env("HANDOFF_ROLE=reviewer")
	out, err := sh.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { sh.Process.Kill(); sh.Wait() }()
	var line [32]byte
	n, _ := out.Read(line[:])
	pid, err := strconv.Atoi(strings.TrimSpace(string(line[:n])))
	if err != nil {
		t.Fatalf("pid %q: %v", line[:n], err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	eventually(t, "the hook listening", r.listening(t, "reviewer", pid))
	syscall.Kill(pid, syscall.SIGSTOP)
	sh.Process.Kill() // the session dies while its hook is stopped
	sh.Wait()
	req := r.request(t, "W1", r.commitA)
	syscall.Kill(pid, syscall.SIGCONT)
	eventually(t, "the orphan's exit", func() bool { return syscall.Kill(pid, 0) != nil })
	res := r.run(t, "", "", nil, "wait", "--for", "reviewer", "--timeout", "2s")
	if res.code != 0 || !strings.Contains(res.out, "#"+req+" request W1") {
		t.Fatalf("the next listener: exit %d, stdout %q, stderr %q; the orphan took the entry", res.code, res.out, res.err)
	}
}

// installed copies the test binary to dir/handoff and returns its path.
func installed(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "handoff")
	if err := os.WriteFile(path, b, 0o755); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// Installed under a path with a space, handoff still prints commands a
// shell can run as given, in the brief and in the permission rules
// (GPT-6's probe, evidence/reviewer/H10a-ebc04f2/spaces.txt).
func TestH10APathWithSpaces(t *testing.T) {
	r := newRepo(t)
	req := r.request(t, "W1", r.commitA)
	h := installed(t, filepath.Join(t.TempDir(), "install with spaces"))
	brief := exec.Command(h, "hook", "brief")
	brief.Dir, brief.Env, brief.Stdin = r.dir, env("HANDOFF_ROLE=reviewer"), strings.NewReader("{}")
	out, err := brief.Output()
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(out), "read it with `")
	show, _, ok2 := strings.Cut(rest, "`")
	if !ok || !ok2 || !strings.HasPrefix(show, "'"+h+"' show") {
		t.Fatalf("brief:\n%s", out)
	}
	sh := exec.Command("/bin/sh", "-c", strings.ReplaceAll(show, "<id>", req))
	sh.Dir, sh.Env = r.dir, env()
	if got, err := sh.CombinedOutput(); err != nil || !strings.Contains(string(got), "#"+req+" request W1") {
		t.Fatalf("sh -c %q: %v\n%s", show, err, got)
	}
	settings := exec.Command(h, "settings", "claude", "--as", "reviewer")
	settings.Dir, settings.Env = r.dir, env()
	if out, err := settings.Output(); err != nil || !strings.Contains(string(out), `"Bash('`+h+`' *)"`) {
		t.Fatalf("settings: %v\n%s", err, out)
	}
}

// record is the record's real path.
func (r *repo) record(t *testing.T) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(filepath.Join(r.dir, ".handoff"))
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// In a linked worktree, the printed rules keep hands off the record every
// worktree shares, not a .handoff/ under the worktree's own folder (GPT-6's
// probe, evidence/reviewer/H10a-ebc04f2/worktree.txt).
func TestH10SettingsInALinkedWorktree(t *testing.T) {
	r := newRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, r.dir, "worktree", "add", "-q", "--detach", linked, r.commitA)
	res := (&repo{dir: linked}).run(t, "", "", nil, "settings", "claude", "--as", "reviewer")
	want := `"Edit(/` + r.record(t) + `/**)"`
	if res.code != 0 || !strings.Contains(res.out, want) || strings.Contains(res.out, `"Write(`) {
		t.Fatalf("exit %d; want %s in\n%s%s", res.code, want, res.out, res.err)
	}
}
