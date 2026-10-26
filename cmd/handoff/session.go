package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// Roles handed to running sessions (proposal §17 rows 73, 74). The operator tells a
// running Claude Code or Codex session its role, and the session runs
// `handoff role <role>`. The role is kept in .handoff/sessions/ for that
// session alone, so two sessions of one CLI, on any models, can hold two
// roles in one project.
//
// A session is known by the id its CLI gives every command
// (CLAUDE_CODE_SESSION_ID, CODEX_THREAD_ID) and every hook on stdin. A
// resumed session keeps its id. Claude Code also gives CLAUDE_PID, its own
// process, which hosts one conversation at a time: /clear starts a new id in
// it, with a SessionStart whose source is "clear", and only then does a role
// move to the new id. Codex gives nothing of the kind: its hooks' parent may
// be a server for many threads (GPT-6, H12 review), so a new thread starts
// with no role, and the parent only tells whether a session may still run.

// session is the agent session a command or hook runs in.
type session struct {
	ID   string // the CLI's session id; "" outside an agent session
	PID  int    // Claude Code's process for this session (CLAUDE_PID); 0 for Codex
	Host int    // a Codex hook's parent, which may host other sessions too; 0 if unknown
}

// callerSession is the session a command runs in, from its environment.
func callerSession() session {
	if id := os.Getenv("CLAUDE_CODE_SESSION_ID"); id != "" {
		pid, _ := strconv.Atoi(os.Getenv("CLAUDE_PID"))
		return session{ID: id, PID: pid}
	}
	return session{ID: os.Getenv("CODEX_THREAD_ID")}
}

// hookInput is what both CLIs write to a hook's stdin, as far as handoff
// reads it.
type hookInput struct {
	Session string `json:"session_id"`
	Event   string `json:"hook_event_name"`
	Source  string `json:"source"`
	Type    string `json:"notification_type"`
	Message string `json:"message"`
}

// hookSession is the session a hook runs for: the id on its stdin; Claude
// Code's process from CLAUDE_PID, or else the hook's parent, as a host.
func hookSession(in hookInput) session {
	s := callerSession()
	if in.Session != "" {
		s.ID = in.Session
	}
	if s.PID == 0 {
		s.Host = os.Getppid()
	}
	return s
}

// binding is one session's role, .handoff/sessions/<session>.json.
type binding struct {
	V       int    `json:"v"`
	Role    string `json:"role"`
	Session string `json:"session"`
	PID     int    `json:"pid,omitempty"`
	Host    int    `json:"host,omitempty"`
	Since   string `json:"since"`
}

// running says whether b's session may still run: no when its process, or
// the process that hosts it, has ended.
func (b binding) running() bool {
	switch {
	case b.PID > 0:
		return alive(b.PID)
	case b.Host > 0:
		return alive(b.Host)
	}
	return true
}

const sessionsDir = "sessions"

var plainID = regexp.MustCompile(`^[A-Za-z0-9-]{1,100}$`)

func (a *app) bindingPath(id string) string {
	name := id
	if !plainID.MatchString(id) {
		sum := sha256.Sum256([]byte(id))
		name = "h-" + hex.EncodeToString(sum[:16])
	}
	return filepath.Join(a.dir, sessionsDir, name+".json")
}

// bindings reads every binding. One that cannot be read is skipped: it
// binds nobody.
func (a *app) bindings() ([]binding, error) {
	ents, err := os.ReadDir(filepath.Join(a.dir, sessionsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []binding
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.dir, sessionsDir, e.Name()))
		if err != nil {
			continue
		}
		var bd binding
		if json.Unmarshal(b, &bd) == nil && bd.Session != "" && bd.Role != "" {
			out = append(out, bd)
		}
	}
	return out, nil
}

// alive says whether a process exists.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// hasSession says whether a session has role, as far as handoff can tell:
// the operator gave it to one that may still run, or something listens for
// it now. init makes roles the operator may never give, such as a second
// reviewer (row 89).
func (a *app) hasSession(role string) bool {
	bs, _ := a.bindings()
	for _, b := range bs {
		if b.Role == role && b.running() {
			return true
		}
	}
	return wait.Listening(a.dir, role)
}

// bound finds s's binding, by its id.
func (a *app) bound(s session) (binding, bool) {
	if s.ID == "" {
		return binding{}, false
	}
	bs, _ := a.bindings()
	for _, b := range bs {
		if b.Session == s.ID {
			return b, true
		}
	}
	return binding{}, false
}

// holds is a check that s still holds role by the operator's word. A listener whose
// role was resolved from s's binding takes it from that same resolution, so
// a role that moves at any moment, even before the listener starts, stops
// it (row 75).
func (a *app) holds(s session, role string) func() bool {
	return func() bool {
		b, ok := a.bound(s)
		return ok && b.Role == role
	}
}

// startupDelay is for tests only: a pause after a listener resolves its role
// and before it takes the lock, where a role can move (row 75).
func startupDelay() {
	if d, err := time.ParseDuration(os.Getenv("HANDOFF_TEST_STARTUP_DELAY")); err == nil {
		time.Sleep(d)
	}
}

// listen takes role's listener lock. For a role held by the operator's word, it
// waits up to three seconds for it: a role just moved here, and its earlier
// holder's listener gives the lock up within a second.
func (a *app) listen(role, how string, held bool) (*os.File, error) {
	lock, err := wait.Listen(a.dir, role, how)
	for try := 0; held && errors.Is(err, wait.ErrListening) && try < 12; try++ {
		time.Sleep(250 * time.Millisecond)
		lock, err = wait.Listen(a.dir, role, how)
	}
	return lock, err
}

// lockSessions serialises changes to the bindings.
func (a *app) lockSessions() (unlock func(), err error) {
	dir := filepath.Join(a.dir, sessionsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// bind gives s the role. It drops s's earlier binding (and, for Claude
// Code, any other in the same process, whose conversation /clear ended),
// the role's binding to another session (took says whether that one may
// still run), and bindings whose session has ended.
func (a *app) bind(s session, role string) (took bool, err error) {
	unlock, err := a.lockSessions()
	if err != nil {
		return false, err
	}
	defer unlock()
	bs, err := a.bindings()
	if err != nil {
		return false, err
	}
	var drop []binding
	for _, b := range bs {
		mine := b.Session == s.ID || (s.PID > 0 && b.PID == s.PID)
		switch {
		case mine:
			drop = append(drop, b)
		case b.Role == role:
			drop = append(drop, b)
			took = took || b.running()
		case !b.running():
			drop = append(drop, b)
		}
	}
	for _, b := range drop {
		if err := os.Remove(a.bindingPath(b.Session)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	return took, a.writeBinding(binding{V: 1, Role: role, Session: s.ID, PID: s.PID, Host: s.Host, Since: time.Now().UTC().Format(time.RFC3339)})
}

func (a *app) writeBinding(b binding) error {
	data, _ := json.Marshal(b)
	return record.WriteFileAtomic(a.bindingPath(b.Session), append(data, '\n'))
}

// sessionRole is the role a hook's session holds, with its binding brought
// up to date: a new process or host noted, as after a resume. At a start:
// after Claude Code's /clear, the role of the one binding in the same
// process moves to the new id. A session with no role gets none (row 80).
func (a *app) sessionRole(s session, in hookInput) (string, bool) {
	if s.ID == "" {
		return "", false
	}
	if b, ok := a.bound(s); ok {
		if (b.PID != s.PID || b.Host != s.Host) && !a.update(s, b.Role) {
			return "", false // the role moved meanwhile
		}
		return b.Role, true
	}
	if in.Event == "SessionStart" && in.Source == "clear" && s.PID > 0 {
		if role, ok := a.moveCleared(s); ok {
			return role, true
		}
	}
	return "", false
}

// update notes s's process and host in its binding, and says whether s
// still holds role.
func (a *app) update(s session, role string) bool {
	unlock, err := a.lockSessions()
	if err != nil {
		return true // the note waits for another hook
	}
	defer unlock()
	b, ok := a.bound(s) // again, under the lock
	if !ok || b.Role != role {
		return false
	}
	b.PID, b.Host = s.PID, s.Host
	a.writeBinding(b)
	return true
}

// moveCleared moves the role of the one binding in s's Claude Code process
// to s's new id.
func (a *app) moveCleared(s session) (string, bool) {
	unlock, err := a.lockSessions()
	if err != nil {
		return "", false
	}
	defer unlock()
	bs, _ := a.bindings()
	var same []binding
	for _, b := range bs {
		if b.PID == s.PID {
			same = append(same, b)
		}
	}
	if len(same) != 1 {
		return "", false
	}
	b := same[0]
	os.Remove(a.bindingPath(b.Session))
	b.Session = s.ID
	return b.Role, a.writeBinding(b) == nil
}

// cmdRole gives the calling session its role, or shows it.
func (a *app) cmdRole(args []string) (int, error) {
	fs := flag.NewFlagSet("role", flag.ContinueOnError)
	pos, err := parse(fs, args)
	if err == nil && len(pos) > 1 {
		err = errors.New("give one role")
	}
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	s := callerSession()
	if s.ID == "" {
		return 0, errors.New("handoff role is for a Claude Code or Codex session: the operator tells a session its role, and the session runs it")
	}
	project := filepath.Base(filepath.Dir(a.dir))
	if len(pos) == 0 {
		role, err := a.role("")
		if err != nil {
			return 0, err
		}
		fmt.Fprintf(a.stdout, "You are %s in %s.\n", role, project)
		return exitOK, nil
	}
	name := pos[0]
	if name == status.Operator {
		return 0, errors.New("operator is the person who decides; give the session one of the agents' roles")
	}
	if _, ok := a.cfg.role(name); !ok {
		return 0, fmt.Errorf("unknown role %q; the roles are %s", name, a.roleNames())
	}
	if env := os.Getenv("HANDOFF_ROLE"); env != "" && env != name {
		return 0, fmt.Errorf("this session's settings fix its role to %s (HANDOFF_ROLE); refused", env)
	}
	took, err := a.bind(s, name)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(a.stdout, "You are now %s in %s.", name, project)
	if took {
		fmt.Fprintf(a.stdout, " Another session had %s; it has no role now.", name)
	}
	fmt.Fprint(a.stdout, "\n\n")
	return exitOK, a.writeBrief(a.stdout, name, listenMaybe)
}
