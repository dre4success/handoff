package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
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
	case name == status.Spare:
		return errors.New("spare is built in: it is the session that can take over a stuck seat while the operator is away; do not list it")
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
	from, err := a.writer(*as)
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
	roles, err := a.addressees(*to, from)
	if err != nil {
		return 0, err
	}
	olds, err := parseIDs(*supersedes)
	if err != nil {
		return 0, err
	}
	if len(roles) == 1 && len(olds) > 1 {
		return 0, fmt.Errorf("--supersedes takes one id for each role in --to, and --to names one")
	}
	e := &record.Entry{Kind: "request", Item: *item, From: from, To: roles[0], Title: strings.TrimSpace(*title), Key: *key}
	if e.Body, err = a.readBody(*bodyPath); err != nil {
		return 0, err
	}
	if e.Commit, err = gitx.ResolveCommit(a.cwd, *ref); err != nil {
		return 0, err
	}
	if len(olds) > 0 {
		e.Supersedes = olds[0]
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
	if len(roles) > 1 {
		e.Supersedes = 0
		return a.postRound(e, roles, olds, *baseRef != "" || *parentRefs != "")
	}

	var warnings []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
		warnings = snap.Warnings()
		if x, err := checkKey(snap, e); x != nil || err != nil {
			return nil, x, err
		}
		if e.Key != "" {
			for i := range snap.Entries {
				if x := &snap.Entries[i]; roundKey(x) == e.Key {
					return nil, nil, fmt.Errorf("key %q was used by round #%d; nothing written", e.Key, x.Round)
				}
			}
		}
		st := status.Compute(snap.Entries)
		if err := checkDuplicate(snap, st, e); err != nil {
			return nil, nil, err
		}
		if rd := st.PendingRound(e.Item, e.From); rd != nil {
			if rd.Broken() {
				return nil, nil, fmt.Errorf("round #%d on %s is broken: its requests don't match each other, which posting again can't repair; tell the operator; nothing written", rd.ID, e.Item)
			}
			return nil, nil, fmt.Errorf("round #%d on %s, to %s at %.12s, is incomplete: post it again as it was, to complete it; nothing written",
				rd.ID, e.Item, listOf(rd.Panel, "and"), rd.Commit)
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
			if old, ok := st.Request(e.Supersedes); ok && old.State == status.Withdrawn && !hasLive {
				return nil, nil, fmt.Errorf("#%d was withdrawn (#%d), so there is nothing to supersede; post without --supersedes", e.Supersedes, old.WithdrawnBy.ID)
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

// addressees checks --to for a request: one role, or several separated by
// commas, each named once (row 106).
func (a *app) addressees(to, from string) ([]string, error) {
	if to == "" {
		return nil, a.addressee(to, from, false)
	}
	var roles []string
	for _, r := range strings.Split(to, ",") {
		r = strings.TrimSpace(r)
		if err := a.addressee(r, from, false); err != nil {
			return nil, err
		}
		if slices.Contains(roles, r) {
			return nil, fmt.Errorf("--to names %s twice", r)
		}
		roles = append(roles, r)
	}
	return roles, nil
}

// parseIDs reads --supersedes: none, one id, or several separated by commas.
func parseIDs(list string) ([]int64, error) {
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	var ids []int64
	for _, x := range strings.Split(list, ",") {
		id, err := parseID(strings.TrimSpace(x))
		if err != nil {
			return nil, err
		}
		if slices.Contains(ids, id) {
			return nil, fmt.Errorf("--supersedes names #%d twice", id)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// postRound posts one request to each of several reviewers, all in one
// write, as one round (row 106). Each is an entry of kind round, so a
// handoff older than row 106 counts none of them, and names the round, the
// id of its first request, and the panel, the roles asked. A round is
// complete only once every role has its request, so a write cut short
// leaves an incomplete round, which nothing approves under the go-ahead.
// The same post again writes only what is missing, from the request already
// written: its title, body, base and parents. Posted again once complete,
// in whatever state, it writes nothing. A new round replaces the last: olds
// are the requests it supersedes, one for each role that has a live one,
// superseded only when the new round is complete. explicit says --base or
// --parents was given, so a retry must give the same.
func (a *app) postRound(tmpl *record.Entry, roles []string, olds []int64, explicit bool) (int, error) {
	tmpl.Kind = "round"
	// A listener from a handoff older than row 106 counts none of a round's
	// requests, so it can't see a round replace the requests before it, and
	// could announce one of those approved under the go-ahead. So no round
	// is posted while one listens, for any role (reviewer-2, #331).
	// The operator's own wait runs the project's alert pass too (reviewer-2,
	// #355). This sees the listeners of now; the README's upgrade steps
	// cover one started later from an older binary.
	names := []string{status.Operator}
	for _, r := range a.cfg.Roles {
		names = append(names, r.Name)
	}
	if old := wait.Older(a.dir, names); len(old) > 0 {
		return 0, fmt.Errorf("an older handoff listens for %s, which can't see a round; nothing written. An idle listener ends once something is delivered to its role, and the next one is this handoff's: send that role a note, then post again", listOf(old, "and"))
	}
	var warnings []string
	var existed []record.Entry
	errSame := errors.New("the round is complete")
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		if err := a.fenced(); err != nil {
			return nil, err
		}
		warnings, existed = snap.Warnings(), nil
		st := status.Compute(snap.Entries)
		replaces := map[string]int64{}
		for _, id := range olds {
			r, ok := st.Request(id)
			switch {
			case !ok:
				return nil, fmt.Errorf("#%d is not a request", id)
			case r.Entry.Item != tmpl.Item || r.Entry.From != tmpl.From || !slices.Contains(roles, r.Entry.To):
				return nil, fmt.Errorf("#%d is not a request on %s from %s to %s", id, tmpl.Item, tmpl.From, listOf(roles, "or"))
			case replaces[r.Entry.To] != 0:
				return nil, fmt.Errorf("#%d and #%d both go to %s; --supersedes takes one for each role", replaces[r.Entry.To], id, r.Entry.To)
			}
			replaces[r.Entry.To] = id
		}
		// sameAs says this post is the one that wrote the round: the same
		// requester, item, commit, panel and supersessions, as the round's
		// first request records them, the same title, body and key, and,
		// when given, the same base and parents.
		sameAs := func(rd *status.Round) bool {
			if rd.Item != tmpl.Item || rd.From != tmpl.From || rd.Commit != tmpl.Commit || !slices.Equal(rd.Panel, roles) ||
				len(rd.Replaces) != len(replaces) || !maps.Equal(rd.Replaces, replaces) {
				return false
			}
			for role, m := range rd.Members {
				x := m.Entry
				if x.Title != tmpl.Title || x.Body != tmpl.Body || x.Key != memberKey(tmpl.Key, role) ||
					(explicit && (x.Base != tmpl.Base || !slices.Equal(x.Parents, tmpl.Parents))) {
					return false
				}
			}
			return true
		}
		member := func(role string, round int64, from *record.Entry) (*record.Entry, error) {
			m := *tmpl
			if from != nil {
				// Completing a round: what its first request asked, word
				// for word, not what this run computed again.
				m.Title, m.Body, m.Base, m.Parents = from.Title, from.Body, from.Base, from.Parents
			}
			m.To, m.Round, m.Panel, m.Supersedes, m.Key = role, round, roles, replaces[role], memberKey(tmpl.Key, role)
			if len(replaces) > 0 {
				m.Replaces = replaces
			}
			// Until the round is complete, the request it replaces is
			// still the live one. Completing a round, a request it replaces
			// that the operator has decided since, or that was withdrawn
			// since, is no obstacle: the round is completed as it was
			// posted, and the decision or withdrawal stands.
			live, hasLive := st.Live(m.Item, m.From, role)
			if old, ok := st.Request(m.Supersedes); from != nil && m.Supersedes != 0 && ok && (old.State == status.Decided || old.State == status.Withdrawn) && !hasLive {
				return &m, nil
			}
			switch {
			case m.Supersedes == 0 && hasLive:
				return nil, fmt.Errorf("request #%d on %s to %s is still open (%s); to replace it, pass its id in --supersedes", live.Entry.ID, m.Item, role, live.State)
			case m.Supersedes != 0 && (!hasLive || live.Entry.ID != m.Supersedes):
				if old, ok := st.Request(m.Supersedes); ok && old.State == status.Superseded {
					return nil, fmt.Errorf("#%d is already superseded by #%d", m.Supersedes, old.SupersededBy)
				}
				if old, ok := st.Request(m.Supersedes); ok && old.State == status.Withdrawn && !hasLive {
					return nil, fmt.Errorf("#%d was withdrawn (#%d), so there is nothing to supersede; leave it out of --supersedes", m.Supersedes, old.WithdrawnBy.ID)
				}
				return nil, fmt.Errorf("#%d is not the open request on %s from %s to %s", m.Supersedes, m.Item, m.From, role)
			}
			if x, err := checkKey(snap, &m); x != nil || err != nil {
				if err == nil {
					err = fmt.Errorf("key %q was used by #%d; nothing written", m.Key, x.ID)
				}
				return nil, err
			}
			return &m, nil
		}
		// A key is the agent's guard for a retry it can't see the end of, so
		// it works as for one request: the same post again finds its round,
		// below, whatever its state; a key used for anything else refuses.
		// Any request of a round with this key counts, whatever its role,
		// so a key can't open a second round with another panel
		// (reviewer-2, #331); so does an entry keyed as this key, or as one
		// of this round's requests would be. A key that only begins the same
		// is another key (#355).
		if tmpl.Key != "" {
			for i := range snap.Entries {
				x := &snap.Entries[i]
				if roundKey(x) != tmpl.Key && x.Key != tmpl.Key && !slices.ContainsFunc(roles, func(r string) bool { return x.Key == memberKey(tmpl.Key, r) }) {
					continue
				}
				if r, ok := st.Request(x.ID); !ok || r.Round == nil || !sameAs(r.Round) {
					return nil, fmt.Errorf("key %q was used by #%d with different content; nothing written", x.Key, x.ID)
				}
			}
		}
		if rd := st.PendingRound(tmpl.Item, tmpl.From); rd != nil {
			switch {
			case rd.Broken():
				return nil, fmt.Errorf("round #%d on %s is broken: its requests don't match each other, which posting again can't repair; nothing on %s is approved under the go-ahead; tell the operator; nothing written", rd.ID, tmpl.Item, tmpl.Item)
			case !sameAs(rd):
				return nil, fmt.Errorf("round #%d on %s, to %s at %.12s, is incomplete: post it again as it was, with the same title, body and key, to complete it; nothing written",
					rd.ID, tmpl.Item, listOf(rd.Panel, "and"), rd.Commit)
			}
			var first *record.Entry
			for _, role := range rd.Panel {
				if m := rd.Members[role]; m != nil && first == nil {
					first = &m.Entry
				}
			}
			var es []*record.Entry
			for _, role := range rd.Panel {
				if m := rd.Members[role]; m != nil {
					existed = append(existed, m.Entry)
					continue
				}
				m, err := member(role, rd.ID, first)
				if err != nil {
					return nil, err
				}
				es = append(es, m)
			}
			return es, nil
		}
		// The same post again, once complete, whatever has been decided on
		// it since, writes nothing. A post that differs, in its title or
		// body say, is a new round.
		for _, rd := range st.Rounds {
			if rd.Complete && rd.Item == tmpl.Item && rd.From == tmpl.From && sameAs(rd) {
				for _, role := range rd.Panel {
					existed = append(existed, rd.Members[role].Entry)
				}
				return nil, errSame
			}
		}
		round := snap.NextID()
		var es []*record.Entry
		for _, role := range roles {
			m, err := member(role, round, nil)
			if err != nil {
				return nil, err
			}
			es = append(es, m)
		}
		return es, nil
	})
	if err != nil && !errors.Is(err, errSame) {
		return 0, err
	}
	for _, e := range existed {
		a.receipt(e, true)
	}
	for _, e := range got {
		a.receipt(e, false)
	}
	a.alert()
	a.printWarnings(warnings)
	for _, role := range roles {
		a.noSession(role)
	}
	return exitOK, nil
}

// roundKey is the key a round's request was posted with: its own key less
// the exact "/<role>" it was given for its role; "" for anything else.
func roundKey(x *record.Entry) string {
	if x.Kind != "round" || x.Key == "" || !strings.HasSuffix(x.Key, "/"+x.To) {
		return ""
	}
	return strings.TrimSuffix(x.Key, "/"+x.To)
}

// memberKey is a round's key for one of its requests.
func memberKey(key, role string) string {
	if key == "" {
		return ""
	}
	return key + "/" + role
}

// noSession tells an agent that what it sent waits, when no session has the
// role it went to (row 89).
func (a *app) noSession(role string) {
	if role == status.Operator {
		return
	}
	d, ok := a.deliveryTo(role)
	if !ok {
		fmt.Fprintf(a.stderr, "handoff: no session has %s now, so this waits until the operator gives one that role; tell the operator.\n", role)
		return
	}
	// A session that can't take its work now says so (row 110).
	if line := blockedLine(role, d); line != "" {
		fmt.Fprintf(a.stderr, "handoff: %s\n", line)
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
						if x.ID == supersedes && (x.Kind == "request" || x.Kind == "round") && x.Base == "" {
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
		slices.Equal(x.Panel, e.Panel) && maps.Equal(x.Replaces, e.Replaces) && x.Body == e.Body
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
	role, err := a.writer(*as)
	if err != nil {
		return 0, err
	}
	var warnings []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
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
	role, err := a.writer(*as)
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
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
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
		if r.State == status.Withdrawn {
			// Refused, not recorded: a listener older than H38, still
			// running, doesn't know the withdrawal, and would take a late
			// APPROVE for one (reviewer-2, #906).
			return nil, nil, fmt.Errorf("request #%d was withdrawn by %s (#%d), so nothing is owed on it; nothing written", id, r.Entry.From, r.WithdrawnBy.ID)
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
	a.noSession(got.To) // reply gets the sender's check too (row 110)
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
	from, err := a.writer(*as)
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
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
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
	st := a.status(snap)
	t, err := target(st, pos[0])
	if err != nil {
		// Nothing on it awaits the operator: work approved and not merged
		// is held by a deny (row 108). With something on it awaiting, the
		// error says what to give instead (the reviewer, #468).
		ap, herr := holdable(st, pos[0])
		var amb ambiguous
		switch {
		case awaitsOn(st, pos[0]):
			return 0, err
		case errors.As(herr, &amb):
			return 0, herr // several approvals: give the id (reviewer-3, #475)
		case herr != nil:
			return 0, err
		}
		if decision != "deny" {
			return 0, fmt.Errorf("%s @ %.12s is approved already, and not merged; --deny holds it", orDash(ap.Item), ap.Commit)
		}
		a.describeHold(ap)
		fmt.Fprintf(a.stderr, "Hold %s @ %.12s? %s is told not to merge it. [y/N] ", orDash(ap.Item), ap.Commit, ap.From)
		line, _ := bufio.NewReader(a.stdin).ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
			return 0, errors.New("not confirmed; nothing written")
		}
		got, existed, err := a.writeHold(ap.Requests[0].Entry.ID, body)
		if err != nil {
			return 0, err
		}
		a.receipt(got, existed)
		return exitOK, nil
	}
	a.describe(st, t)
	for _, ap := range st.ToMerge() {
		if ap.Item != "" && ap.Item == t.Entry.Item {
			fmt.Fprintf(a.stdout, "  also on %s, approved and not merged: %s; to hold it, handoff decide %d --deny\n", ap.Item, ap.Line(), ap.Requests[0].Entry.ID)
		}
	}
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
	var told, sessions, idle []string
	var warnings []string
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		// Who has each role is read under the record's lock, which a role's
		// move holds for writing, so it is read before the move or after,
		// never between its two writes (the reviewer, #451).
		told, sessions, idle = a.sessions()
		if len(told) == 0 {
			return nil, errors.New("no agent has a session now, so no one was told; nothing written")
		}
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
	// When a session told can't take its fyi now, say when it will (row 112).
	testPause("told") // for tests: the receipt held after the write
	for _, e := range got {
		if e.Kind == "fyi" {
			if l := a.whenFyiGets(e); l != "" {
				fmt.Fprintln(a.stdout, l)
			}
		}
	}
	if len(idle) > 0 {
		fmt.Fprintf(a.stdout, "not told, no session: %s\n", strings.Join(idle, ", "))
	}
	// An fyi goes to every agent with a session, but the spare holds no
	// work and is sent nothing, so the operator is told it wasn't (the
	// reviewer, #810).
	if _, ok := a.spareBinding(); ok {
		fmt.Fprintln(a.stdout, "the spare isn't told: it holds no work")
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
	in := bufio.NewReader(a.stdin)
	yes := func() bool {
		line, _ := in.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		return ans == "y" || ans == "yes"
	}
	now := approvedByAway(snap)
	if len(now) > 0 {
		fmt.Fprintf(a.stderr, "Your go-ahead approves now: %s. Go ahead? [y/N] ", approvedList(now))
		if !yes() {
			return 0, errors.New("not confirmed; nothing written")
		}
	}
	// With a spare, away always asks whether it covers, approved work or not,
	// so the operator's yes is recorded for it alone (H35; reviewer-2, #703).
	spare, hasSpare := a.spareBinding()
	cover := false
	if hasSpare {
		// A spare that can't be woken can't cover: the operator hears so
		// before answering, in handoff's own words (the reviewer, #854).
		if d, ok := a.deliveryTo(status.Spare); ok && d.block != nil {
			fmt.Fprintf(a.stderr, "Note: %s.\n", spareBlocked(d))
		}
		fmt.Fprint(a.stderr, "Cover: the spare can take any seat that gets stuck while you're away. Turn cover on? [y/N] ")
		cover = yes()
		testPause("cover") // for tests: away held after the question, as a spare moves
	}
	var told, sessions, idle []string
	var warnings []string
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		told, sessions, idle = a.sessions() // under the lock, as fyi reads it (the reviewer, #451)
		warnings = snap.Warnings()
		if again := approvedByAway(snap); approvedList(again) != approvedList(now) {
			return nil, errors.New("what your go-ahead approves now changed while you were asked; nothing written; run it again")
		}
		// Only a cover the operator said yes to depends on the spare asked
		// about (reviewer-3, #855).
		if cur, ok := a.spareBinding(); cover && (!ok || cur.Session != spare.Session || cur.Gen != spare.Gen) {
			return nil, errors.New("the spare changed while you were asked; nothing written; run it again")
		}
		id := snap.NextID()
		body := fmt.Sprintf("%s\n\nThis is the operator's go-ahead while away (#%d). %s", msg, id, goAheadRule)
		if len(now) > 0 {
			body += " Approved under it now: " + approvedList(now) + "."
		}
		away := &record.Entry{Kind: "away", From: status.Operator, Body: msg}
		if cover {
			away.Cover = &record.Cover{Session: spare.Session, Gen: spare.Gen}
		}
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
	switch {
	case cover:
		fmt.Fprintln(a.stdout, "Cover: on. The spare can take a seat that gets stuck while you're away; you'll be told if it does.")
	case hasSpare:
		fmt.Fprintln(a.stdout, "Away without cover: if a seat gets stuck, you'll be told, as now.")
	case a.isSpare(status.Spare):
		// Not in a project whose config has a role named spare, where that
		// name gives the ordinary role (the reviewer, #854).
		fmt.Fprintln(a.stdout, "No spare: if a seat gets stuck, you'll be told, as now. To have one, start a session and give it the role spare.")
	}
	a.printTold(told, idle, got)
	if len(told) == 0 {
		fmt.Fprintln(a.stdout, "no agent has a session now; each session's brief shows your go-ahead when it starts")
	}
	// Antigravity lets hooks wake a session 10 times before someone types
	// in it (row 102); while away, that is how far it can go (row 112).
	for i, role := range told {
		if b, ok := a.readBinding(sessions[i]); ok && b.CLI == "antigravity" {
			fmt.Fprintf(a.stdout, "%s is an Antigravity session: it listens with a wait it runs in the background. If it stops running one, handoff can wake it only %d times before you type in it.\n", role, agyContinuations)
		}
	}
	if hasSpare && spare.CLI == "antigravity" {
		// The spare is told nothing, so it isn't in told: its line too (the
		// reviewer, #854).
		fmt.Fprintf(a.stdout, "The spare is an Antigravity session: it listens with a wait it runs in the background. If it stops running one, handoff can wake it only %d times before you type in it.\n", agyContinuations)
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
	var told, sessions, idle []string
	var warnings []string
	var away int64
	var approved []*status.Request
	var waiting int
	got, err := a.store.AppendAll(func(snap *record.Snapshot) ([]*record.Entry, error) {
		told, sessions, idle = a.sessions() // under the lock, as fyi reads it (the reviewer, #451)
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
// approved it, with where the item's other requests stand, or the note and
// what it asks.
func (a *app) describe(st *status.Status, t *status.Target) {
	w := a.stdout
	fmt.Fprintf(w, "\n%s\n", status.Summary(t.Entry))
	if t.Request != nil {
		if t.Request.Verdict != nil {
			fmt.Fprintf(w, "  %s\n", status.Summary(*t.Request.Verdict))
		}
		if rd := t.Request.Round; rd != nil && rd.Pending() {
			// The approval is of a round a cut write left incomplete (row 106).
			fmt.Fprintf(w, "  but %s\n", rd.MissingText())
		}
		if others := st.OthersOn(t.Request); others != "" {
			fmt.Fprintf(w, "  on %s: %s\n", t.Entry.Item, others)
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
		if m, ok := a.marked(s); ok {
			return 0, lostErr(m)
		}
		a.writeNoRole(a.stdout)
		return exitOK, nil
	}
	snap, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	a.warn(snap)
	st := a.status(snap)
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
		a.describe(st, t)
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
	// A role's session that can't take its work, or hasn't listened for
	// longer than quiet_after (row 110). A healthy session doesn't listen
	// while it works on its turn, so a shorter gap isn't shown (the reviewer,
	// #495).
	var states []string
	names := make([]string, 0, len(a.cfg.Roles)+1)
	for _, r := range a.cfg.Roles {
		names = append(names, r.Name)
	}
	if _, ok := a.spareBinding(); ok {
		// Cover rests on a spare that can be woken, so the operator who
		// looks before leaving sees a blocked or silent one too (the
		// reviewer, #810).
		names = append(names, status.Spare)
	}
	for _, name := range names {
		d, ok := a.deliveryTo(name)
		switch {
		case !ok:
		case d.block != nil && a.isSpare(name):
			states = append(states, "blocked: "+spareBlocked(d))
		case d.block != nil:
			states = append(states, "blocked: "+blockedLine(name, d))
		case !d.listening && time.Since(d.last) > a.quiet:
			states = append(states, fmt.Sprintf("%s: not listening now (%s)", name, lastListened(d.last)))
		}
	}
	// Cover names the spare by its generation; once no session holds the
	// spare at it, as after a /clear of the spare cut short, no seat is
	// covered, and the operator should know (the reviewer, #822).
	if g := st.Away; g != nil && g.Cover != nil {
		if why := a.coverGone(g.Cover); why != "" {
			states = append(states, fmt.Sprintf("cover: the spare your go-ahead #%d names %s, so no seat is covered. To cover with the spare there is now, run handoff away again", g.ID, why))
		}
	}
	if len(states) > 0 {
		fmt.Fprintf(a.stdout, "\nsessions:\n")
		for _, l := range states {
			fmt.Fprintf(a.stdout, "  %s\n", l)
		}
	}
	// Approved and not merged, and merges recorded on an agent's word
	// (row 108).
	a.printToMerge(a.stdout, "approved, to merge", st.ToMerge(), true)
	if held := st.HeldInMain(); len(held) > 0 {
		fmt.Fprintf(a.stdout, "\nheld by you, but in main:\n")
		for _, ap := range held {
			fmt.Fprintf(a.stdout, "  %s @ %.12s for %s (held in #%d)\n", orDash(ap.Item), ap.Commit, ap.From, ap.Hold.ID)
		}
	}
	if rec := a.recordedMerges(st); len(rec) > 0 {
		fmt.Fprintf(a.stdout, "\nrecorded as merged by their requester (not checked against the approved commit):\n")
		for _, l := range rec {
			fmt.Fprintf(a.stdout, "  %s\n", l)
		}
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
	if a.isSpare(as) || a.isSpare(os.Getenv("HANDOFF_ROLE")) {
		// As roleOfSession refuses them, with --for too (reviewer-2, #817).
		return "", false, fmt.Errorf("the spare is the session the operator names with `%s role spare` in it; --as spare and HANDOFF_ROLE=spare are refused", shellQuote(selfCommand()))
	}
	if _, ok := a.cfg.role(forRole); !ok && forRole != status.Operator && !a.isSpare(forRole) {
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
	st := a.status(snap)
	in := st.Inbox(role)
	if *quiet {
		if in.Waiting() {
			return exitWaiting, nil
		}
		return exitOK, nil
	}
	fmt.Fprintf(a.stdout, "inbox for %s\n", role)
	if a.isSpare(role) {
		fmt.Fprintln(a.stdout, "\nNothing is sent to the spare: it holds no work until it takes a seat.")
		return exitOK, nil
	}
	if d, ok := a.deliveryTo(role); ok {
		if line := blockedLine(role, d); line != "" {
			fmt.Fprintf(a.stdout, "\nblocked: %s\n", line) // row 110
		}
	}
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
	if len(in.Incomplete) > 0 {
		// Only when there is one: a cut write is rare (row 106).
		fmt.Fprintf(w, "\nincomplete, to post again as it was:\n")
		for _, rd := range in.Incomplete {
			if rd.Broken() {
				fmt.Fprintf(w, "  round #%d on %s is broken: its requests don't match each other, which posting again can't repair; tell the operator\n", rd.ID, rd.Item)
				continue
			}
			fmt.Fprintf(w, "  round #%d on %s, to %s @ %.12s: no request to %s\n", rd.ID, rd.Item, listOf(rd.Panel, "and"), rd.Commit, listOf(rd.Missing(), "or"))
		}
	}
	// Only when there is one: a role that never asks for work never has one
	// (row 108).
	a.printToMerge(w, "approved, to merge", in.ToMerge, false)
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
	// wait takes the role's lock and moves its cursor: not a read, so a
	// session that lost its role may not, whatever --for says (reviewer-3,
	// #371).
	// One reading of the session's binding, marked or bound, never a moment
	// between (reviewer-3, #378).
	start, bound := a.readBinding(callerSession().ID)
	if m, lost := a.markOf(start, bound, callerSession().ID); lost {
		return 0, lostErr(m)
	}
	role, _, err := a.inboxRoleOf(*forRole, *as)
	if err != nil {
		return 0, err
	}
	if *timeout <= 0 {
		return 0, errors.New("--timeout must be positive")
	}
	// A wait by a session that holds its role by the operator's word stops when the
	// role moves, as its hook would, even before it starts (rows 74, 75). Any
	// other wait of a session stops when the session is marked (row 107).
	given := bound && start.Role == role
	// A wait takes the role's lock and moves its cursor, so the spare is
	// listened for only by its own session, and listens for nothing else
	// (H35; reviewer-2, #803). Reading an inbox stays open to either.
	switch {
	case a.isSpare(role) && !given:
		return 0, errors.New("only the spare's own session waits for the spare")
	case bound && a.isSpare(start.Role) && !a.isSpare(role):
		return 0, fmt.Errorf("the spare listens only for the spare, not for %s: it holds no work until it takes a seat", role)
	}
	var holds func() bool
	var fence func(func() error) error
	if s := callerSession(); s.ID != "" {
		fence = a.waitFence(s, role, start, given)
		holds = a.holds(s, start, given)
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
	defer wait.Release(lock, a.dir, role)
	if given {
		a.listened(callerSession(), start) // and any block it had is over (row 110)
	}
	code, err := wait.Run(a.store, wait.Options{Role: role, Session: callerSession().ID, Was: a.was(callerSession()), Timeout: *timeout, Emit: *emit, Interval: time.Second, Clock: wait.Real, Out: a.stdout, Err: a.stderr,
		Quiet: a.quiet, Notify: a.notifier().Send, Alive: holds, Awake: awake.New().Set, Fence: fence, Check: a.checkMerge, Wake: a.spareWake(role, start)})
	if m, ok := a.marked(callerSession()); err == nil && code == wait.ExitTimeout && ok {
		return 0, lostErr(m)
	}
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
	head := fmt.Sprintf("#%d %s", e.ID, status.KindName(e.Kind))
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
	if e.Round != 0 {
		field("round", fmt.Sprintf("#%d, to %s", e.Round, listOf(e.Panel, "and")))
	}
	field("verdict", e.Verdict)
	if st := status.Compute(snap.Entries); st != nil {
		if g, ok := st.UnderGoAhead(*e); ok {
			if h := st.HeldBy(*e); h != nil {
				// Held since (reviewer-3, #475).
				field("approved under", fmt.Sprintf("#%d, the operator's go-ahead while away; then held by the operator (#%d): don't merge it", g.ID, h.ID))
			} else {
				field("approved under", fmt.Sprintf("#%d, the operator's go-ahead while away: that is the operator's approve", g.ID))
			}
		}
	}
	field("decision", e.Decision)
	field("key", e.Key)
	field("session", e.Session)
	if d := e.Damage; d != nil {
		field("damage", fmt.Sprintf("line %d, %d bytes, sha256 %s, after %s", d.Line, d.Bytes, d.SHA256, d.After))
	}
	if r, ok := status.Compute(snap.Entries).Request(id); ok && verified {
		state := string(r.State)
		switch r.State {
		case status.Superseded:
			state = fmt.Sprintf("superseded by #%d", r.SupersededBy)
		case status.Withdrawn:
			state = fmt.Sprintf("withdrawn by %s (#%d)", r.Entry.From, r.WithdrawnBy.ID)
		}
		field("state", state)
		// What ended an approval before git saw it in main (row 108; the
		// reviewer, #468).
		if r.Held != nil {
			field("approval", fmt.Sprintf("approved, then held by the operator (#%d)", r.Held.ID))
		}
		if m := r.Merged; m != nil {
			field("approval", fmt.Sprintf("merged as %.12s, recorded by %s (#%d)", m.Commit, m.From, m.ID))
		}
		if rd := r.Round; rd != nil && rd.Broken() {
			field("round state", fmt.Sprintf("broken: its requests don't match each other, so nothing on %s is approved under the go-ahead; posting again can't repair it, and only the operator can sort it out", rd.Item))
		} else if rd != nil && rd.Pending() {
			field("round state", fmt.Sprintf("incomplete: no request to %s, so nothing on %s is approved under the go-ahead; %s posts the same again to complete it", listOf(rd.Missing(), "or"), rd.Item, rd.From))
		}
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
	role, err := a.writer(*as)
	if err != nil {
		return 0, err
	}
	e, err := a.store.Repair(role, a.fenced)
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
	a.notifier().Send(a.status(snap).Alerts(time.Now(), a.quiet))
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
