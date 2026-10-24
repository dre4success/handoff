package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/awake"
	"github.com/dre4success/handoff/internal/gitx"
	"github.com/dre4success/handoff/internal/notify"
	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/term"
	"github.com/dre4success/handoff/internal/wait"
)

var (
	roleName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	itemName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

func checkItem(item string) error {
	if item == "" {
		return errors.New("--item is required")
	}
	if !itemName.MatchString(item) {
		return fmt.Errorf("item %q: use letters, digits, '.', '_' or '-', up to 64", item)
	}
	return nil
}

func noArgs(pos []string) error {
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q", pos[0])
	}
	return nil
}

func (a *app) cmdInit(args []string) (int, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	roles := fs.String("roles", "claude,codex", "")
	reviewers := fs.String("reviewers", "", "")
	mainBranch := fs.String("main", "", "")
	pos, err := parse(fs, args)
	if err == nil {
		err = noArgs(pos)
	}
	if err != nil {
		return 0, err
	}
	dir, err := gitx.RecordDir(a.cwd)
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(dir); err == nil {
		return 0, fmt.Errorf("%s exists; refused", dir)
	}
	if *mainBranch == "" {
		if *mainBranch, err = gitx.MainBranch(filepath.Dir(dir)); err != nil {
			return 0, err
		}
	}
	cfg := config{V: record.Version, Main: *mainBranch, QuietAfter: "30m"}
	var revs []string
	if *reviewers != "" {
		revs = strings.Split(*reviewers, ",")
	}
	for _, name := range strings.Split(*roles, ",") {
		name = strings.TrimSpace(name)
		switch {
		case !roleName.MatchString(name):
			return 0, fmt.Errorf("role %q: use lower-case letters, digits and '-'", name)
		case name == status.Operator:
			return 0, errors.New("operator is built in; do not list it")
		case slices.ContainsFunc(cfg.Roles, func(r roleCfg) bool { return r.Name == name }):
			return 0, fmt.Errorf("role %q is listed twice", name)
		}
		// With no --reviewers, every role reviews but one named implementer,
		// so roles named after the agents can each implement or review.
		reviews := slices.Contains(revs, name) || (revs == nil && name != "implementer")
		cfg.Roles = append(cfg.Roles, roleCfg{Name: name, Reviews: reviews})
	}
	if len(cfg.Roles) < 2 {
		return 0, errors.New("--roles needs at least two roles, for example implementer,reviewer")
	}
	for _, r := range revs {
		if _, ok := cfg.role(r); !ok {
			return 0, fmt.Errorf("--reviewers names %q, which is not in --roles", r)
		}
	}
	if err := record.Create(dir); err != nil {
		return 0, err
	}
	// The record is never committed (§5). A .gitignore inside it keeps it
	// out of git without touching the project's files (§9, row 72).
	if err := record.WriteFileAtomic(filepath.Join(dir, ".gitignore"), []byte("# handoff's record is never committed.\n*\n")); err != nil {
		return 0, err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := record.WriteFileAtomic(filepath.Join(dir, "config.json"), append(b, '\n')); err != nil {
		return 0, err
	}
	var names, revNames []string
	for _, r := range cfg.Roles {
		names = append(names, r.Name)
		if r.Reviews {
			revNames = append(revNames, r.Name)
		}
	}
	fmt.Fprintf(a.stdout, "Created %s. Git ignores it; nothing else in the project changed.\n", dir)
	if len(revNames) == len(names) {
		fmt.Fprintf(a.stdout, "Roles: %s. %s can implement or review.\n", listOf(names, "and"), map[bool]string{true: "Either", false: "Each"}[len(names) == 2])
	} else {
		fmt.Fprintf(a.stdout, "Roles: %s; %s %s.\n", listOf(names, "and"), listOf(revNames, "and"), map[bool]string{true: "reviews", false: "review"}[len(revNames) == 1])
	}
	fmt.Fprintf(a.stdout, "Main branch: %s.\n\nTo keep each CLI's sessions on duty here, run these and do what they say:\n", cfg.Main)
	h := shellQuote(selfCommand())
	for _, cli := range []string{"claude", "codex"} {
		if _, ok := cfg.role(cli); ok {
			fmt.Fprintf(a.stdout, "  %s settings %s\n", h, cli)
		} else {
			fmt.Fprintf(a.stdout, "  %s settings %s --as <role>\n", h, cli)
		}
	}
	return exitOK, nil
}

// listOf joins names for a person to read: "a", "a and b", "a, b and c".
func listOf(names []string, conj string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " " + conj + " " + names[len(names)-1]
}

func (a *app) cmdPost(args []string) (int, error) {
	fs := flag.NewFlagSet("post", flag.ContinueOnError)
	item := fs.String("item", "", "")
	to := fs.String("to", "", "")
	ref := fs.String("commit", "", "")
	title := fs.String("title", "", "")
	bodyPath := fs.String("body", "", "")
	baseRef := fs.String("base", "", "")
	on := fs.String("on", "", "")
	parentRefs := fs.String("parents", "", "")
	supersedes := fs.String("supersedes", "", "")
	key := fs.String("key", "", "")
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
	from, err := a.role(*as)
	if err != nil {
		return 0, err
	}
	if from == status.Operator {
		return 0, errors.New("the operator does not post requests; use `handoff note` or `handoff decide`")
	}
	if err := checkItem(*item); err != nil {
		return 0, err
	}
	if strings.TrimSpace(*title) == "" || strings.ContainsAny(*title, "\r\n") {
		return 0, errors.New("--title is required, on one line")
	}
	if err := a.addressee(*to, from, false); err != nil {
		return 0, err
	}
	e := &record.Entry{Kind: "request", Item: *item, From: from, To: *to, Title: strings.TrimSpace(*title), Key: *key}
	if e.Body, err = a.readBody(*bodyPath); err != nil {
		return 0, err
	}
	if e.Commit, err = gitx.ResolveCommit(a.cwd, *ref); err != nil {
		return 0, err
	}
	if e.Base, err = a.base(*baseRef, e.Commit); err != nil {
		return 0, err
	}
	if *on != "" {
		tip, err := gitx.ResolveCommit(a.cwd, *on)
		if err != nil {
			return 0, err
		}
		if ok, err := gitx.IsAncestor(a.cwd, e.Commit, tip); err != nil {
			return 0, err
		} else if !ok {
			return 0, fmt.Errorf("%.12s is not on %s", e.Commit, *on)
		}
	}
	if *parentRefs != "" {
		if e.Parents, err = a.parents(*parentRefs, e.Commit); err != nil {
			return 0, err
		}
	}
	if *supersedes != "" {
		if e.Supersedes, err = parseID(*supersedes); err != nil {
			return 0, err
		}
	}

	var warnings []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		warnings = snap.Warnings()
		if x, err := checkKey(snap, e); x != nil || err != nil {
			return nil, x, err
		}
		st := status.Compute(snap.Entries)
		if err := checkDuplicate(snap, st, e); err != nil {
			return nil, nil, err
		}
		live, hasLive := st.Live(e.Item, e.From, e.To)
		switch {
		case e.Supersedes == 0 && hasLive:
			return nil, nil, fmt.Errorf("request #%d on %s to %s is still open (%s); to replace it, pass --supersedes %d",
				live.Entry.ID, e.Item, e.To, live.State, live.Entry.ID)
		case e.Supersedes != 0 && (!hasLive || live.Entry.ID != e.Supersedes):
			if old, ok := st.Request(e.Supersedes); ok && old.State == status.Superseded {
				return nil, nil, fmt.Errorf("#%d is already superseded by #%d", e.Supersedes, old.SupersededBy)
			}
			return nil, nil, fmt.Errorf("#%d is not the open request on %s from %s to %s", e.Supersedes, e.Item, e.From, e.To)
		}
		return e, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
}

// base resolves --base, or defaults it to the merge base with the main
// branch, and checks that it is an ancestor of the commit under review.
func (a *app) base(ref, commit string) (string, error) {
	var base string
	var err error
	if ref != "" {
		base, err = gitx.ResolveCommit(a.cwd, ref)
	} else {
		var tip string
		if tip, err = gitx.ResolveCommit(a.cwd, a.cfg.Main); err != nil {
			return "", fmt.Errorf("the main branch %q does not resolve (%v); pass --base", a.cfg.Main, err)
		}
		base, err = gitx.MergeBase(a.cwd, commit, tip)
	}
	if err != nil {
		return "", err
	}
	if ok, err := gitx.IsAncestor(a.cwd, base, commit); err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf("base %.12s is not an ancestor of %.12s", base, commit)
	}
	return base, nil
}

// parents resolves --parents and checks them against the commit's own.
func (a *app) parents(list, commit string) ([]string, error) {
	var want []string
	for _, p := range strings.Split(list, ",") {
		sha, err := gitx.ResolveCommit(a.cwd, strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		want = append(want, sha)
	}
	got, err := gitx.Parents(a.cwd, commit)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(got, want) {
		return nil, fmt.Errorf("the parents of %.12s are %s, not %s", commit, short(got), short(want))
	}
	return want, nil
}

func short(shas []string) string {
	var out []string
	for _, s := range shas {
		out = append(out, s[:min(12, len(s))])
	}
	return "[" + strings.Join(out, " ") + "]"
}

// checkKey returns the existing entry a keyed retry matches. A key reused
// with different content is refused.
func checkKey(snap *record.Snapshot, e *record.Entry) (*record.Entry, error) {
	if e.Key == "" {
		return nil, nil
	}
	for i := range snap.Entries {
		x := &snap.Entries[i]
		if x.Key != e.Key {
			continue
		}
		if sameContent(x, e) {
			return x, nil
		}
		return nil, fmt.Errorf("key %q was used by #%d with different content; nothing written", e.Key, x.ID)
	}
	return nil, nil
}

func sameContent(x, e *record.Entry) bool {
	return x.Kind == e.Kind && x.Item == e.Item && x.From == e.From && x.To == e.To &&
		x.Title == e.Title && x.Commit == e.Commit && x.Base == e.Base && slices.Equal(x.Parents, e.Parents) &&
		x.Re == e.Re && x.Supersedes == e.Supersedes && x.Verdict == e.Verdict && x.Decision == e.Decision &&
		x.Body == e.Body
}

// checkDuplicate refuses an entry identical, in the fields proposal §5
// names, to one already in the same open thread.
func checkDuplicate(snap *record.Snapshot, st *status.Status, e *record.Entry) error {
	if !st.ThreadOpen(e.Item) {
		return nil
	}
	for _, x := range snap.Entries {
		if x.Kind == e.Kind && x.Item == e.Item && x.From == e.From && x.To == e.To &&
			x.Commit == e.Commit && x.Re == e.Re && x.Verdict == e.Verdict && x.Body == e.Body {
			return fmt.Errorf("identical to #%d; nothing written (to retry a post safely, give it a --key)", x.ID)
		}
	}
	return nil
}

func (a *app) cmdClaim(args []string) (int, error) {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	id, err := oneID(pos)
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	role, err := a.role(*as)
	if err != nil {
		return 0, err
	}
	var warnings []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		warnings = snap.Warnings()
		r, ok := status.Compute(snap.Entries).Request(id)
		if !ok {
			return nil, nil, fmt.Errorf("#%d is not a request", id)
		}
		if r.Entry.To != role {
			return nil, nil, fmt.Errorf("#%d is addressed to %s, not %s", id, r.Entry.To, role)
		}
		for i := range snap.Entries {
			if x := &snap.Entries[i]; x.Kind == "claim" && x.Re == id && x.From == role {
				return nil, x, nil
			}
		}
		return &record.Entry{Kind: "claim", Item: r.Entry.Item, From: role, Re: id}, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
}

var verdicts = []string{"APPROVE", "CHANGES", "QUESTION"}

func (a *app) cmdReply(args []string) (int, error) {
	fs := flag.NewFlagSet("reply", flag.ContinueOnError)
	verdict := fs.String("verdict", "", "")
	ref := fs.String("commit", "", "")
	bodyPath := fs.String("body", "", "")
	key := fs.String("key", "", "")
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	id, err := oneID(pos)
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	role, err := a.role(*as)
	if err != nil {
		return 0, err
	}
	if rc, ok := a.cfg.role(role); !ok || !rc.Reviews {
		return 0, fmt.Errorf("%s does not review (reviews is false in config.json), so it cannot give a verdict", role)
	}
	if !slices.Contains(verdicts, *verdict) {
		return 0, fmt.Errorf("--verdict must be APPROVE, CHANGES or QUESTION, not %q", *verdict)
	}
	commit, err := gitx.ResolveCommit(a.cwd, *ref)
	if err != nil {
		return 0, err
	}
	body, err := a.readBody(*bodyPath)
	if err != nil {
		return 0, err
	}

	var warnings []string
	var effect string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		warnings = snap.Warnings()
		st := status.Compute(snap.Entries)
		r, ok := st.Request(id)
		if !ok {
			return nil, nil, fmt.Errorf("#%d is not a request", id)
		}
		e := &record.Entry{Kind: "reply", Item: r.Entry.Item, From: role, To: r.Entry.From, Re: id, Verdict: *verdict, Commit: commit, Key: *key, Body: body}
		if x, err := checkKey(snap, e); x != nil || err != nil {
			return nil, x, err
		}
		if r.Entry.To != role {
			return nil, nil, fmt.Errorf("#%d is addressed to %s, not %s", id, r.Entry.To, role)
		}
		if commit != r.Entry.Commit {
			return nil, nil, fmt.Errorf("--commit %.12s differs from the request's %.12s; a verdict names the exact commit it reviewed", commit, r.Entry.Commit)
		}
		if err := checkDuplicate(snap, st, e); err != nil {
			return nil, nil, err
		}
		switch r.State {
		case status.Superseded:
			effect = fmt.Sprintf("request #%d is superseded by #%d; this reply is recorded and changes nothing", id, r.SupersededBy)
		case status.Decided:
			effect = fmt.Sprintf("request #%d is already decided; this reply is recorded and changes nothing", id)
		}
		return e, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	a.alert()
	if effect != "" && !existed {
		fmt.Fprintln(a.stderr, "handoff: "+effect)
	}
	a.printWarnings(warnings)
	return exitOK, nil
}

func (a *app) cmdNote(args []string) (int, error) {
	fs := flag.NewFlagSet("note", flag.ContinueOnError)
	item := fs.String("item", "", "")
	to := fs.String("to", "", "")
	reArg := fs.String("re", "", "")
	title := fs.String("title", "", "")
	bodyPath := fs.String("body", "", "")
	key := fs.String("key", "", "")
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
	from, err := a.role(*as)
	if err != nil {
		return 0, err
	}
	if err := a.addressee(*to, from, true); err != nil {
		return 0, err
	}
	if *item != "" {
		if err := checkItem(*item); err != nil {
			return 0, err
		}
	}
	if strings.ContainsAny(*title, "\r\n") {
		return 0, errors.New("--title must be one line")
	}
	var re int64
	if *reArg != "" {
		if re, err = parseID(*reArg); err != nil {
			return 0, err
		}
	} else if *item == "" {
		return 0, errors.New("--item or --re is required")
	}
	body, err := a.readBody(*bodyPath)
	if err != nil {
		return 0, err
	}

	var warnings []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		warnings = snap.Warnings()
		e := &record.Entry{Kind: "note", Item: *item, From: from, To: *to, Re: re, Title: strings.TrimSpace(*title), Key: *key, Body: body}
		if re != 0 {
			x, ok := snap.Find(re)
			if !ok {
				return nil, nil, fmt.Errorf("#%d does not exist", re)
			}
			switch {
			case e.Item == "":
				e.Item = x.Item
			case x.Item != "" && x.Item != e.Item:
				return nil, nil, fmt.Errorf("#%d is on %s, not %s", re, x.Item, e.Item)
			}
		}
		if x, err := checkKey(snap, e); x != nil || err != nil {
			return nil, x, err
		}
		if err := checkDuplicate(snap, status.Compute(snap.Entries), e); err != nil {
			return nil, nil, err
		}
		return e, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
}

func (a *app) cmdDecide(args []string) (int, error) {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	approve := fs.Bool("approve", false, "")
	deny := fs.Bool("deny", false, "")
	bodyPath := fs.String("body", "", "")
	note := fs.String("note", "", "")
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	if len(pos) != 1 {
		return 0, errors.New("give one item, such as H8, or an entry id")
	}
	if err := a.open(); err != nil {
		return 0, err
	}
	if err := a.operator(*as); err != nil {
		return 0, err
	}
	if !term.IsTerminal(a.stdin) {
		return 0, errors.New("decide needs a terminal on stdin: the operator types it, an agent's shell cannot")
	}
	if *approve == *deny {
		return 0, errors.New("give exactly one of --approve or --deny")
	}
	if *bodyPath != "" && *note != "" {
		return 0, errors.New("give --note or --body, not both")
	}
	if *bodyPath == "-" {
		return 0, errors.New("--body - is not allowed here: stdin is the terminal")
	}
	decision, body := "approve", strings.TrimSpace(*note)
	if *deny {
		decision = "deny"
	}
	if *bodyPath != "" {
		if body, err = a.readBody(*bodyPath); err != nil {
			return 0, err
		}
	}
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	t, err := target(status.Compute(snap.Entries), pos[0])
	if err != nil {
		return 0, err
	}
	a.describe(t)
	fmt.Fprintf(a.stderr, "%s %s? [y/N] ", verbs[decision], label(t))
	line, _ := bufio.NewReader(a.stdin).ReadString('\n')
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return 0, errors.New("not confirmed; nothing written")
	}
	got, existed, err := a.writeDecision(t.Entry.ID, decision, body)
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	return exitOK, nil
}

var verbs = map[string]string{"approve": "Approve", "deny": "Deny"}

// operator checks that the caller may decide. With no role given, the caller
// is the operator; decide also needs a terminal, which an agent's shell does
// not have. An agent's role is refused (proposal §17 row 64).
func (a *app) operator(as string) error {
	if callerSession().ID != "" {
		// Whatever role it claims (row 73).
		return errors.New("decide is the operator's, in a terminal of the operator's own or the pop-up, never in an agent's session")
	}
	if as == "" && os.Getenv("HANDOFF_ROLE") == "" {
		return nil
	}
	role, err := a.role(as)
	if err != nil {
		return err
	}
	if role != status.Operator {
		return fmt.Errorf("decide is for the operator; the caller's role is %s", role)
	}
	return nil
}

// target finds what to decide: an entry id, or an item with exactly one
// entry awaiting the operator. A name of digits only is read as an id.
func target(st *status.Status, arg string) (*status.Target, error) {
	if id, err := parseID(arg); err == nil {
		return st.Decidable(id)
	}
	if checkItem(arg) != nil {
		return nil, fmt.Errorf("%q is neither an item nor an entry id", arg)
	}
	var found []*status.Target
	var ids []string
	for _, t := range st.Inbox(status.Operator).AwaitingOperator {
		if t.Entry.Item == arg {
			found = append(found, t)
			ids = append(ids, fmt.Sprintf("#%d", t.Entry.ID))
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("nothing on %s awaits the operator", arg)
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("%s has %d entries awaiting the operator (%s); give the id", arg, len(found), strings.Join(ids, ", "))
}

func label(t *status.Target) string {
	if t.Entry.Item == "" {
		return fmt.Sprintf("#%d", t.Entry.ID)
	}
	return fmt.Sprintf("%s (#%d)", t.Entry.Item, t.Entry.ID)
}

// describe prints what the operator is deciding: the request and the verdict that
// approved it, or the note and what it asks.
func (a *app) describe(t *status.Target) {
	w := a.stdout
	fmt.Fprintf(w, "\n%s\n", status.Summary(t.Entry))
	if t.Request != nil {
		if t.Request.Verdict != nil {
			fmt.Fprintf(w, "  %s\n", status.Summary(*t.Request.Verdict))
		}
		return
	}
	lines := strings.Split(strings.TrimRight(t.Entry.Body, "\n"), "\n")
	if len(lines) > maxNoteLines {
		lines = append(lines[:maxNoteLines], fmt.Sprintf("… (handoff show %d for the rest)", t.Entry.ID))
	}
	for _, l := range lines {
		fmt.Fprintf(w, "  | %s\n", l)
	}
}

const maxNoteLines = 20

// writeDecision appends the operator's decision, checking again under the lock that
// the entry still awaits the operator. With no note, the decision is its own
// body.
func (a *app) writeDecision(id int64, decision, body string) (record.Entry, bool, error) {
	if body == "" {
		body = map[string]string{"approve": "Approved.", "deny": "Denied."}[decision]
	}
	return a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		t, err := status.Compute(snap.Entries).Decidable(id)
		if err != nil {
			return nil, nil, err
		}
		return &record.Entry{Kind: "decision", Item: t.Entry.Item, From: status.Operator, To: t.Entry.From, Re: t.Entry.ID, Decision: decision, Body: body}, nil, nil
	})
}

// cmdHome is handoff on its own, in a project. For the operator it lists what waits
// for a decision and what has gone quiet, and at a terminal asks about each
// decision in turn. In an agent's session it is that role's inbox.
func (a *app) cmdHome() (int, error) {
	if role := os.Getenv("HANDOFF_ROLE"); role != "" && role != status.Operator {
		return a.cmdInbox(nil)
	}
	if s := callerSession(); s.ID != "" {
		// An agent's session never answers for the operator, with a role or without
		// one (row 73).
		if _, ok := a.bound(s); ok {
			return a.cmdInbox(nil)
		}
		a.writeNoRole(a.stdout)
		return exitOK, nil
	}
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	st := status.Compute(snap.Entries)
	waiting := st.Inbox(status.Operator).AwaitingOperator
	if len(waiting) == 0 {
		fmt.Fprintln(a.stdout, "Nothing needs your decision.")
	}
	atTerminal := term.IsTerminal(a.stdin)
	in := bufio.NewReader(a.stdin)
	for _, t := range waiting {
		a.describe(t)
		if !atTerminal {
			continue
		}
		decision, ok := ask(in, a.stderr)
		if !ok {
			break // end of input: leave the rest
		}
		if decision == "" {
			continue
		}
		fmt.Fprint(a.stderr, "Note (optional): ")
		note, _ := in.ReadString('\n')
		got, existed, err := a.writeDecision(t.Entry.ID, decision, strings.TrimSpace(note))
		if err != nil {
			fmt.Fprintf(a.stderr, "handoff: %v\n", err)
			continue
		}
		a.receipt(got, existed)
	}
	if len(waiting) > 0 && !atTerminal {
		fmt.Fprintln(a.stdout, "\nRun handoff in a terminal to decide.")
	}
	if gone := quietAlerts(st, a.quiet); len(gone) > 0 {
		fmt.Fprintf(a.stdout, "\ngone quiet:\n")
		for _, g := range gone {
			fmt.Fprintf(a.stdout, "  %s\n", g)
		}
	}
	return exitOK, nil
}

// ask reads approve, deny or skip ("") from the operator. It is false at the end of
// input.
func ask(in *bufio.Reader, prompt io.Writer) (string, bool) {
	for {
		fmt.Fprint(prompt, "Approve, deny or skip? [a/d/s] ")
		line, err := in.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "a", "approve":
			return "approve", true
		case "d", "deny":
			return "deny", true
		case "s", "skip", "":
			if err != nil {
				return "", false
			}
			return "", true
		}
		if err != nil {
			return "", false
		}
	}
}

// inboxRole is --for, or else the caller's role. --for only chooses whose
// inbox to read; it never lets the caller write as that role.
func (a *app) inboxRole(forRole, as string) (string, error) {
	r, _, err := a.inboxRoleOf(forRole, as)
	return r, err
}

// inboxRoleOf is inboxRole, and says whether the role is the one the operator gave
// the caller's session, from the same reading of its binding (row 75).
func (a *app) inboxRoleOf(forRole, as string) (string, bool, error) {
	if forRole == "" {
		r, given, err := a.roleOf(as)
		if err != nil {
			return "", false, fmt.Errorf("%w; or give --for <role>", err)
		}
		return r, given, nil
	}
	if _, ok := a.cfg.role(forRole); !ok && forRole != status.Operator {
		return "", false, fmt.Errorf("unknown role %q; the roles are %s", forRole, a.roleNames())
	}
	b, ok := a.bound(callerSession())
	return forRole, ok && b.Role == forRole, nil
}

func (a *app) cmdInbox(args []string) (int, error) {
	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	forRole := fs.String("for", "", "")
	quiet := fs.Bool("quiet", false, "")
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
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	st := status.Compute(snap.Entries)
	in := st.Inbox(role)
	if *quiet {
		if in.Waiting() {
			return exitWaiting, nil
		}
		return exitOK, nil
	}
	fmt.Fprintf(a.stdout, "inbox for %s\n", role)
	a.printInbox(a.stdout, st, in)
	return exitOK, nil
}

// printInbox prints a role's four lists and what has gone quiet.
func (a *app) printInbox(w io.Writer, st *status.Status, in status.Inbox) {
	list := func(title string, rs []*status.Request, withVerdict bool) {
		fmt.Fprintf(w, "\n%s:\n", title)
		if len(rs) == 0 {
			fmt.Fprintln(w, "  (none)")
		}
		for _, r := range rs {
			fmt.Fprintf(w, "  %s\n", status.Summary(r.Entry))
			if withVerdict && r.Verdict != nil {
				fmt.Fprintf(w, "    %s\n", status.Summary(*r.Verdict))
			}
		}
	}
	list("needs my review", in.NeedsReview, false)
	list("being reviewed by me", in.BeingReviewed, true)
	list("needs my response", in.NeedsResponse, true)
	fmt.Fprintf(w, "\nawaiting the operator:\n")
	if len(in.AwaitingOperator) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, t := range in.AwaitingOperator {
		fmt.Fprintf(w, "  %s\n", status.Summary(t.Entry))
		if t.Request != nil && t.Request.Verdict != nil {
			fmt.Fprintf(w, "    %s\n", status.Summary(*t.Request.Verdict))
		}
	}
	// What the operator was, or will be, notified about, for when the notification
	// was missed.
	gone := quietAlerts(st, a.quiet)
	fmt.Fprintf(w, "\ngone quiet:\n")
	if len(gone) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, g := range gone {
		fmt.Fprintf(w, "  %s\n", g)
	}
}

func (a *app) cmdWait(args []string) (int, error) {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	forRole := fs.String("for", "", "")
	timeout := fs.Duration("timeout", 25*time.Minute, "")
	emit := fs.Bool("emit", false, "")
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
	role, given, err := a.inboxRoleOf(*forRole, *as)
	if err != nil {
		return 0, err
	}
	if *timeout <= 0 {
		return 0, errors.New("--timeout must be positive")
	}
	// A wait by a session that holds its role by the operator's word stops when the
	// role moves, as its hook would, even before it starts (rows 74, 75).
	var holds func() bool
	if given {
		holds = a.holds(callerSession(), role)
	}
	startupDelay()
	lock, err := a.listen(role, "wait", given)
	if errors.Is(err, wait.ErrListening) {
		fmt.Fprintf(a.stderr, "handoff wait: %v; this one exits\n", err)
		return exitListening, nil
	}
	if err != nil {
		return 0, err
	}
	defer lock.Close()
	code, err := wait.Run(a.store, wait.Options{Role: role, Timeout: *timeout, Emit: *emit, Interval: time.Second, Clock: wait.Real, Out: a.stdout, Err: a.stderr,
		Quiet: a.quiet, Notify: a.notifier().Send, Alive: holds, Awake: awake.New().Set})
	if err == nil && code == wait.ExitTimeout && holds != nil && !holds() {
		return 0, fmt.Errorf("this session no longer holds %s: the operator gave it another role, or gave %s to another session", role, role)
	}
	return code, err
}

func (a *app) cmdShow(args []string) (int, error) {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	id, err := oneID(pos)
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	e, verified := snap.Find(id)
	if !verified {
		var ok bool
		if e, ok = snap.FindUnverified(id); !ok {
			return 0, fmt.Errorf("#%d does not exist", id)
		}
	}
	w := a.stdout
	to := e.To
	if to == "" {
		to = "-"
	}
	fmt.Fprintf(w, "#%d %s %s\nfrom: %s → %s\nts: %s\n", e.ID, e.Kind, e.Item, e.From, to, e.TS)
	if !verified {
		fmt.Fprintf(w, "verified: NO; the hash chain fails at line %d, and this entry is withheld from inbox and wait\n", snap.Broken.Line)
	}
	field := func(name, value string) {
		if value != "" {
			fmt.Fprintf(w, "%s: %s\n", name, value)
		}
	}
	idField := func(name string, v int64) {
		if v != 0 {
			field(name, fmt.Sprintf("#%d", v))
		}
	}
	field("title", e.Title)
	field("commit", e.Commit)
	field("base", e.Base)
	field("parents", strings.Join(e.Parents, " "))
	idField("re", e.Re)
	idField("supersedes", e.Supersedes)
	field("verdict", e.Verdict)
	field("decision", e.Decision)
	field("key", e.Key)
	if d := e.Damage; d != nil {
		field("damage", fmt.Sprintf("line %d, %d bytes, sha256 %s, after %s", d.Line, d.Bytes, d.SHA256, d.After))
	}
	if r, ok := status.Compute(snap.Entries).Request(id); ok && verified {
		state := string(r.State)
		if r.State == status.Superseded {
			state = fmt.Sprintf("superseded by #%d", r.SupersededBy)
		}
		field("state", state)
		if r.Verdict != nil {
			field("standing verdict", fmt.Sprintf("%s in #%d", r.Verdict.Verdict, r.Verdict.ID))
		}
	}
	field("prev", e.Prev)
	fmt.Fprintf(w, "---\n%s", e.Body)
	if !strings.HasSuffix(e.Body, "\n") && e.Body != "" {
		fmt.Fprintln(w)
	}
	return exitOK, nil
}

func (a *app) cmdRepair(args []string) (int, error) {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
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
	role, err := a.role(*as)
	if err != nil {
		return 0, err
	}
	e, err := a.store.Repair(role)
	if errors.Is(err, record.ErrNothingToRepair) {
		fmt.Fprintln(a.stdout, "nothing to repair: the chain verifies and head.json agrees with it")
		return exitOK, nil
	}
	if err != nil {
		return 0, err
	}
	a.receipt(e, false)
	return exitOK, nil
}

func (a *app) printWarnings(ws []string) {
	for _, m := range ws {
		fmt.Fprintln(a.stderr, "handoff: "+m)
	}
}

// notifier tells the operator about alerts, naming the project by the main
// worktree's folder, so notifications from several projects can be told apart.
func (a *app) notifier() *notify.Notifier {
	n := notify.New(a.dir, filepath.Base(filepath.Dir(a.dir)), a.stderr)
	if os.Getenv(testPopupEnv) != "" || (runtime.GOOS == "darwin" && os.Getenv(notify.TestEnv) == "") {
		n.Ask = a.startPopup
	}
	return n
}

// alert runs after a write: an APPROVE, or a note to the operator, reaches
// the operator at once, whether or not any wait is running. It also passes on any
// silence that has come due. The write has succeeded by now, so a failed
// read here only delays the news to the next command or wait.
func (a *app) alert() {
	snap, err := a.store.Load()
	if err != nil {
		return
	}
	a.notifier().Send(status.Compute(snap.Entries).Alerts(time.Now(), a.quiet))
}

// quietAlerts is what the operator was, or will be, told has gone quiet.
func quietAlerts(st *status.Status, quiet time.Duration) []string {
	var gone []string
	for _, al := range st.Alerts(time.Now(), quiet) {
		if al.Kind == status.Quiet {
			gone = append(gone, al.Text)
		}
	}
	return gone
}
