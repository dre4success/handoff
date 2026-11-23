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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/awake"
	"github.com/dre4success/handoff/internal/notify"
	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// Hooks keep a Claude Code session on duty (proposal §7, §17 row 67). The
// CLI runs them; `handoff settings claude` prints the lines that set them.
//
//   - listen, on Stop and SessionStart with asyncRewake: waits in the
//     background for the session's role and exits 2 when something arrives,
//     which wakes the session with what arrived. The role's lock means only
//     one listens at a time.
//   - brief, on SessionStart: prints the brief, which reaches the model, so a
//     fresh or summarised session knows its duty without a paste.
//   - notify, on Notification for permission prompts: tells the operator the session
//     is stuck at one.
//
// The role is --as, or HANDOFF_ROLE, when the settings fix one; else the
// role the operator gave the session (row 73), which can move, so a listener stops
// when it does. A session the operator has given no role has none: settings
// made before row 80 pass --prefer, a role to take at a start, and it is
// ignored.
//
// A hook acts only for a session with a role, in a project with a record.
// Otherwise it does nothing, quietly: hooks may be set for sessions that
// never use handoff. A hook never wakes a session to report its own trouble,
// since each wake ends in another Stop and would loop; it tells the operator instead.

// exitWake is what Claude Code's asyncRewake hooks exit with to wake the
// model; what the hook printed on stderr is shown to it.
const exitWake = 2

func (a *app) cmdHook(args []string) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("hook needs listen, brief or notify; the CLI runs it, as `handoff settings claude` prints")
	}
	// The CLI writes its hook input to stdin. Read it all so the CLI never
	// blocks on a full pipe.
	raw, _ := io.ReadAll(io.LimitReader(a.stdin, 1<<20))
	var in hookInput
	json.Unmarshal(raw, &in)
	if in.Session == "" && in.Conversation != "" {
		// Antigravity runs a hook in the folder of the hooks.json that names it,
		// so the project is the one of the folder the agent is in.
		in.Session, in.Event, in.agy = in.Conversation, in.EventName, true
		a.hookCLI = "antigravity"
		if filepath.IsAbs(in.Cwd) {
			a.cwd = in.Cwd
		}
	}
	switch args[0] {
	case "listen":
		if in.agy {
			return a.agyListen(args[1:], in)
		}
		return a.hookListen(args[1:], in)
	case "brief":
		if in.agy {
			return a.agyBrief(args[1:], in)
		}
		return a.hookBrief(args[1:], in)
	case "notify":
		return a.hookNotify(args[1:], in)
	}
	return 0, fmt.Errorf("unknown hook %q; want listen, brief or notify", args[0])
}

// hookRole is the role the hook acts for. ok is false when the session is
// not on duty here: no role, no record, or the operator. given says the role
// is the one the operator gave the session, not fixed by its settings; then
// a.fence is the binding it was resolved from, read once, whose generation
// the hook's listener keeps (reviewer-2, #388).
func (a *app) hookRole(as string, in hookInput) (role string, given, ok bool, err error) {
	if err := a.open(); err != nil {
		return "", false, false, nil
	}
	s := hookSession(in)
	if os.Getenv("HANDOFF_ROLE") == "" && as == "" {
		b, c := a.sessionRole(s, in)
		if c != carriedRole {
			return "", false, false, nil
		}
		a.fence = &b
		return b.Role, true, true, nil
	}
	// A role fixed by the settings applies to a session with no binding only
	// once nothing is carried to it: no role or mark moved to this new id by
	// /clear or /background, and no such move pending, ambiguous or cut
	// short (reviewer-2, #408, #422). With none, the fixed role stands, as
	// before; while one is unsettled, the session has no role at all.
	if _, ok := a.readBinding(s.ID); !ok && s.ID != "" {
		if _, c := a.sessionRole(s, in); c == carriedBlocked {
			return "", false, false, nil
		}
	}
	role, given, err = a.roleOfSession(s, as)
	if err != nil {
		if _, lost := a.settledMark(s); !lost {
			// Read during a move to s by another of its hooks, as one cut
			// short: read again, now that it has landed (the reviewer, #442).
			role, given, err = a.roleOfSession(s, as)
		}
	}
	if err != nil || role == status.Operator {
		return "", false, false, err
	}
	return role, given, true, nil
}

func (a *app) hookListen(args []string, in hookInput) (int, error) {
	fs := flag.NewFlagSet("hook listen", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 23*time.Hour, "")
	as := fs.String("as", "", "")
	fs.String("prefer", "", "") // ignored; see above
	pos, err := parse(fs, args)
	if err == nil {
		err = noArgs(pos)
	}
	if err == nil && *timeout <= 0 {
		err = errors.New("--timeout must be positive")
	}
	if err != nil {
		return 0, err
	}
	// The CLI starts this hook. If the CLI dies without stopping it, the hook
	// is handed to launchd (pid 1) and stops listening.
	parent := os.Getppid()
	if parent == 1 {
		return exitOK, nil
	}
	parentAlive := func() bool { return os.Getppid() == parent }
	began := time.Now() // a block from this hook counts only after it (row 110)
	if in.agy && in.FullyIdle != nil && !*in.FullyIdle {
		// The agent's own task still runs, and wakes it when it ends; that
		// turn's Stop listens. Holding the turn now would keep the task's
		// message from it (evidence/h34/agy-probe/findings.md, row 110).
		return exitOK, nil
	}
	role, given, ok, err := a.hookRole(*as, in)
	if err != nil || !ok {
		// A session that lost its role is told once, by waking it, and then
		// left quiet (row 107).
		// Antigravity ignores a continue past its 10, so the telling waits
		// for a turn it can reach (reviewer-3, #371).
		if in.Continuations >= agyContinuations {
			return exitOK, nil
		}
		if m := a.tellLost(hookSession(in)); m != nil {
			fmt.Fprintf(a.stderr, "handoff: %v.\n", lostErr(m))
			return exitWake, nil
		}
		return exitOK, nil // the brief reports a wrong role, once per session start
	}
	s := hookSession(in)
	var start binding
	if given && a.fence != nil {
		start = *a.fence
	}
	if in.Continuations >= agyContinuations {
		// Antigravity, the only CLI that counts these, would ignore a continue
		// now, so the session can't be woken until the operator types in it.
		// It ends its turn. The block goes in its binding, so senders and the
		// operator see it (row 110). With work waiting, the operator hears of
		// that; with none yet, they are told as before.
		if given {
			switch w, _ := a.writeBlock(s, start, began, blockCap, ""); w {
			case blockStale:
				// What this hook saw is no longer so: say nothing (#494-#496).
				return exitOK, nil
			case blockWritten:
				if a.workWaits(role, s.ID, start.Was) {
					a.alert()
					return exitOK, nil
				}
			}
		}
		a.tellOperator(fmt.Sprintf("handoff can't wake %s's Antigravity session for new work until you type in it: Antigravity lets hooks continue a turn %d times in a row.", role, agyContinuations))
		return exitOK, nil
	}
	// Stop when the session's role moves: the operator gave it another, or
	// gave this one to another session, even before listening starts. A
	// role fixed by --as or HANDOFF_ROLE, with no binding, stops when the
	// session is marked, as it may be once it has taken a role and lost it
	// (reviewer-2, #388). Either way each delivery is fenced, from the
	// binding the role was resolved from.
	alive := parentAlive
	var fence func(func() error) error
	if s.ID != "" {
		holds := a.holds(s, start, given)
		alive = func() bool { return parentAlive() && holds() }
		fence = a.waitFence(s, role, start, given)
	}
	startupDelay()
	lock, err := a.listen(role, "hook", given)
	if errors.Is(err, wait.ErrListening) {
		return exitOK, nil // an earlier turn's listener is still at it
	}
	if err != nil {
		a.troubleOnce(fmt.Sprintf("%s's session could not listen for its work: %v", role, err))
		return exitOK, nil
	}
	defer wait.Release(lock, a.dir, role)
	if given {
		a.listened(s, start) // and any block it had is over (row 110)
	}
	var out bytes.Buffer
	code, err := wait.Run(a.store, wait.Options{Role: role, Session: s.ID, Was: start.Was, Timeout: *timeout, Interval: time.Second, Clock: wait.Real,
		Out: &out, Err: &out, Quiet: a.quiet, Notify: a.notifier().Send, Alive: alive, Awake: awake.New().Set, Fence: fence, Check: a.checkMerge, Wake: a.spareWake(role, start)})
	h := shellQuote(selfCommand())
	switch {
	case err != nil:
		a.troubleOnce(fmt.Sprintf("%s's session stopped listening: %v", role, err))
		// At a break in the chain, wait gave every verified entry before it
		// and moved the cursor past them: the session hears them, or they
		// are lost to it, as a held APPROVE given there would be. Diagnostics
		// alone wake no one (reviewer-2, #549). Any other error may have come
		// before the cursor moved, as when it can't be written, and then the
		// same entry would wake the session at every turn's end, so it only
		// tells the operator, as before (reviewer-2 and the reviewer, #563,
		// #564).
		if errors.Is(err, record.ErrBroken) && gave(out.String()) {
			fmt.Fprintf(a.stderr, "handoff: new for %s:\n%sRead each with `%s show <id>` and act on it as your brief says. The record is broken after these, and the operator has been told; listening starts again when your turn ends.%s\n",
				role, out.String(), h, wakesLeft(in)+agyWaitHint(in, h, role))
			return exitWake, nil
		}
		return exitOK, nil
	case code == wait.ExitDelivered && a.isSpare(role):
		// A wake names no entry: it says what to do (the reviewer, #854).
		// Nor a hint to start a wait, as it is told to take a seat
		// (reviewer-3, #855).
		fmt.Fprintf(a.stderr, "handoff: %sListening starts again when your turn ends.%s\n", out.String(), wakesLeft(in))
		return exitWake, nil
	case code == wait.ExitDelivered:
		fmt.Fprintf(a.stderr, "handoff: new for %s:\n%sRead each with `%s show <id>` and act on it as your brief says. Listening starts again when your turn ends.%s\n",
			role, out.String(), h, wakesLeft(in)+agyWaitHint(in, h, role))
		return exitWake, nil
	case !alive():
		return exitOK, nil
	}
	fmt.Fprintf(a.stderr, "%shandoff: nothing new for %s in %s. Just end your turn; listening starts again when it ends.%s\n", out.String(), role, *timeout, wakesLeft(in)+agyWaitHint(in, h, role))
	return exitWake, nil
}

func (a *app) hookBrief(args []string, in hookInput) (int, error) {
	fs := flag.NewFlagSet("hook brief", flag.ContinueOnError)
	as := fs.String("as", "", "")
	fs.String("prefer", "", "") // ignored; see above
	pos, err := parse(fs, args)
	if err == nil {
		err = noArgs(pos)
	}
	if err != nil {
		return 0, err
	}
	role, _, ok, err := a.hookRole(*as, in)
	if m, lost := a.settledMark(hookSession(in)); err != nil && lost {
		fmt.Fprintf(a.stdout, "handoff: %v.\n", lostErr(m))
		return exitOK, nil
	}
	if err != nil {
		// What a SessionStart hook prints reaches the model, which can tell the operator.
		fmt.Fprintf(a.stdout, "handoff: this session is not on duty: %v. Tell the operator.\n", err)
		return exitOK, nil
	}
	if !ok {
		if m, lost := a.settledMark(hookSession(in)); lost {
			fmt.Fprintf(a.stdout, "handoff: %v.\n", lostErr(m))
			return exitOK, nil
		}
		if a.dir != "" && *as == "" && hookSession(in).ID != "" {
			a.writeNoRole(a.stdout)
		}
		return exitOK, nil
	}
	a.warnOutside(a.stdout, hookSession(in), in.Transcript)
	if err := a.writeBrief(a.stdout, role, listenHook); err != nil {
		fmt.Fprintf(a.stdout, "handoff: no brief: %v. Tell the operator.\n", err)
	}
	return exitOK, nil
}

// hookNotify tells the operator when a session on duty stops at a permission prompt,
// where it would otherwise wait unseen.
func (a *app) hookNotify(args []string, in hookInput) (int, error) {
	fs := flag.NewFlagSet("hook notify", flag.ContinueOnError)
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err == nil {
		err = noArgs(pos)
	}
	if err != nil {
		return 0, err
	}
	began := time.Now()
	role, given, ok, _ := a.hookRole(*as, in)
	if !ok || in.Type != "permission_prompt" {
		return exitOK, nil
	}
	// The prompt is real whatever the block's fence says, so the operator
	// hears of it, as before; a session that no longer holds its role isn't
	// on duty, and hookRole says so first (row 110).
	var al status.Alert
	keyed := false
	if given && a.fence != nil {
		if w, wrote := a.writeBlock(hookSession(in), *a.fence, began, blockPermission, firstLine(in.Message)); w == blockWritten {
			testPause("notified") // for tests: a prompt's hook held after its write
			// With work waiting, the notice is the block's alert, naming
			// the work; with none, it is the prompt alone, and marks nothing.
			al, keyed = a.blockAlertNow(wrote)
		}
	}
	if keyed {
		// The block's alert for the work it names, through the one sender
		// that checks and marks each key under its lock, so an alert pass
		// sending it meanwhile sends it once (reviewer-2, #610); another
		// comes only when more arrives (row 110).
		a.notifier().Send([]status.Alert{al})
	} else {
		a.toldOperator(fmt.Sprintf("%s is waiting for your permission: %s", role, firstLine(in.Message)))
	}
	return exitOK, nil
}

// troubleOnce tells the operator about a failure once, however many turns' hooks
// meet it again.
func (a *app) troubleOnce(text string) {
	sum := sha256.Sum256([]byte(text))
	key := "trouble-" + hex.EncodeToString(sum[:8])
	a.notifier().Send([]status.Alert{{Kind: status.Trouble, Key: key, Text: text}})
}

// tellOperator shows the operator one line now, outside the once-per-alert bookkeeping.
func (a *app) tellOperator(text string) { a.toldOperator(text) }

// toldOperator is tellOperator, and says whether the operator was told.
func (a *app) toldOperator(text string) bool {
	project := filepath.Base(filepath.Dir(a.dir))
	if err := notify.Tell(project, text); err != nil {
		fmt.Fprintf(a.stderr, "handoff: could not tell the operator (%v): %s\n", err, text)
		return false
	}
	return true
}

// Antigravity's hooks (row 102) answer in JSON on stdout. Its Stop hook
// continues the turn with {"decision": "continue", "reason": …}, and the
// reason reaches the model; anything else lets the turn end. A SessionStart
// hook adds a system message with {"injectSteps": …}. Antigravity starts a
// session's hooks at its first message, not when it opens or resumes, and a
// person who presses Esc while a Stop hook holds the turn ends that hook.

// agyContinuations is how many times in a row Antigravity lets Stop hooks
// continue a turn; it ignores the next (evidence/h32/). Its executionNum, on
// a Stop hook's input, counts them, and starts again when the person types.
const agyContinuations = 10

// agyListen is listen for an Antigravity session, with what would wake it as
// the reason to continue.
func (a *app) agyListen(args []string, in hookInput) (int, error) {
	var says bytes.Buffer
	stderr := a.stderr
	a.stderr = &says
	code, err := a.hookListen(args, in)
	a.stderr = stderr
	if err != nil {
		return code, err
	}
	if code != exitWake {
		fmt.Fprintln(a.stdout, "{}")
		return code, nil
	}
	return exitOK, json.NewEncoder(a.stdout).Encode(map[string]string{"decision": "continue", "reason": says.String()})
}

// agyBrief is brief for an Antigravity session: what it would print, as a
// system message.
func (a *app) agyBrief(args []string, in hookInput) (int, error) {
	var says bytes.Buffer
	stdout := a.stdout
	a.stdout = &says
	code, err := a.hookBrief(args, in)
	a.stdout = stdout
	if err != nil || says.Len() == 0 {
		fmt.Fprintln(a.stdout, "{}")
		return code, err
	}
	step := map[string]any{"systemMessage": map[string]string{"systemMessage": says.String()}}
	return code, json.NewEncoder(a.stdout).Encode(map[string]any{"injectSteps": []any{step}})
}

// gave says whether a wait's output delivered an entry: a line of one
// begins with its id, and a diagnostic with "handoff:".
func gave(out string) bool {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "#") {
			return true
		}
	}
	return false
}

// wakesLeft is, for an Antigravity wake from its 7th on, how many more
// handoff has before the operator types in the session: executionNum counts
// from 0, so 0 to 9 are its 10 (row 112). If the operator is there, the
// agent can say so in its session.
func wakesLeft(in hookInput) string {
	if !in.agy || in.Continuations < 6 || in.Continuations >= agyContinuations {
		return ""
	}
	n := agyContinuations - 1 - in.Continuations
	if n == 0 {
		return " This is the last time handoff can wake this session before the operator types in it; if the operator is with you, say so."
	}
	times := "times"
	if n == 1 {
		times = "time"
	}
	return fmt.Sprintf(" handoff can wake this session %d more %s before the operator types in it; if the operator is with you, say so.", n, times)
}

// agyWaitHint is, for an Antigravity wake by its Stop hook, the reminder to
// listen as its brief says: a background wait wakes it with no cap, and the
// hook, the safety net, then stands aside (row 114).
func agyWaitHint(in hookInput, h, role string) string {
	if !in.agy {
		return ""
	}
	return fmt.Sprintf(" Before your turn ends, run `%s wait --for %s --timeout 60m` in the background, as your brief says, so this hook needn't wake you.", h, role)
}
