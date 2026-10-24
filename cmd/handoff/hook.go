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
	"time"

	"github.com/dre4success/handoff/internal/awake"
	"github.com/dre4success/handoff/internal/notify"
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
	switch args[0] {
	case "listen":
		return a.hookListen(args[1:], in)
	case "brief":
		return a.hookBrief(args[1:], in)
	case "notify":
		return a.hookNotify(args[1:], in)
	}
	return 0, fmt.Errorf("unknown hook %q; want listen, brief or notify", args[0])
}

// hookRole is the role the hook acts for. ok is false when the session is
// not on duty here: no role, no record, or the operator. given says the role
// is the one the operator gave the session, not fixed by its settings.
func (a *app) hookRole(as string, in hookInput) (role string, given, ok bool, err error) {
	if err := a.open(); err != nil {
		return "", false, false, nil
	}
	if os.Getenv("HANDOFF_ROLE") == "" && as == "" {
		role, ok = a.sessionRole(hookSession(in), in)
		return role, ok, ok, nil
	}
	role, given, err = a.roleOf(as)
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
	alive := func() bool { return os.Getppid() == parent }
	role, given, ok, err := a.hookRole(*as, in)
	if err != nil || !ok {
		return exitOK, nil // the brief reports a wrong role, once per session start
	}
	if given {
		// Stop when the session's role moves: the operator gave it another, or gave
		// this one to another session, even before listening starts.
		holds, parentAlive := a.holds(hookSession(in), role), alive
		alive = func() bool { return parentAlive() && holds() }
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
	defer lock.Close()
	var out bytes.Buffer
	code, err := wait.Run(a.store, wait.Options{Role: role, Timeout: *timeout, Interval: time.Second, Clock: wait.Real,
		Out: &out, Err: &out, Quiet: a.quiet, Notify: a.notifier().Send, Alive: alive, Awake: awake.New().Set})
	h := shellQuote(selfCommand())
	switch {
	case err != nil:
		a.troubleOnce(fmt.Sprintf("%s's session stopped listening: %v", role, err))
		return exitOK, nil
	case code == wait.ExitDelivered:
		fmt.Fprintf(a.stderr, "handoff: new for %s:\n%sRead each with `%s show <id>` and act on it as your brief says. Listening starts again when your turn ends.\n",
			role, out.String(), h)
		return exitWake, nil
	case !alive():
		return exitOK, nil
	}
	fmt.Fprintf(a.stderr, "%shandoff: nothing new for %s in %s. Just end your turn; listening starts again when it ends.\n", out.String(), role, *timeout)
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
	if err != nil {
		// What a SessionStart hook prints reaches the model, which can tell the operator.
		fmt.Fprintf(a.stdout, "handoff: this session is not on duty: %v. Tell the operator.\n", err)
		return exitOK, nil
	}
	if !ok {
		if a.dir != "" && *as == "" && hookSession(in).ID != "" {
			a.writeNoRole(a.stdout)
		}
		return exitOK, nil
	}
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
	role, _, ok, _ := a.hookRole(*as, in)
	if !ok || in.Type != "permission_prompt" {
		return exitOK, nil
	}
	a.tellOperator(fmt.Sprintf("%s is waiting for your permission: %s", role, firstLine(in.Message)))
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
func (a *app) tellOperator(text string) {
	project := filepath.Base(filepath.Dir(a.dir))
	if err := notify.Tell(project, text); err != nil {
		fmt.Fprintf(a.stderr, "handoff: could not tell the operator (%v): %s\n", err, text)
	}
}
