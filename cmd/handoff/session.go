package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
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
// move to the new id. /background goes on in a new id in another process; the
// old transcript's last lines name it, and the role follows once the old
// process has ended (row 99). Codex gives nothing of the kind: its hooks' parent may
// be a server for many threads (GPT-6, H12 review), so a new thread starts
// with no role, and the parent only tells whether a session may still run.

// session is the agent session a command or hook runs in.
type session struct {
	ID   string // the CLI's session id; "" outside an agent session
	PID  int    // Claude Code's process for this session (CLAUDE_PID); 0 for Codex
	Host int    // a Codex hook's parent, which may host other sessions too; 0 if unknown
}

// callerSession is the session a command runs in, from its environment.
// Antigravity gives every command its agent runs, and every hook, the
// conversation's id (row 102). Its commands run with a terminal on stdin, so
// without the id, decide would take an Antigravity agent for the operator.
func callerSession() session {
	if id := os.Getenv("CLAUDE_CODE_SESSION_ID"); id != "" {
		pid, _ := strconv.Atoi(os.Getenv("CLAUDE_PID"))
		return session{ID: id, PID: pid}
	}
	if id := os.Getenv("CODEX_THREAD_ID"); id != "" {
		return session{ID: id}
	}
	return session{ID: os.Getenv("ANTIGRAVITY_CONVERSATION_ID")}
}

// hookInput is what both CLIs write to a hook's stdin, as far as handoff
// reads it.
type hookInput struct {
	Session string `json:"session_id"`
	Event   string `json:"hook_event_name"`
	Source  string `json:"source"`
	Type    string `json:"notification_type"`
	Message string `json:"message"`
	// Transcript is the session's transcript, which Claude Code gives every hook.
	Transcript string `json:"transcript_path"`

	// Antigravity names the session and the event otherwise, and gives the
	// folder the agent is in, and how many times in a row a hook has continued
	// its turn (row 102).
	Conversation  string `json:"conversationId"`
	EventName     string `json:"hookEventName"`
	Cwd           string `json:"cwd"`
	Continuations int    `json:"executionNum"`
	// FullyIdle is Antigravity's word that no task of the agent's own runs:
	// false while one does, such as its own build in the background. nil
	// when the input doesn't say (row 110).
	FullyIdle *bool `json:"fullyIdle"`
	agy       bool  // the input is Antigravity's
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
	// Was is the sessions this one went on from, by /background (row 100).
	Was []string `json:"was,omitempty"`
	// Gen is this binding's generation: a new one each time a role is given
	// to a session, so a write or a delivery started under one can tell, at
	// the moment it lands, that the role has moved since (row 107).
	Gen int64 `json:"gen,omitempty"`
	// Lost marks a session whose role moved to another while it might run
	// on, or be resumed: it holds no role, and every command from it is
	// refused until the operator gives it one again (row 107).
	Lost *lostMark `json:"lost,omitempty"`
	// Gone says a later bind found the session's process ended. The binding
	// is kept, so its role can be marked lost if it is given on, and its pid
	// is dropped, so a new process given the same pid can't take it for its
	// own (the reviewer, #372). A resume, by its id, notes its new process.
	Gone bool `json:"gone,omitempty"`
	// Listened is when a listener of this binding, at its generation, last
	// took the role's lock; Block is what handoff knows keeps the session
	// from taking its work, such as Antigravity's cap (row 110).
	Listened string `json:"listened,omitempty"`
	Block    *block `json:"block,omitempty"`
	// Answered is when the hook of the last permission prompt a command of
	// the session cleared saw it: a report from no later can't block the
	// session again (reviewer-2, #596).
	Answered string `json:"answered,omitempty"`
	// CLI is the CLI the session runs in, when the role was given: "claude",
	// "codex" or "antigravity" (row 112).
	CLI string `json:"cli,omitempty"`
}

// lostMark says which role a session lost, to which session, and when.
type lostMark struct {
	Role string `json:"role"`
	To   string `json:"to"`
	At   string `json:"at"`
	Gen  int64  `json:"gen,omitempty"`
	Told bool   `json:"told,omitempty"` // its hooks have said so once
	// Went says how the role went on with the session's conversation, to
	// session To, when it did, and not by the operator's word: "/clear" or
	// "/background". The old id can be resumed, and is marked so it can't act
	// as the role next to it (reviewer-2, #388).
	Went string `json:"went,omitempty"`
	// Cut says the role's move to this session, by Went, was cut short
	// between its two writes: this session never held it (reviewer-2, #408).
	Cut bool `json:"cut,omitempty"`
}

// lostErr is what every command from a session that lost its role is told.
func lostErr(m *lostMark) error {
	at := m.At
	if t, err := time.Parse(time.RFC3339Nano, m.At); err == nil {
		at = t.UTC().Format(time.RFC3339)
	}
	how := fmt.Sprintf("the operator gave it to session %s at %s", m.To, at)
	switch {
	case m.Cut:
		how = fmt.Sprintf("its move to this session, by %s at %s, was cut short, so this session doesn't hold it", m.Went, at)
	case m.Went != "":
		how = fmt.Sprintf("this conversation went on in session %s at %s, by %s", m.To, at, m.Went)
	}
	return fmt.Errorf("you no longer hold %s: %s. Stop work on it, and don't merge. Take a role again only if the operator gives you one in this session: handoff role <role>", m.Role, how)
}

// running says whether b's session may still run: no when its process, or
// the process that hosts it, has ended.
func (b binding) running() bool {
	switch {
	case b.Gone:
		return false
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
	bs, _, err := a.sessionFiles()
	return bs, err
}

// spareBinding is the binding of the spare, if a running session holds it
// (H35).
func (a *app) spareBinding() (binding, bool) {
	bs, _ := a.bindings()
	for _, b := range bs {
		if a.isSpare(b.Role) && b.running() {
			return b, true
		}
	}
	return binding{}, false
}

// errAlreadySpare is bind's answer to role spare in the spare's own session.
var errAlreadySpare = errors.New("already the spare")

// spareCheck is run by bind, under its locks, before it gives s the spare.
// In the spare's own session nothing changes, since a new binding would
// give it a new generation and end the cover the away entry names (the
// reviewer, #744): only a session resumed since gets its process noted, as
// a command's first run does, so it counts as running (reviewer-3, #804).
// A session that has held another role here is refused.
func (a *app) spareCheck(s session) error {
	if b, ok := a.readBinding(s.ID); ok && a.isSpare(b.Role) && b.Lost == nil {
		if b.Gone || b.PID != s.PID || b.Host != s.Host {
			b.PID, b.Host, b.Gone = s.PID, s.Host, false
			if err := a.writeBinding(b); err != nil {
				return err
			}
		}
		return errAlreadySpare
	}
	held, err := a.heldAnother(s)
	if err != nil {
		return err
	}
	if held != "" {
		return fmt.Errorf("this session has been %s in %s, so it can't be the spare: the spare may take any seat, and must not review or build on what it has seen. The operator can start a new session for it", held, filepath.Base(filepath.Dir(a.dir)))
	}
	return nil
}

// heldAnother is a role other than the spare that session s has held in
// this project, or "": by s's own file, or the file of another conversation
// in its Claude Code process, which /clear ended. A file in
// .handoff/sessions/ is never deleted (row 107), and a move by /clear or
// /background carries the role, or its mark, to the new id, so a session's
// lineage counts as that session (H35, "Who may take a seat").
func (a *app) heldAnother(s session) (string, error) {
	bound, marked, err := a.sessionFiles()
	if err != nil {
		return "", err
	}
	for _, b := range append(bound, marked...) {
		// A move of a role to s by /clear or /background, cut short
		// between its two writes, leaves only the old id's mark naming s:
		// s's lineage all the same, as markOf takes it (reviewer-3, #804).
		if b.Lost != nil && b.Lost.Went != "" && b.Lost.To == s.ID && !a.isSpare(b.Lost.Role) {
			return b.Lost.Role, nil
		}
		if b.Session != s.ID && (s.PID <= 0 || b.PID != s.PID) {
			continue
		}
		if b.Role != "" && !a.isSpare(b.Role) {
			return b.Role, nil
		}
		if b.Lost != nil && !a.isSpare(b.Lost.Role) {
			return b.Lost.Role, nil
		}
	}
	return "", nil
}

// sessionFiles reads every session file once: the bindings that hold a role,
// and the marks. One pass, so a file rewritten from one to the other while it
// runs, as a move by /clear or /background rewrites the old id's, is seen as
// one or the other, never both, nor neither (reviewer-3, #450).
func (a *app) sessionFiles() (bound, marked []binding, err error) {
	ents, err := os.ReadDir(filepath.Join(a.dir, sessionsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.dir, sessionsDir, e.Name()))
		if err != nil {
			continue
		}
		var bd binding
		switch {
		case json.Unmarshal(b, &bd) != nil || bd.Session == "":
		case bd.Role != "":
			bound = append(bound, bd)
		case bd.Lost != nil:
			marked = append(marked, bd)
		}
	}
	return bound, marked, nil
}

// marks reads every session marked as having lost its role. They hold no
// role, so bindings leaves them out (row 107).
func (a *app) marks() []binding {
	_, ms, _ := a.sessionFiles()
	return ms
}

// tellLost returns s's mark the first time a hook asks, noting that it was
// told, and nil after: a session that lost its role is woken once to hear
// it, and then left quiet (row 107).
func (a *app) tellLost(s session) *lostMark {
	if _, ok := a.marked(s); !ok {
		return nil
	}
	unlock, err := a.lockSessions()
	if err != nil {
		return nil
	}
	defer unlock()
	b, ok := a.readBinding(s.ID) // again, under the lock
	if !ok || b.Role != "" || b.Lost == nil || b.Lost.Told {
		return nil
	}
	b.Lost.Told = true
	if a.writeBinding(b) != nil {
		return nil
	}
	return b.Lost
}

// readBinding reads one session's binding file, marked or not.
func (a *app) readBinding(id string) (binding, bool) {
	if id == "" || a.dir == "" {
		return binding{}, false
	}
	data, err := os.ReadFile(a.bindingPath(id))
	if err != nil {
		return binding{}, false
	}
	var b binding
	if json.Unmarshal(data, &b) != nil || b.Session != id {
		return binding{}, false
	}
	return b, true
}

// marked returns the mark of s, if s lost its role and was given none since.
func (a *app) marked(s session) (*lostMark, bool) {
	b, ok := a.readBinding(s.ID)
	return a.markOf(b, ok, s.ID)
}

// settledMark is marked, read under the sessions' lock, which a move by
// /clear or /background holds for both its writes: so a hook never takes a
// move under way for one cut short, and never tells a session holding its
// role to stop work (the reviewer, #442). A command's refusal reads
// unlocked: a late "stop" is worse in a hook than a refusal is there.
func (a *app) settledMark(s session) (*lostMark, bool) {
	unlock, err := a.lockSessions()
	if err != nil {
		return a.marked(s)
	}
	defer unlock()
	return a.marked(s)
}

// markOf is the mark of session id, from one reading of its binding, b, ok:
// its own, or, with no binding, the mark of a move to it cut short, which
// its next hook writes. No binding is deleted, so a session with none that a
// "went on" mark names is one whose move was cut short, and a command from
// it is refused as its hook would mark it (the reviewer, #427).
func (a *app) markOf(b binding, ok bool, id string) (*lostMark, bool) {
	switch {
	case ok && b.Role == "" && b.Lost != nil:
		return b.Lost, true
	case ok || id == "":
		return nil, false
	}
	for _, m := range a.marks() {
		if m.Lost.Went != "" && m.Lost.To == id {
			return &lostMark{Role: m.Lost.Role, To: id, At: m.Lost.At, Gen: m.Lost.Gen, Went: m.Lost.Went, Cut: true}, true
		}
	}
	return nil, false
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
	_, ok := a.holder(role)
	return ok
}

// holder is who has role, as hasSession reads it: the session the operator
// gave it to, while that may still run, or, with no such session, "" when
// something listens for it now. Giving a role to a session takes it from any
// other (bind), so there is at most one. An fyi names this session, so
// that only it gets the fyi (row 93).
func (a *app) holder(role string) (session string, ok bool) {
	bs, _ := a.bindings()
	for _, b := range bs {
		if b.Role == role && b.running() {
			return b.Session, true
		}
	}
	return "", wait.Listening(a.dir, role)
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

// holds is a check that s still holds the role of start, the binding its
// listener's role was resolved from, at start's generation: so a role that
// moves at any moment, even before the listener starts, stops it (row 75,
// row 107). With no binding, s holds its fixed role while it isn't marked
// (reviewer-2, #388).
func (a *app) holds(s session, start binding, held bool) func() bool {
	return func() bool {
		b, ok := a.readBinding(s.ID)
		if !held {
			// A session made the spare since this wait began listens for
			// nothing else (reviewer-2, #817): the wait retires, and lets the
			// role's lock go.
			if ok && a.isSpare(b.Role) && b.Lost == nil {
				return false
			}
			if _, lost := a.markOf(b, ok, s.ID); !lost {
				return true
			}
			// Lost, read with no lock: read again under the sessions' lock,
			// which a move by /clear or /background holds for both its
			// writes, so a listener never stops for a move under way (the
			// reviewer, #451).
			_, lost := a.settledMark(s)
			return !lost
		}
		return ok && b.Role == start.Role && b.Gen == start.Gen
	}
}

// waitFence is a wait's Fence for s, from start, the binding its role was
// resolved from, read once. Each delivery runs under the record's lock,
// shared, which a role's move takes for writing. held says s holds role by
// start: then each delivery needs that role at start's generation, so an old
// listener can't take an entry meant for its successor (row 107). A wait of
// s for a role it doesn't hold, or holds fixed by --as or HANDOFF_ROLE,
// still stops if s is marked meanwhile (reviewer-3, #378; reviewer-2, #388).
func (a *app) waitFence(s session, role string, start binding, held bool) func(func() error) error {
	return func(deliver func() error) error {
		testPause("deliver") // a delivery held before its lock
		unlock, err := a.store.LockRead()
		if err != nil {
			return err
		}
		defer unlock()
		b, ok := a.readBinding(s.ID)
		if held && (!ok || b.Role != role || b.Gen != start.Gen) {
			return wait.ErrFenced
		}
		if _, lost := a.markOf(b, ok, s.ID); !held && lost {
			return wait.ErrFenced
		}
		// Nor does it deliver, under the lock, once its session is the
		// spare (reviewer-2, #817).
		if !held && ok && a.isSpare(b.Role) && b.Lost == nil {
			return wait.ErrFenced
		}
		return deliver()
	}
}

// testPause is for tests only: a pause point, named in HANDOFF_TEST_PAUSE
// as name=dir, or name@n=dir for its nth hit in this process, several
// separated by commas. There the process writes dir/name.paused and waits,
// up to a minute, for dir/name.go: so a test acts while it is held at the
// point, and lets it go once it has, with no sleep for a loaded machine to
// outlast (H36). Once let go, it passes the point freely.
func testPause(name string) {
	v := os.Getenv("HANDOFF_TEST_PAUSE")
	if v == "" {
		return
	}
	pauseMu.Lock()
	pauseHits[name]++
	hit := pauseHits[name]
	pauseMu.Unlock()
	for _, p := range strings.Split(v, ",") {
		point, dir, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		n, at, _ := strings.Cut(point, "@")
		if n != name || (at != "" && at != strconv.Itoa(hit)) {
			continue
		}
		gone := filepath.Join(dir, name+".go")
		os.WriteFile(filepath.Join(dir, name+".paused"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
		for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			if _, err := os.Stat(gone); err == nil {
				return
			}
		}
		return
	}
}

var (
	pauseMu   sync.Mutex
	pauseHits = map[string]int{}
)

// startupDelay is for tests only: a pause after a listener resolves its role
// and before it takes the lock, where a role can move (row 75).
func startupDelay() {
	if d, err := time.ParseDuration(os.Getenv("HANDOFF_TEST_STARTUP_DELAY")); err == nil {
		time.Sleep(d)
	}
}

// listen takes role's listener lock. For a role held by the operator's word, it
// tries for it for about six seconds: 13 tries 250 ms apart, each of which
// wait.Listen makes for about 0.3 s. A role just moved here, and its earlier
// holder's listener gives the lock up within a second (the reviewer's note on
// the count, #650).
func (a *app) listen(role, how string, held bool) (*os.File, error) {
	lock, err := wait.Listen(a.dir, role, how)
	for try := 0; held && errors.Is(err, wait.ErrListening) && try < 12; try++ {
		time.Sleep(250 * time.Millisecond)
		lock, err = wait.Listen(a.dir, role, how)
	}
	return lock, err
}

// lockRoles takes the locks a role's move needs: the sessions', then the
// record's for writing, as bind always has. Every write by an agent, and
// every delivery, reads its binding under the record's lock, so it lands
// wholly before a move or sees it, whether the operator moved the role or
// /clear or /background did (row 107; reviewer-2, #388).
func (a *app) lockRoles() (unlock func(), err error) {
	unlockSessions, err := a.lockSessions()
	if err != nil {
		return nil, err
	}
	if a.store == nil {
		return unlockSessions, nil
	}
	unlockRecord, err := a.store.LockWrite()
	if err != nil {
		unlockSessions()
		return nil, err
	}
	return func() { unlockRecord(); unlockSessions() }, nil
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

// bind gives s the role. It replaces s's earlier binding, marks any other
// in the same Claude Code process, whose conversation /clear ended, and
// marks the session that had the role (took says whether that one may still
// run). A binding whose session has ended is kept, marked gone. Nothing is
// deleted.
func (a *app) bind(s session, role string) (took bool, err error) {
	unlock, err := a.lockRoles()
	if err != nil {
		return false, err
	}
	defer unlock()
	if a.isSpare(role) {
		// Decided under the locks that write the binding, so a role given
		// to s meanwhile is neither missed nor overwritten (reviewer-2,
		// #803; reviewer-3, #804).
		if err := a.spareCheck(s); err != nil {
			return false, err
		}
	}
	bs, err := a.bindings()
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	var drop, lose, gone []binding
	for _, b := range bs {
		mine := b.Session == s.ID || (s.PID > 0 && b.PID == s.PID)
		switch {
		case mine:
			drop = append(drop, b)
		case b.Role == role:
			// The session that had the role is marked, not forgotten,
			// running or not: a session can be resumed under its id.
			lose = append(lose, b)
			took = took || b.running()
		case !b.Gone && !b.running():
			// Kept, not deleted, so the role can still be marked lost if
			// it is given on (the reviewer, #372).
			gone = append(gone, b)
		}
	}
	for _, b := range gone {
		b.PID, b.Host, b.Gone = 0, 0, true
		if err := a.writeBinding(b); err != nil {
			return false, err
		}
	}
	for _, b := range drop {
		if b.Session == s.ID {
			continue // rewritten below, atomically, with no moment absent
		}
		// An earlier conversation in this process, which /clear ended with
		// no hook to move its role: marked, not deleted, as a move by /clear
		// would leave it, since it can be resumed by its id. So no binding
		// is ever deleted, and a session with none was never written: a
		// move to it was cut short (cut; the reviewer, #410).
		m := binding{V: 1, Session: b.Session, Since: b.Since, Was: b.Was, Gone: true,
			Lost: &lostMark{Role: b.Role, To: s.ID, At: now.Format(time.RFC3339Nano), Gen: b.Gen, Went: byClear}}
		if err := a.writeBinding(m); err != nil {
			return false, err
		}
	}
	// A mark left in this process by an earlier conversation keeps its
	// mark, for a resume by its id, but no longer its process: so a later
	// /clear here moves the one mark of the session now in it
	// (reviewer-3, #371).
	for _, m := range a.marks() {
		if (s.PID > 0 && m.PID == s.PID && m.Session != s.ID) || (!m.Gone && !m.running()) {
			// A mark keeps its mark, for a resume by its id, but not a pid
			// that another conversation, or another process, now has
			// (reviewer-3, #371; the reviewer, #372).
			m.PID, m.Host, m.Gone = 0, 0, true
			if err := a.writeBinding(m); err != nil {
				return false, err
			}
		}
	}
	for _, b := range lose {
		m := binding{V: 1, Session: b.Session, PID: b.PID, Host: b.Host, Since: b.Since, Gone: b.Gone,
			Lost: &lostMark{Role: role, To: s.ID, At: now.Format(time.RFC3339Nano), Gen: b.Gen}}
		if !m.Gone && !m.running() {
			// Its process has ended: gone, with no pid, as above
			// (reviewer-3, #378).
			m.PID, m.Host, m.Gone = 0, 0, true
		}
		if err := a.writeBinding(m); err != nil {
			return false, err
		}
	}
	return took, a.writeBinding(binding{V: 1, Role: role, Session: s.ID, PID: s.PID, Host: s.Host, Since: now.Format(time.RFC3339Nano), Gen: now.UnixNano(), CLI: a.cli()})
}

func (a *app) writeBinding(b binding) error {
	data, _ := json.Marshal(b)
	return record.WriteFileAtomic(a.bindingPath(b.Session), append(data, '\n'))
}

// carried is what sessionRole found for a session (reviewer-2, #422).
type carried int

const (
	// carriedNone: no binding, and nothing carried to the session by /clear
	// or /background. Only then may a role fixed by the settings apply.
	carriedNone carried = iota
	// carriedRole: a binding with a role.
	carriedRole
	// carriedBlocked: a mark, or a move to the session known and not
	// settled: pending while the session it went on from still runs,
	// ambiguous, cut short, or failed. No role, and no fixed one either.
	carriedBlocked
)

// sessionRole is the binding a hook's session holds its role by, brought up
// to date: a new process or host noted, as after a resume. At a start: after
// Claude Code's /clear, the role of the one binding in the same process
// moves to the new id. At any hook: after /background, the role of the
// session it went on from moves to it (row 99). A session with no role gets
// none (row 80), and a marked one keeps its mark, with nothing moved onto it
// (row 107).
func (a *app) sessionRole(s session, in hookInput) (binding, carried) {
	if s.ID == "" {
		return binding{}, carriedNone
	}
	if b, ok := a.readBinding(s.ID); ok {
		if b.Role == "" {
			return binding{}, carriedBlocked // marked
		}
		if (b.Gone || b.PID != s.PID || b.Host != s.Host) && !a.update(s, b.Role) {
			return binding{}, carriedBlocked // the role moved meanwhile
		}
		return b, carriedRole
	}
	if b, c := a.cutShort(s); c != carriedNone {
		return b, c
	}
	if in.Event == "SessionStart" && in.Source == "clear" && s.PID > 0 {
		if b, c := a.moveCleared(s); c != carriedNone {
			return b, c
		}
	}
	if in.Transcript != "" {
		return a.moveContinued(s, in.Transcript)
	}
	return binding{}, carriedNone
}

// settled is what a move to s that landed, or was refused, comes to: s's
// binding as it is now, a role or a mark.
func settled(b binding, ok bool) (binding, carried) {
	if ok && b.Role != "" {
		return b, carriedRole
	}
	return binding{}, carriedBlocked
}

// moveContinued moves a role to s when Claude Code says the session that
// holds it went on as s. /background ends a session, and its conversation
// goes on in a new id, in another process, with a SessionStart whose source
// is "fork"; the old transcript ends with a line naming the new id. /fork
// starts the same kind of session but keeps the old one, and writes no such
// line. So the role moves only from a session whose transcript, beside s's,
// names s, at once or after a few such moves, in lines no older than the
// role. The new session usually starts a moment before the old process ends
// (row 100), so while that process runs, the hook waits for it, a few seconds
// at most, without the sessions' lock; a running session keeps its role
// (row 99).
func (a *app) moveContinued(s session, transcript string) (binding, carried) {
	if !plainID.MatchString(s.ID) || filepath.Base(transcript) != s.ID+".jsonl" {
		return binding{}, carriedNone
	}
	dir := filepath.Dir(transcript)
	deadline := time.Now().Add(continuedWait())
	for {
		b, c, running := a.tryMoveContinued(s, dir)
		if !running || !time.Now().Before(deadline) {
			// Still running at the deadline: pending, so blocked, and a
			// later hook moves it (reviewer-2, #422).
			return b, c
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// continuedWait is how long a hook waits for the session it went on from to
// end. The longest seen was about a second (row 100).
func continuedWait() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("HANDOFF_TEST_CONTINUED_WAIT")); err == nil {
		return d // for tests only
	}
	return 5 * time.Second
}

// tryMoveContinued moves the role, under the locks of a role's move, when
// exactly one binding went on as s and its process has ended; running says
// that binding's process still runs. Claude Code starts a session's
// SessionStart hooks at once, so another of them may have moved it already
// (row 100). The new binding keeps the role's time, and notes the session it
// went on from, so that an fyi named for that one still reaches it.
func (a *app) tryMoveContinued(s session, dir string) (moved binding, c carried, running bool) {
	// The locks are taken only for the move itself: not while nothing went
	// on as s, nor while the session it went on from still runs, as this
	// runs every 50 ms for a few seconds then (the reviewer, #397).
	// Another hook of s may have moved it already, while this one waited
	// for the old process to end (reviewer-3, #450).
	if b, ok := a.readBinding(s.ID); ok {
		b, c := settled(b, true)
		return b, c, false
	}
	// Ambiguity is decided only under the locks, after s's own binding is
	// read there.
	switch from := a.wentOnFrom(s, dir); {
	case len(from) == 0:
		// Nothing went on as s, or the move to it is under way: then its
		// "went on" mark is there, and cutShort waits for it.
		if b, ok := a.readBinding(s.ID); ok {
			b, c := settled(b, true)
			return b, c, false
		}
		b, c := a.cutShort(s)
		return b, c, false
	case len(from) == 1 && from[0].running():
		return binding{}, carriedBlocked, true
	}
	unlock, err := a.lockRoles()
	if err != nil {
		return binding{}, carriedBlocked, false
	}
	defer unlock()
	if b, ok := a.readBinding(s.ID); ok {
		b, c := settled(b, true) // another hook of s moved it meanwhile
		return b, c, false
	}
	if a.cut(s) {
		return binding{}, carriedBlocked, false
	}
	from := a.wentOnFrom(s, dir) // again, under the locks
	switch {
	case len(from) == 0:
		return binding{}, carriedNone, false
	case len(from) > 1:
		return binding{}, carriedBlocked, false
	case from[0].running():
		return binding{}, carriedBlocked, true
	}
	moved, c = settled(a.carry(from[0], s, byBackground))
	return moved, c, false
}

// wentOnFrom is the bindings and marks whose session went on as s, by
// /background, the latest of them: one whose transcripts lead to s through
// another of them is left out. An id left behind by /background keeps a
// mark, and its transcript still leads on, through the one it went on as, to
// s; that one may have been given its role again since, with no note of
// where it came from, so the chain, not the binding, says which is latest
// (the reviewer, #410). A session that lost its role keeps its mark through
// /background, and gets no role from it (row 107).
func (a *app) wentOnFrom(s session, dir string) []binding {
	bs, ms, _ := a.sessionFiles() // one pass (reviewer-3, #450)
	bs = append(bs, ms...)
	type candidate struct {
		b       binding
		through []string
	}
	var cs []candidate
	for _, b := range bs {
		if b.Session == s.ID {
			continue
		}
		if through, ok := wentOnThrough(dir, b, s.ID); ok {
			cs = append(cs, candidate{b, through})
		}
	}
	var from []binding
	for _, c := range cs {
		if !slices.ContainsFunc(cs, func(o candidate) bool { return slices.Contains(c.through, o.b.Session) }) {
			from = append(from, c.b)
		}
	}
	return from
}

// carry moves b, a role or a mark, from its session to s, the session its
// conversation went on as, by /background, or by /clear in the same
// process. The old id can be resumed, so it keeps a mark, detached from any
// process, as bind detaches one (reviewer-3, #371): a mark stays as it was,
// and a role leaves a mark saying where it went (reviewer-2, #388). s gets
// the role, or the mark, with the same generation: the conversation is the
// same. The restriction is written first, so a move that fails part way
// leaves no session free that was bound or marked: a mark is carried before
// it is detached, and a role's old id is marked before s is given the role;
// a failure then leaves the role with nobody, for the operator to give
// again, and s is marked at its next hook (cut).
func (a *app) carry(b binding, s session, how string) (binding, bool) {
	old := b
	old.PID, old.Host, old.Gone, old.Block = 0, 0, true, nil
	if b.Lost == nil {
		old.Role = ""
		old.Lost = &lostMark{Role: b.Role, To: s.ID, At: time.Now().UTC().Format(time.RFC3339Nano), Gen: b.Gen, Went: how}
	}
	nu := b
	if how == byBackground {
		nu.Was = append(slices.Clone(b.Was), b.Session)
	}
	// What blocked the old session isn't the new one's, and it hasn't
	// listened yet (row 110).
	nu.Listened, nu.Block, nu.Answered = "", nil, ""
	// The session it went on as is live: not gone (reviewer-3, #378).
	nu.Session, nu.PID, nu.Host, nu.Gone = s.ID, s.PID, s.Host, false
	first, then := nu, old
	if b.Lost == nil {
		first, then = old, nu
	}
	if a.writeBinding(first) != nil {
		return binding{}, false
	}
	if os.Getenv(record.FaultEnv) == "move-between" {
		syscall.Kill(os.Getpid(), syscall.SIGKILL) // for tests: a move cut short
		time.Sleep(time.Minute)
	}
	testPause("move") // for tests: a move held between its writes, as another hook looks
	if a.writeBinding(then) != nil {
		return binding{}, false
	}
	return nu, nu.Role != ""
}

// How a role went on with a session's conversation (lostMark.Went).
const (
	byClear      = "/clear"
	byBackground = "/background"
)

// cutShort is cut for any hook of s, under the locks of a role's move.
// A "went on" mark naming s is there from the moment a move to s begins, so
// a hook that finds one, and then s's binding under the lock, takes what the
// other hook moved: a role, or a mark (reviewer-3, #441).
func (a *app) cutShort(s session) (binding, carried) {
	if !slices.ContainsFunc(a.marks(), func(m binding) bool { return m.Lost.Went != "" && m.Lost.To == s.ID }) {
		return binding{}, carriedNone // nothing moved to s, or cut short: no lock taken
	}
	unlock, err := a.lockRoles()
	if err != nil {
		return binding{}, carriedBlocked // fail closed: no role
	}
	defer unlock()
	if b, ok := a.readBinding(s.ID); ok {
		return settled(b, true) // another hook of s moved it meanwhile
	}
	if a.cut(s) {
		return binding{}, carriedBlocked
	}
	return binding{}, carriedNone
}

// cut marks s when a move of a role to s was cut short: its old id is
// marked as having gone on in s, and s was never given the role. Nothing
// finishes such a move. s is marked too, so it can't act as the role, or as
// any, until the operator gives it one again, and any role the operator has
// given meanwhile stays theirs (reviewer-2, #408). The caller holds the
// locks of a role's move.
func (a *app) cut(s session) (found bool) {
	for _, m := range a.marks() {
		if m.Lost.Went == "" || m.Lost.To != s.ID {
			continue
		}
		b := binding{V: 1, Session: s.ID, PID: s.PID, Host: s.Host, Since: m.Since, Was: m.Was,
			Lost: &lostMark{Role: m.Lost.Role, To: s.ID, At: time.Now().UTC().Format(time.RFC3339Nano), Gen: m.Lost.Gen, Went: m.Lost.Went, Cut: true}}
		a.writeBinding(b)
		return true
	}
	return false
}

// wentOnThrough says whether Claude Code's transcripts in dir show b's
// session going on as session to, at once or after a few moves, each in a
// line no older than b's role: a session resumed after /background keeps its
// id, and the operator may give it the role after that line (row 100).
// through is the sessions between, in order.
func wentOnThrough(dir string, b binding, to string) (through []string, ok bool) {
	since, err := time.Parse(time.RFC3339Nano, b.Since)
	if err != nil {
		return nil, false
	}
	id := b.Session
	for range 8 {
		if !plainID.MatchString(id) {
			return nil, false
		}
		next, at := continuedIn(filepath.Join(dir, id+".jsonl"))
		if next == "" || at.Before(since) {
			return nil, false
		}
		if next == to {
			return through, true
		}
		through = append(through, next)
		id = next
	}
	return nil, false
}

// continuedIn is the session a Claude Code transcript says it went on in, and
// when, from the last "continued-in" line among its last lines, or "". It
// reads only a regular file, and never waits to open one (row 100).
func continuedIn(path string) (string, time.Time) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", time.Time{}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return "", time.Time{}
	}
	const tail = 64 << 10 // the line is followed by a few short ones
	off := max(st.Size()-tail, 0)
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
		return "", time.Time{}
	}
	next, at := "", time.Time{}
	for _, line := range bytes.Split(buf, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"continued-in"`)) {
			continue
		}
		var e struct {
			Type string    `json:"type"`
			Next string    `json:"continuedInSessionId"`
			At   time.Time `json:"timestamp"`
		}
		if json.Unmarshal(line, &e) == nil && e.Type == "continued-in" {
			next, at = e.Next, e.At
		}
	}
	return next, at
}

// was is the sessions s went on from, by /background, as its binding notes.
func (a *app) was(s session) []string {
	b, _ := a.bound(s)
	return b.Was
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
	b.PID, b.Host, b.Gone = s.PID, s.Host, false
	a.writeBinding(b)
	return true
}

// moveCleared moves the role of the one binding in s's Claude Code process
// to s's new id, under the locks of a role's move. Two or more there are
// ambiguous, and blocked.
func (a *app) moveCleared(s session) (binding, carried) {
	if len(a.inProcess(s)) == 0 {
		// Nothing left to move here. Another hook of s may have moved it
		// since sessionRole looked, or be moving it now: its binding, or the
		// "went on" mark it writes first, says so (reviewer-3, #441).
		if b, ok := a.readBinding(s.ID); ok {
			return settled(b, true)
		}
		return a.cutShort(s)
	}
	unlock, err := a.lockRoles()
	if err != nil {
		return binding{}, carriedBlocked
	}
	defer unlock()
	if b, ok := a.readBinding(s.ID); ok {
		return settled(b, true) // another hook of s moved it meanwhile
	}
	if a.cut(s) {
		return binding{}, carriedBlocked
	}
	switch same := a.inProcess(s); len(same) {
	case 0:
		return binding{}, carriedNone
	case 1:
		return settled(a.carry(same[0], s, byClear))
	}
	return binding{}, carriedBlocked
}

// inProcess is the bindings in s's Claude Code process, or with none, the
// marks there of another conversation: a session that lost its role keeps
// its mark through /clear, and gets no role from it (row 107).
func (a *app) inProcess(s session) []binding {
	bs, ms, _ := a.sessionFiles() // one pass (reviewer-3, #450)
	var same []binding
	for _, b := range bs {
		if b.PID == s.PID && b.Session != s.ID {
			same = append(same, b)
		}
	}
	if len(same) == 0 {
		for _, m := range ms {
			if m.PID == s.PID && m.Session != s.ID {
				same = append(same, m)
			}
		}
	}
	return same
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
		return 0, errors.New("handoff role is for a Claude Code, Codex or Antigravity session: the operator tells a session its role, and the session runs it")
	}
	project := filepath.Base(filepath.Dir(a.dir))
	if len(pos) == 0 {
		role, err := a.role("")
		if err != nil {
			return 0, err
		}
		if a.isSpare(role) {
			fmt.Fprintf(a.stdout, "You are the spare in %s.\n", project)
		} else {
			fmt.Fprintf(a.stdout, "You are %s in %s.\n", role, project)
		}
		a.warnOutside(a.stdout, s, "")
		return exitOK, nil
	}
	name := pos[0]
	if name == status.Operator {
		return 0, errors.New("operator is the person who decides; give the session one of the agents' roles")
	}
	if env := os.Getenv("HANDOFF_ROLE"); a.isSpare(env) {
		// The operator names the spare by its session (reviewer-3, #804).
		return 0, fmt.Errorf("HANDOFF_ROLE=spare is refused: the operator names the spare in its session, with `%s role spare` and no fixed role", shellQuote(selfCommand()))
	}
	if _, ok := a.cfg.role(name); !ok && !a.isSpare(name) {
		if err := checkRoleName(name); err != nil {
			return 0, err
		}
		// The operator may have named a role the project doesn't have yet,
		// such as a third reviewer (row 112).
		h := shellQuote(selfCommand())
		return 0, fmt.Errorf("%s isn't a role here (the roles are %s). If the operator gave it to you, run `%s roles add %s`, then `%s role %s`", name, a.roleNames(), h, name, h, name)
	}
	if env := os.Getenv("HANDOFF_ROLE"); env != "" && env != name {
		return 0, fmt.Errorf("this session's settings fix its role to %s (HANDOFF_ROLE); refused", env)
	}
	testPause("role") // for tests: a role held before its binding is written
	was, _ := a.readBinding(s.ID)
	took, err := a.bind(s, name)
	if errors.Is(err, errAlreadySpare) {
		fmt.Fprintf(a.stdout, "You are the spare already in %s.\n\n", project)
		a.warnOutside(a.stdout, s, "")
		return exitOK, a.writeBrief(a.stdout, name, listenMaybe)
	}
	if err != nil {
		return 0, err
	}
	switch {
	case a.isSpare(name):
		fmt.Fprintf(a.stdout, "You are now the spare in %s.", project)
		if took {
			fmt.Fprint(a.stdout, " Another session was the spare; it has no role now.")
		}
	default:
		fmt.Fprintf(a.stdout, "You are now %s in %s.", name, project)
		if took {
			fmt.Fprintf(a.stdout, " Another session had %s; it has no role now.", name)
		}
	}
	fmt.Fprint(a.stdout, "\n\n")
	a.warnOutside(a.stdout, s, "")
	a.tellSpareTook(was, name)
	return exitOK, a.writeBrief(a.stdout, name, listenMaybe)
}

// tellSpareTook tells the operator, once, when the spare their cover names
// took a seat with handoff role while they were away (H35). That is the
// takeover: the move is the one handoff role always makes, which marks the
// old session so it can't act as the seat (row 107). The operator gives it
// a role again, if wanted, as for any session whose role moved.
func (a *app) tellSpareTook(was binding, seat string) {
	if !a.isSpare(was.Role) || was.Lost != nil || a.isSpare(seat) {
		return
	}
	snap, err := a.store.Load()
	if err != nil {
		return
	}
	g := status.Compute(snap.Entries).Away
	if g == nil || g.Cover == nil || g.Cover.Gen != was.Gen {
		return
	}
	a.notifier().Send([]status.Alert{{Kind: status.Quiet, Key: fmt.Sprintf("spare-took-%d", g.ID),
		Text: fmt.Sprintf("Under your go-ahead #%d, the spare took %s, which was stuck. Its old session can't act as %s now. No spare is left.", g.ID, seat, seat)}})
}
