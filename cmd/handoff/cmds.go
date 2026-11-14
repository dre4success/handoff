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
	// Roles are named by job, not by agent, so no one reads a role as a
	// model (row 80).
	roles := fs.String("roles", "implementer,reviewer,reviewer-2", "")
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
		if err := checkRoleName(name); err != nil {
			return 0, err
		}
		if _, ok := cfg.role(name); ok {
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
	fmt.Fprintf(a.stdout, "Created %s. Git ignores it; nothing else in the project changed.\n", dir)
	writeRoles(a.stdout, &cfg)
	h := shellQuote(selfCommand())
	// The CLI's name comes first, so the rest of the line is a command to
	// copy as it is (row 83). A CLI already set up needs none (row 87), and
	// with both set up, init asks for nothing but roles (row 88).
	clis := []struct{ name, cli, file string }{{"Claude Code:", "claude", ""}, {"Codex:", "codex", ""}}
	missing := false
	for i := range clis {
		clis[i].file = a.hooked(clis[i].cli)
		missing = missing || clis[i].file == ""
	}
	head, give := "Both CLIs have them:", "Give"
	if missing {
		head, give = "Set them once, for every project:", "Then give"
	}
	fmt.Fprintf(a.stdout, "Main branch: %s.\n\nhandoff's hooks keep sessions on duty. %s\n", cfg.Main, head)
	for _, c := range clis {
		if c.file != "" {
			fmt.Fprintf(a.stdout, "  %-13s set up, in %s\n", c.name, tilde(c.file))
		} else {
			fmt.Fprintf(a.stdout, "  %-13s %s settings %s --global\n", c.name, h, c.cli)
		}
	}
	if missing {
		fmt.Fprintln(a.stdout, "For each CLI you use that isn't set up, run its command and do what it prints, or ask one of its sessions to.")
	}
	fmt.Fprintf(a.stdout, "%s each session its role: tell it, such as \"you review\", and it runs %s role <role>.\n", give, h)
	return exitOK, nil
}

// checkRoleName checks the name of a role to be made.
func checkRoleName(name string) error {
	switch {
	case !roleName.MatchString(name):
		return fmt.Errorf("role %q: use lower-case letters, digits and '-'", name)
	case name == status.Operator:
		return errors.New("operator is built in; do not list it")
	}
	return nil
}

// writeRoles says what the roles are and which of them review.
func writeRoles(w io.Writer, cfg *config) {
	var names, revNames []string
	for _, r := range cfg.Roles {
		names = append(names, r.Name)
		if r.Reviews {
			revNames = append(revNames, r.Name)
		}
	}
	if len(revNames) == len(names) {
		fmt.Fprintf(w, "Roles: %s. %s can implement or review.\n", listOf(names, "and"), map[bool]string{true: "Either", false: "Each"}[len(names) == 2])
	} else {
		fmt.Fprintf(w, "Roles: %s; %s %s.\n", listOf(names, "and"), listOf(revNames, "and"), map[bool]string{true: "reviews", false: "review"}[len(revNames) == 1])
	}
}

// cmdRoles adds roles to a record (row 80), such as a second reviewer, or
// roles named by job in a record made with roles named after the agents.
// Entries name roles, so a role is never renamed or removed, and adding one
// changes no entry. A new role reviews unless it is named implementer, as at
// init. Under the sessions' lock, two adds at once never lose one.
func (a *app) cmdRoles(args []string) (int, error) {
	if len(args) == 0 || args[0] != "add" {
		return 0, errors.New("roles needs add: handoff roles add <role>...")
	}
	pos, err := parse(flag.NewFlagSet("roles add", flag.ContinueOnError), args[1:])
	if err == nil && len(pos) == 0 {
		err = errors.New("roles add needs a role, such as reviewer-2")
	}
	if err == nil {
		err = a.open()
	}
	if err != nil {
		return 0, err
	}
	unlock, err := a.lockSessions()
	if err != nil {
		return 0, err
	}
	defer unlock()
	if err := a.open(); err != nil { // again, under the lock
		return 0, err
	}
	cfg := *a.cfg
	cfg.Roles = slices.Clone(cfg.Roles)
	var added []string
	for _, arg := range pos {
		for _, name := range strings.Split(arg, ",") {
			name = strings.TrimSpace(name)
			if err := checkRoleName(name); err != nil {
				return 0, err
			}
			if _, ok := cfg.role(name); ok {
				if slices.Contains(added, name) {
					return 0, fmt.Errorf("role %q is listed twice", name)
				}
				return 0, fmt.Errorf("%s is already a role; the roles are %s", name, a.roleNames())
			}
			cfg.Roles = append(cfg.Roles, roleCfg{Name: name, Reviews: name != "implementer"})
			added = append(added, name)
		}
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := record.WriteFileAtomic(filepath.Join(a.dir, "config.json"), append(b, '\n')); err != nil {
		return 0, err
	}
	fmt.Fprintf(a.stdout, "Added %s.\n", listOf(added, "and"))
	writeRoles(a.stdout, &cfg)
	fmt.Fprintf(a.stdout, "Give a session a role by telling it; it runs %s role <role>.\n", shellQuote(selfCommand()))
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
	if *supersedes != "" {
		if e.Supersedes, err = parseID(*supersedes); err != nil {
			return 0, err
		}
	}
	if e.Base, err = a.base(*baseRef, e.Commit, e.Supersedes); err != nil {
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
	a.noSession(got.To)
	return exitOK, nil
}

// noSession tells an agent that what it sent waits, when no session has the
// role it went to (row 89).
func (a *app) noSession(role string) {
	if role != status.Operator && !a.hasSession(role) {
		fmt.Fprintf(a.stderr, "handoff: no session has %s now, so this waits until the operator gives one that role; tell the operator.\n", role)
	}
}

// base resolves --base, or defaults it to the merge base with the main
// branch, and checks that it is an ancestor of the commit under review.
// supersedes is the request a new round replaces, or 0.
func (a *app) base(ref, commit string, supersedes int64) (string, error) {
	var base string
	var err error
	if ref != "" {
		base, err = gitx.ResolveCommit(a.cwd, ref)
	} else {
		var tip string
		if tip, err = gitx.ResolveCommit(a.cwd, a.cfg.Main); err != nil {
			// In a new project, main has no commit yet. The project's first
			// commit has no parent, so nothing can be its base, and the
			// review covers all of it. Any other commit still needs main or
			// --base, so a misnamed main is still caught (row 85).
			if ps, perr := gitx.Parents(a.cwd, commit); perr == nil && len(ps) == 0 {
				return "", nil
			}
			// A later round of a request with no base has none either: the
			// fix goes on top, and the review still covers all of it (row
			// 86). Entries never change, so reading this one outside the
			// lock is safe; under it, post checks that it is the live one.
			if supersedes != 0 {
				if snap, lerr := a.store.Load(); lerr == nil {
					for _, x := range snap.Entries {
						if x.ID == supersedes && x.Kind == "request" && x.Base == "" {
							return "", nil
						}
					}
				}
			}
			return "", fmt.Errorf("the main branch %q does not resolve (%v): it has no commit yet, or init named the wrong branch. Pass --base. A project's first request, on a commit with no parent, needs none, nor do its later rounds", a.cfg.Main, err)
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
	a.noSession(got.To)
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
	if err := a.operator("decide", *as); err != nil {
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

// operator checks that the caller may run cmd, such as decide, fyi or away, as the operator.
// With no role given, the caller is the operator; both also need a terminal,
// which an agent's shell does not have. An agent's role is refused (proposal
// §17 rows 64, 92, 94, 96).
func (a *app) operator(cmd, as string) error {
	if callerSession().ID != "" {
		// Whatever role it claims (row 73).
		where := "in a terminal of the operator's own"
		if cmd == "decide" {
			where += " or the pop-up"
		}
		return fmt.Errorf("%s is the operator's, %s, never in an agent's session", cmd, where)
	}
	if as == "" && os.Getenv("HANDOFF_ROLE") == "" {
		return nil
	}
	role, err := a.role(as)
	if err != nil {
		return err
	}
	if role != status.Operator {
		return fmt.Errorf("%s is for the operator; the caller's role is %s", cmd, role)
	}
	return nil
}

// cmdFyi is the operator telling every agent with a session the same thing
// at once, such as "carry on while I'm away" (row 92; named fyi in row 94).
// The message is the arguments, so it needs no file, item or role. It
// writes one fyi to each role with a session, all in one write under the
// lock, so no listener sees its fyi before the others are written. A crash in the write keeps the
// whole lines before the cut: those roles were told and the command printed
// nothing. A cut inside a line is a torn tail, and posting stops until
// repair; a cut just after a newline leaves none, and only head.json's
// advisory shows it (row 93). Each fyi names the session the operator gave
// the role to, and only that session's listener brings it, as it brings a
// note; any other passes over it, so a session never gets an fyi written
// before it held the role. A role held by a listener alone, with no session
// given it, gets an unnamed fyi, which that listener brings within a
// second. A role with no session gets none (row 93). An fyi is the
// operator's, so it needs a terminal, as decide does.
// It asks nothing of anyone: no item's state moves, no move is owed, and it
// raises no alert of its own. The agents act on it and don't reply through
// handoff (the brief says so).
func (a *app) cmdFyi(args []string) (int, error) {
	msg, err := message("fyi", args)
	if err != nil {
		return 0, err
	}
	if err := a.atOperatorsTerminal("fyi"); err != nil {
		return 0, err
	}
	told, sessions, idle := a.sessions()
	if len(told) == 0 {
		return 0, errors.New("no agent has a session now, so no one was told; nothing written")
	}
	var warnings []string
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		warnings = snap.Warnings()
		return fyis(told, sessions, msg, 0), nil
	})
	if err != nil {
		return 0, err
	}
	a.printTold(told, idle, got)
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
}

// message is what the operator typed after fyi or away: the arguments, joined.
func message(cmd string, args []string) (string, error) {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		return "", flag.ErrHelp
	}
	example := map[string]string{"fyi": "Carry on while I'm away.", "away": "Merge when the reviewers approve."}[cmd]
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return "", fmt.Errorf(`%s takes no flags, and a message can't start with -: give the message alone, in quotes, such as handoff %s "%s"`, cmd, cmd, example)
	}
	msg := strings.TrimSpace(strings.Join(args, " "))
	if msg == "" {
		return "", fmt.Errorf(`give the message, in quotes, such as handoff %s "%s"`, cmd, example)
	}
	if len(msg) > maxBody {
		return "", fmt.Errorf("the message is over %d bytes", maxBody)
	}
	return msg, nil
}

// atOperatorsTerminal opens the project and checks that the caller is the
// operator, at a terminal, as decide does.
func (a *app) atOperatorsTerminal(cmd string) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.operator(cmd, ""); err != nil {
		return err
	}
	if !term.IsTerminal(a.stdin) {
		return fmt.Errorf("%s needs a terminal on stdin: the operator types it, an agent's shell cannot", cmd)
	}
	return nil
}

// sessions lists the roles with a session now, the session each names (row
// 93), and the roles with none.
func (a *app) sessions() (told, sessions, idle []string) {
	for _, r := range a.cfg.Roles {
		if s, ok := a.holder(r.Name); ok {
			told, sessions = append(told, r.Name), append(sessions, s)
		} else {
			idle = append(idle, r.Name)
		}
	}
	return told, sessions, idle
}

// fyis is one fyi from the operator to each role told, naming its session.
func fyis(told, sessions []string, body string, re int64) []*record.Entry {
	es := make([]*record.Entry, len(told))
	for i, r := range told {
		es[i] = &record.Entry{Kind: "fyi", From: status.Operator, To: r, Session: sessions[i], Re: re, Body: body}
	}
	return es
}

// printTold says which roles the fyis in got reached, and which have no
// session.
func (a *app) printTold(told, idle []string, got []record.Entry) {
	var ids []string
	for _, e := range got {
		if e.Kind == "fyi" {
			ids = append(ids, fmt.Sprintf("#%d", e.ID))
		}
	}
	if len(told) > 0 {
		fmt.Fprintf(a.stdout, "told %s (%s)\n", listOf(told, "and"), strings.Join(ids, ", "))
	}
	if len(idle) > 0 {
		fmt.Fprintf(a.stdout, "not told, no session: %s\n", strings.Join(idle, ", "))
	}
}

// cmdAway is the operator going away with a go-ahead, such as "merge when
// both reviewers approve" (row 96). It records the go-ahead, an away entry,
// and tells it to every agent with a session as an fyi, all in one write.
// Until handoff back, an item is approved under the go-ahead once every
// live request on it is approved at one commit; what is approved already is
// approved under it at once, so the operator confirms that first. An
// approval is still a reviewer's alone: the go-ahead is the operator's
// decision, given in advance, and a request no reviewer approved is never
// approved by it.
func (a *app) cmdAway(args []string) (int, error) {
	msg, err := message("away", args)
	if err != nil {
		return 0, err
	}
	if err := a.atOperatorsTerminal("away"); err != nil {
		return 0, err
	}
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	now := approvedByAway(snap)
	if len(now) > 0 {
		fmt.Fprintf(a.stderr, "Your go-ahead approves now: %s. Go ahead? [y/N] ", approvedList(now))
		line, _ := bufio.NewReader(a.stdin).ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
			return 0, errors.New("not confirmed; nothing written")
		}
	}
	told, sessions, idle := a.sessions()
	var warnings []string
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		warnings = snap.Warnings()
		if again := approvedByAway(snap); approvedList(again) != approvedList(now) {
			return nil, errors.New("what your go-ahead approves now changed while you were asked; nothing written; run it again")
		}
		id := snap.NextID()
		body := fmt.Sprintf("%s\n\nThis is the operator's go-ahead while away (#%d). %s", msg, id, goAheadRule)
		if len(now) > 0 {
			body += " Approved under it now: " + approvedList(now) + "."
		}
		away := &record.Entry{Kind: "away", From: status.Operator, Body: msg}
		return append([]*record.Entry{away}, fyis(told, sessions, body, id)...), nil
	})
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(a.stdout, "away (#%d): your go-ahead stands until you run handoff back\n", got[0].ID)
	// The tool applies one rule, whatever the words say, so the operator
	// reads it here, as the go-ahead is given (row 98).
	fmt.Fprintf(a.stdout, "%s handoff doesn't read your words: to tell the agents something without approving anything, use handoff fyi.\n", goAheadRule)
	if len(now) > 0 {
		fmt.Fprintf(a.stdout, "approved under it now: %s\n", approvedList(now))
	}
	a.printTold(told, idle, got)
	if len(told) == 0 {
		fmt.Fprintln(a.stdout, "no agent has a session now; each session's brief shows your go-ahead when it starts")
	}
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
}

// goAheadRule is what a go-ahead approves, as status applies it (row 96).
const goAheadRule = "Under it, an item is approved once every reviewer it was sent to approves the same commit; one approval is enough for an item sent to one reviewer."

// approvedByAway lists the requests an away entry written now would approve.
func approvedByAway(snap *record.Snapshot) []*status.Request {
	id := snap.NextID()
	es := append(append([]record.Entry(nil), snap.Entries...), record.Entry{Kind: "away", ID: id, From: status.Operator})
	return status.Compute(es).ApprovedSince(id)
}

// approvedList names approved requests by item, such as "H30 (#140, #141)".
func approvedList(rs []*status.Request) string {
	var items []string
	ids := map[string][]string{}
	for _, r := range rs {
		item := r.Entry.Item
		if _, ok := ids[item]; !ok {
			items = append(items, item)
		}
		ids[item] = append(ids[item], fmt.Sprintf("#%d", r.Entry.ID))
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = fmt.Sprintf("%s (%s)", item, strings.Join(ids[item], ", "))
	}
	return strings.Join(parts, ", ")
}

// cmdBack ends the operator's go-ahead (row 96): a back entry, and an fyi
// to every agent with a session that it has ended, in one write. It says
// what was approved under the go-ahead, and what waits for the operator.
func (a *app) cmdBack(args []string) (int, error) {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		return 0, flag.ErrHelp
	}
	if len(args) > 0 {
		return 0, errors.New("back takes nothing: run handoff back")
	}
	if err := a.atOperatorsTerminal("back"); err != nil {
		return 0, err
	}
	told, sessions, idle := a.sessions()
	var warnings []string
	var away int64
	var approved []*status.Request
	var waiting int
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		warnings = snap.Warnings()
		st := status.Compute(snap.Entries)
		if st.Away == nil {
			return nil, errors.New("you aren't away; nothing written")
		}
		away = st.Away.ID
		approved = st.ApprovedSince(st.AwaySince.ID)
		waiting = len(st.Inbox(status.Operator).AwaitingOperator)
		body := fmt.Sprintf("The operator is back: the go-ahead of #%d has ended. On an APPROVE, wait for the operator's decision again.", away)
		back := &record.Entry{Kind: "back", From: status.Operator, Re: away, Body: "Back."}
		return append([]*record.Entry{back}, fyis(told, sessions, body, away)...), nil
	})
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(a.stdout, "back (#%d): the go-ahead of #%d has ended\n", got[0].ID, away)
	if len(approved) > 0 {
		fmt.Fprintf(a.stdout, "approved under it while you were away: %s\n", approvedList(approved))
	} else {
		fmt.Fprintln(a.stdout, "nothing was approved under it")
	}
	if waiting > 0 {
		fmt.Fprintf(a.stdout, "waiting for your decision: %d; run handoff to decide\n", waiting)
	}
	a.printTold(told, idle, got)
	a.alert()
	a.printWarnings(warnings)
	return exitOK, nil
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
	if g := st.Away; g != nil {
		fmt.Fprintf(a.stdout, "You're away (#%d, since %s), with this go-ahead: %s\nRun handoff back to end it.\n", g.ID, g.TS, firstLine(g.Body))
		if len(waiting) == 0 {
			fmt.Fprintln(a.stdout)
		}
	}
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
	code, err := wait.Run(a.store, wait.Options{Role: role, Session: callerSession().ID, Was: a.was(callerSession()), Timeout: *timeout, Emit: *emit, Interval: time.Second, Clock: wait.Real, Out: a.stdout, Err: a.stderr,
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
	// An fyi or a repair has no item, and the line ends at its kind.
	head := fmt.Sprintf("#%d %s", e.ID, e.Kind)
	if e.Item != "" {
		head += " " + e.Item
	}
	fmt.Fprintf(w, "%s\nfrom: %s → %s\nts: %s\n", head, e.From, to, e.TS)
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
	if g, ok := status.Compute(snap.Entries).UnderGoAhead(*e); ok {
		field("approved under", fmt.Sprintf("#%d, the operator's go-ahead while away: that is the operator's approve", g.ID))
	}
	field("decision", e.Decision)
	field("key", e.Key)
	field("session", e.Session)
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
