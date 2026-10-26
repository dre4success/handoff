package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dre4success/handoff/internal/status"
)

// The brief (proposal §8, §17 row 67): what an agent on duty needs after a
// fresh start or a summary, so nobody has to paste it again. Its role, how
// it listens, what to do with each kind of entry, the rules, and what waits
// for it now.

func (a *app) cmdBrief(args []string) (int, error) {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	forRole := fs.String("for", "", "")
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err == nil {
		err = noArgs(pos)
	}
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	role, err := a.inboxRole(*forRole, *as)
	if err != nil {
		return 0, err
	}
	if role == status.Operator {
		return 0, fmt.Errorf("the brief is for an agent's role; the operator runs handoff on its own")
	}
	return exitOK, a.writeBrief(a.stdout, role, listenSelf)
}

// How the session the brief is for listens for its work.
type listening int

const (
	listenSelf  listening = iota // it runs wait itself
	listenHook                   // a hook listens for it (row 67)
	listenMaybe                  // not known: a hook does if the project's settings have one
)

// writeBrief prints the brief for role.
func (a *app) writeBrief(w io.Writer, role string, how listening) error {
	snap, err := a.store.Load()
	if err != nil {
		return err
	}
	h := shellQuote(selfCommand())
	project := filepath.Base(filepath.Dir(a.dir))
	var others, reviewers, implementers []string
	allReview := true
	for _, r := range a.cfg.Roles {
		allReview = allReview && r.Reviews
		if r.Name != role {
			others = append(others, r.Name)
			if r.Reviews {
				reviewers = append(reviewers, r.Name)
			} else {
				implementers = append(implementers, r.Name)
			}
		}
	}
	rc, _ := a.cfg.role(role)
	// A role that doesn't review implements. When every role reviews, as
	// with roles named after the agents (row 72), each may do either.
	asks := len(reviewers) > 0 && (!rc.Reviews || allReview)

	fmt.Fprintf(w, "handoff brief: you are %s in %s. You and %s hand work to each other through handoff. The operator decides.\n",
		role, project, strings.Join(others, ", "))
	if allReview {
		fmt.Fprintf(w, "%s of you can implement or review; the operator says who does which, per task.\n", map[bool]string{true: "Either", false: "Each"}[len(others) == 1])
	}
	fmt.Fprintln(w)
	// With no hooks, a Claude Code session waits in the background. Codex
	// hands a running command back to the model about once a minute, each
	// time a whole model call, so a wait there used up the operator's plan in
	// an hour; it ends its turn instead (row 87).
	cli := sessionCLI()
	wait := fmt.Sprintf("run `%s wait --for %s --timeout 25m` in the background. Exit 3: run it again. Exit 4: another session already listens for %s. After you handle what it delivers, wait again.", h, role, role)
	idle := "end your turn. If work waits for you, the operator hears of it and tells you."
	self := map[string]string{
		"claude": wait,
		"codex":  fmt.Sprintf("don't run `%s wait`: Codex checks on a running command about once a minute, each time with a whole model call, which uses up the operator's plan. Instead, %s", h, idle),
		"":       fmt.Sprintf("in Claude Code, %s In Codex, don't run wait, since each minute of it costs a whole model call: %s", wait, idle),
	}[cli]
	// A session with no hooks can't give itself any: they work only from
	// its next start, so it offers, for every project at once (row 87).
	var unset []string
	for _, c := range []string{"claude", "codex"} {
		if (cli == "" || cli == c) && a.hooked(c) == "" {
			unset = append(unset, fmt.Sprintf("`%s settings %s --global`", h, c))
		}
	}
	offer := ""
	if len(unset) > 0 {
		offer = fmt.Sprintf(" If your session has no handoff hooks, offer to set them up for every project: %s prints them and where they go. With the operator's OK, add them. They work once the operator starts this session again.", strings.Join(unset, " or "))
	}
	switch how {
	case listenHook:
		fmt.Fprintf(w, "Listening: you don't run `%s wait`. When your turn ends, a hook listens for %s and brings you what arrives.\n", h, role)
	case listenMaybe:
		fmt.Fprintf(w, "Listening: if your session has handoff's hooks, a hook listens for %s when your turn ends, and you don't run wait. If not, %s%s\n", role, self, offer)
	default:
		fmt.Fprintf(w, "Listening: %s%s\n", strings.ToUpper(self[:1])+self[1:], offer)
	}
	fmt.Fprintf(w, "\nWhen something arrives, read it with `%s show <id>`, then:\n", h)
	if rc.Reviews {
		fmt.Fprintf(w, "- A request to you: `%s claim --as %s <id>`, review exactly the commit it names in a temporary worktree, then `%s reply --as %s <id> --verdict APPROVE|CHANGES|QUESTION --commit <that sha> --body <file>`.\n", h, role, h, role)
	}
	fmt.Fprintf(w, "- A reply to your request: on CHANGES or QUESTION, fix or answer, then post again (`%s post --as %s ... --supersedes <id>`). On APPROVE, the operator decides; wait for the decision.\n", h, role)
	fmt.Fprintf(w, "- A decision from the operator: act on it. Merge only on approve.\n")
	fmt.Fprintf(w, "- A note to you: answer with `%s note --as %s --to <role> --re <id> --body <file>`.", h, role)
	if asks {
		fmt.Fprint(w, " If it has findings on work you haven't asked about, fix them, then ask for a review.")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	if rc.Reviews {
		// Asked in its session to look at work nobody has posted, a reviewer
		// can give no verdict, since only a request names a commit. A note
		// puts what it found in the record (row 85).
		to := "<role>"
		if len(implementers) == 1 {
			to = implementers[0]
		}
		fmt.Fprintf(w, "If the operator asks you to review work with no request, such as uncommitted changes, send what you find to %s as a note: `%s note --as %s --to %s --item <item> --title \"<title>\" --body <file>`. Only a request names a commit, so only a request gets a verdict.\n", to, h, role, to)
	}
	if asks {
		// Of several reviewers, name those with a session: the operator may
		// never give one to each of init's roles (row 89).
		var held, idle []string
		for _, r := range reviewers {
			if a.hasSession(r) {
				held = append(held, r)
			} else {
				idle = append(idle, r)
			}
		}
		to, where := reviewers[0], ""
		switch {
		case len(reviewers) == 1:
		case len(held) == 0:
			to, where = "<reviewer>", ", where <reviewer> is "+listOf(reviewers, "or")+", whichever the operator gives a session; none has one yet"
		default:
			if to = held[0]; len(held) > 1 {
				to, where = "<reviewer>", ", where <reviewer> is "+listOf(held, "or")
			}
			if len(idle) == 1 {
				where += ". " + idle[0] + " has no session now: send to it only once the operator gives it one"
			} else if len(idle) > 1 {
				where += ". " + listOf(idle, "and") + " have no session now: send to one only once the operator gives it one"
			}
		}
		// Agents commit only when asked. The operator asked, by giving this
		// role, so the brief says so (row 85).
		fmt.Fprintf(w, "To ask for a review, commit your work on a branch of its own, not on %s; you don't need to ask the operator first. Then `%s post --as %s --item <item> --to %s --commit <sha> --title \"<title>\" --body <file>`%s.\n", a.cfg.Main, h, role, to, where)
	}
	fmt.Fprintf(w, "To ask the operator about an item: `%s note --as %s --to operator --item <item> --title \"<question>\" --body <file>`.\n", h, role)
	fmt.Fprintf(w, "Rules: never edit .handoff/ by hand. Never merge, push or deploy without the operator's decision. A claim is not an approval.\n")
	fmt.Fprintf(w, "\nWaiting for you now (inbox for %s):\n", role)
	st := status.Compute(snap.Entries)
	a.printInbox(w, st, st.Inbox(role))
	return nil
}

// writeNoRole tells a session in a project with a record that it has no
// role yet, and how it gets one (row 73).
func (a *app) writeNoRole(w io.Writer) {
	fmt.Fprintf(w, "handoff: %s hands work between agents through handoff, and this session has no role in it. When the operator gives you one, run `%s role <role>` (the roles are %s); it prints your brief. Until then, leave handoff alone.\n",
		filepath.Base(filepath.Dir(a.dir)), shellQuote(selfCommand()), a.roleNames())
}

// sessionCLI is the CLI of the session a command runs in, as settings names
// it, or "" outside an agent session.
func sessionCLI() string {
	switch {
	case os.Getenv("CLAUDE_CODE_SESSION_ID") != "":
		return "claude"
	case os.Getenv("CODEX_THREAD_ID") != "":
		return "codex"
	}
	return ""
}

// selfCommand is how an agent should run this handoff: by name when that
// name finds this binary on PATH, else by its full path. Permission rules
// match the command as typed, so the brief and the printed settings agree;
// both quote it with shellQuote, so a path with spaces still runs.
func selfCommand() string {
	self, err := os.Executable()
	if err != nil {
		return "handoff"
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	if found, err := exec.LookPath("handoff"); err == nil {
		if real, err := filepath.EvalSymlinks(found); err == nil && real == self {
			return "handoff"
		}
	}
	return self
}

// shellQuote quotes s for a POSIX shell, unless it needs no quoting.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_/.:@%+=,-") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
