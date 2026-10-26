// Command handoff lets two coding agents hand work to each other through an
// append-only record that a person can read. It never calls a model, never
// starts or messages an agent, and never drives the work (proposal §4).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/gitx"
	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
)

const usage = `usage: handoff [command] [flags]

  (none)  in a project: what waits for the operator's decision; at a terminal, decide each
  init    [--roles implementer,reviewer,reviewer-2] [--reviewers <roles>] [--main <branch>]
          (every role reviews but implementer; main, else master)
  roles   add <role>...         add roles to the record, such as a second reviewer
  post    --item W31 --to reviewer --commit <ref> --title T --body file
          [--base <ref>] [--on <branch>] [--parents a,b] [--supersedes <id>] [--key k]
  claim   <id>
  reply   <id> --verdict APPROVE|CHANGES|QUESTION --commit <sha> --body file [--key k]
  note    [--item W31] --to <role|operator> [--re <id>] [--title T] --body file [--key k]
  decide  <item|id> --approve|--deny [--note text | --body file]   (the operator, at a terminal)
  inbox   [--for role] [--quiet]
  wait    [--for role] [--timeout 25m] [--emit]
  show    <id>
  brief   [--for role]          an agent's duty, its loop and what waits for it
  role    [<role>]              in an agent's session: take the role the operator gave it, or show it
  settings claude|codex [--global | --as <role>]   settings that keep a CLI's sessions
          on duty: --global for every project, else this one; without --as,
          each session takes the role the operator gives it
  hook    listen|brief|notify   run by a CLI's hooks, as settings prints
  repair
  version                       the version of handoff installed
  help

The caller's role comes from --as <role>, HANDOFF_ROLE, or the role the operator gave
its session (handoff role). Those given must agree. --body - reads stdin.
decide, and handoff on its own, need no role: with none, outside an agent's
session, the caller is the operator.

Exit codes: 0 done; 1 inbox --quiet: something waits; 2 refused or failed;
3 wait timed out; 4 wait: another wait for the role is already listening.
`

const (
	exitOK      = 0
	exitWaiting = 1
	exitRefused = 2
	// exitListening: another process already waits for the role (row 67).
	exitListening = 4
	maxBody       = 1 << 20
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type app struct {
	name   string
	stdin  *os.File
	stdout io.Writer
	stderr io.Writer
	cwd    string
	dir    string // the record directory
	cfg    *config
	store  *record.Store
	quiet  time.Duration // config.json's quiet_after
}

func run(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stderr, usage)
		return exitOK
	}
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintln(stdout, "handoff", version())
		return exitOK
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "handoff:", err)
		return exitRefused
	}
	if len(args) == 0 {
		// On its own: the operator's view of the project, or the usage outside one.
		a := &app{name: "handoff", stdin: stdin, stdout: stdout, stderr: stderr, cwd: cwd}
		if err := a.open(); err != nil {
			fmt.Fprintf(stderr, "%s\nhandoff: %v\n", usage, err)
			return exitRefused
		}
		code, err := a.cmdHome()
		if err != nil {
			fmt.Fprintf(stderr, "handoff: %v\n", err)
			return exitRefused
		}
		return code
	}
	a := &app{name: args[0], stdin: stdin, stdout: stdout, stderr: stderr, cwd: cwd}
	cmds := map[string]func([]string) (int, error){
		"init": a.cmdInit, "post": a.cmdPost, "claim": a.cmdClaim, "reply": a.cmdReply,
		"note": a.cmdNote, "decide": a.cmdDecide, "inbox": a.cmdInbox, "wait": a.cmdWait,
		"show": a.cmdShow, "repair": a.cmdRepair, popupCmd: a.cmdPopup,
		"brief": a.cmdBrief, "hook": a.cmdHook, "settings": a.cmdSettings, "role": a.cmdRole,
		"roles": a.cmdRoles,
	}
	cmd, ok := cmds[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "handoff: unknown command %q\n\n%s", args[0], usage)
		return exitRefused
	}
	code, err := cmd(args[1:])
	switch {
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(stderr, usage)
		return exitOK
	case err != nil:
		fmt.Fprintf(stderr, "handoff %s: %v\n", args[0], err)
		return exitRefused
	}
	return code
}

// version is the module version handoff was built from (row 81): the tag,
// such as v0.1.0, when installed with go install …@v0.1.0 or @latest. Built
// in a clone, it is the tag at a tag and a pseudo-version made from the
// commit otherwise; built in a linked worktree, (devel).
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

// config is .handoff/config.json.
type config struct {
	V     int       `json:"v"`
	Roles []roleCfg `json:"roles"`
	Main  string    `json:"main"`
	// QuietAfter is how long a move may be owed before the operator hears about it.
	// Records made before it existed have none, which means 30 minutes.
	QuietAfter string `json:"quiet_after,omitempty"`
}

const defaultQuiet = 30 * time.Minute

func (c *config) quiet() (time.Duration, error) {
	if c.QuietAfter == "" {
		return defaultQuiet, nil
	}
	d, err := time.ParseDuration(c.QuietAfter)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("quiet_after %q in config.json is not a positive duration such as 30m", c.QuietAfter)
	}
	return d, nil
}

type roleCfg struct {
	Name    string `json:"name"`
	Reviews bool   `json:"reviews"`
}

func (c *config) role(name string) (roleCfg, bool) {
	for _, r := range c.Roles {
		if r.Name == name {
			return r, true
		}
	}
	return roleCfg{}, false
}

// open finds the record from any worktree and reads the configuration.
func (a *app) open() error {
	dir, err := gitx.RecordDir(a.cwd)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no record at %s; run `handoff init` first", dir)
	}
	if err != nil {
		return err
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(dir, "config.json"), err)
	}
	quiet, err := c.quiet()
	if err != nil {
		return err
	}
	a.dir, a.cfg, a.store, a.quiet = dir, &c, record.New(dir), quiet
	return nil
}

// role resolves the caller's role from --as, HANDOFF_ROLE and the role the operator
// gave its session (row 73). Those given must agree.
func (a *app) role(as string) (string, error) {
	r, _, err := a.roleOf(as)
	return r, err
}

// roleOf is role, and says whether the role is the one the operator gave the caller's
// session, from the same reading of its binding (row 75).
func (a *app) roleOf(as string) (string, bool, error) {
	env := os.Getenv("HANDOFF_ROLE")
	var given string
	if b, ok := a.bound(callerSession()); ok {
		given = b.Role
	}
	r := as
	switch {
	case env != "" && as != "" && env != as:
		return "", false, fmt.Errorf("--as %s and HANDOFF_ROLE=%s differ; refused", as, env)
	case given != "" && as != "" && given != as:
		return "", false, fmt.Errorf("--as %s, but this session's role is %s (handoff role); refused", as, given)
	case given != "" && env != "" && given != env:
		return "", false, fmt.Errorf("HANDOFF_ROLE=%s, but this session's role is %s (handoff role); refused", env, given)
	case r == "" && env != "":
		r = env
	case r == "":
		r = given
	}
	if r == "" && callerSession().ID != "" {
		return "", false, errors.New("no role: when the operator gives this session one, run `handoff role <role>`")
	}
	if r == "" {
		return "", false, errors.New("no role: pass --as <role>, or set HANDOFF_ROLE")
	}
	if r == status.Operator {
		return r, false, nil
	}
	if _, ok := a.cfg.role(r); !ok {
		return "", false, fmt.Errorf("unknown role %q; the roles are %s", r, a.roleNames())
	}
	return r, given != "" && r == given, nil
}

func (a *app) roleNames() string {
	var names []string
	for _, r := range a.cfg.Roles {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

// addressee checks a --to value: a configured role, or the operator if allowed.
func (a *app) addressee(to, from string, operatorOK bool) error {
	switch {
	case to == "":
		return errors.New("--to is required")
	case to == from:
		return fmt.Errorf("--to %s is the caller's own role", to)
	case to == status.Operator && operatorOK:
		return nil
	case to == status.Operator:
		return errors.New("a request cannot go to the operator; use `handoff note --to operator`")
	}
	if _, ok := a.cfg.role(to); !ok {
		return fmt.Errorf("unknown role %q; the roles are %s", to, a.roleNames())
	}
	return nil
}

// parse accepts flags before, between and after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard)
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos, args = append(pos, args[0]), args[1:]
	}
}

func oneID(pos []string) (int64, error) {
	if len(pos) != 1 {
		return 0, errors.New("give exactly one entry id")
	}
	return parseID(pos[0])
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%q is not an entry id", s)
	}
	return id, nil
}

// readBody reads --body: a file, or stdin for "-".
func (a *app) readBody(path string) (string, error) {
	if path == "" {
		return "", errors.New("--body is required")
	}
	var r io.Reader
	if path == "-" {
		r = a.stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	}
	b, err := io.ReadAll(io.LimitReader(r, maxBody+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxBody {
		return "", fmt.Errorf("the body is over %d bytes", maxBody)
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", errors.New("the body is empty")
	}
	return string(b), nil
}

func (a *app) warn(snap *record.Snapshot) {
	for _, m := range snap.Warnings() {
		fmt.Fprintln(a.stderr, "handoff: "+m)
	}
}

// receipt prints what was written, or the existing entry a retry matched.
func (a *app) receipt(e record.Entry, existed bool) {
	verb := "posted"
	if existed {
		verb = "exists"
	}
	fmt.Fprintf(a.stdout, "%s %s\n", verb, status.Summary(e))
}
