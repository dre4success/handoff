package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dre4success/handoff/internal/gitx"
	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
	"github.com/dre4success/handoff/internal/wait"
)

// "Approved, to merge" (row 108). Work approved by the operator's decision,
// or under the go-ahead, stays in its requester's inbox until its commit is
// in main, which git answers read-only, or until the requester records it
// merged after a squash or a rebase, or the operator holds it. It is an owed
// move: the quiet alert covers it, and the Mac stays awake for it.

const trackingName = "tracking.json"

type trackingFile struct {
	V     int   `json:"v"`
	Since int64 `json:"since"`
}

// track records, the first time a handoff with row 108 opens the project,
// the record's last id: approvals up to it are history, and never listed. It
// is written whole, then linked into place, so two processes can't both
// write it and no reader sees half of it.
func (a *app) track() {
	path := filepath.Join(a.dir, trackingName)
	if _, err := os.Lstat(path); err == nil {
		return
	}
	snap, err := a.store.Load()
	if err != nil {
		return
	}
	b, _ := json.Marshal(trackingFile{V: record.Version, Since: max(snap.LastID, snap.Head.ID)})
	tmp, err := os.CreateTemp(a.dir, ".tracking-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(append(b, '\n'))
	if err == nil {
		err = tmp.Sync()
	}
	if tmp.Close() == nil && err == nil {
		os.Link(tmp.Name(), path) // fails if another got there first, which is as good
	}
}

// tracked is the id tracking began after, or 0 when it can't be read.
func (a *app) tracked() int64 {
	b, err := os.ReadFile(filepath.Join(a.dir, trackingName))
	if err != nil {
		return 0
	}
	var t trackingFile
	if json.Unmarshal(b, &t) != nil {
		return 0
	}
	return t.Since
}

// mainRef is the local main branch the config names. Nothing is fetched.
func (a *app) mainRef() string { return "refs/heads/" + a.cfg.Main }

// repoRoot is the main worktree, where git is asked whether a commit is in
// main. A listener runs for hours, and the folder it started in may be a
// worktree removed meanwhile; every worktree shares the main one's objects
// and branches (reviewer-3, #475).
func (a *app) repoRoot() string { return filepath.Dir(a.dir) }

// checkMerge tells st which of its approvals git doesn't show in main:
// those it says aren't, and those it can't tell about, with its error
// (row 108). Only git's "no" means not in main.
func (a *app) checkMerge(st *status.Status) {
	var out []*status.Approval
	for _, ap := range st.Approvals(a.tracked()) {
		in, err := gitx.IsAncestor(a.repoRoot(), ap.Commit, a.mainRef())
		if in {
			continue
		}
		ap.Err = err
		out = append(out, ap)
	}
	st.SetToMerge(out)
	var held []*status.Approval
	for _, ap := range st.Holds(a.tracked()) {
		if in, err := gitx.IsAncestor(a.repoRoot(), ap.Commit, a.mainRef()); in && err == nil {
			held = append(held, ap)
		}
	}
	st.SetHeldInMain(held)
	st.SetMore(a.blockAlerts(st))               // row 110
	st.SetDelivery(a.deliveries(), a.tracked()) // row 113
}

// status computes the record's status, with what git says of its approvals.
func (a *app) status(snap *record.Snapshot) *status.Status {
	st := status.Compute(snap.Entries)
	a.checkMerge(st)
	return st
}

// mergeState is how an approval stands, for a line of the inbox: not in
// main, or git's error, and whether its requester has had it delivered.
func (a *app) mergeState(ap *status.Approval) string {
	what := "not in main yet"
	if ap.Err != nil {
		what = "can't tell: " + ap.Err.Error()
	}
	holder, _ := a.holder(ap.From)
	switch cur, err := wait.ReadCursor(a.dir, ap.From); {
	case ap.Told == 0:
		what += "; no session had " + ap.From + " to be told"
	case ap.ToldSession != "" && ap.ToldSession != holder:
		// The fyi named a session that has the role no more; the cursor
		// passing it says nothing of the one that has it now (reviewer-2,
		// #485).
		what += "; told a session that no longer has " + ap.From
	case err == nil && cur < ap.Told:
		what += "; not yet delivered"
	}
	return what
}

// printToMerge lists approvals not yet merged, under a title, if any.
func (a *app) printToMerge(w interface{ Write([]byte) (int, error) }, title string, aps []*status.Approval, who bool) {
	if len(aps) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, ap := range aps {
		by := ""
		if who {
			by = " for " + ap.From
		}
		fmt.Fprintf(w, "  %s%s: %s\n", ap.Line(), by, a.mergeState(ap))
	}
}

// recordedMerges are, for the operator's view, the merges a requester
// recorded in the last week whose approved commit git doesn't see in main:
// handoff took the requester's word for them (row 108).
func (a *app) recordedMerges(st *status.Status) []string {
	var out []string
	seen := map[int64]bool{}
	for _, r := range st.Requests {
		m := r.Merged
		if m == nil || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		if t, err := time.Parse(time.RFC3339, m.TS); err != nil || time.Since(t) > 7*24*time.Hour {
			continue
		}
		if in, err := gitx.IsAncestor(a.repoRoot(), r.Entry.Commit, a.mainRef()); in && err == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%s @ %.12s: merged as %.12s, recorded by %s (#%d)", orDash(r.Entry.Item), r.Entry.Commit, m.Commit, m.From, m.ID))
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// findApproval finds, among aps, the one arg names: an item, or the id of
// one of its requests.
func findApproval(aps []*status.Approval, arg string) (*status.Approval, error) {
	if id, err := parseID(arg); err == nil {
		for _, ap := range aps {
			for _, r := range ap.Requests {
				if r.Entry.ID == id {
					return ap, nil
				}
			}
		}
		return nil, fmt.Errorf("#%d is not in anything approved and not merged", id)
	}
	if checkItem(arg) != nil {
		return nil, fmt.Errorf("%q is neither an item nor an entry id", arg)
	}
	var found []*status.Approval
	var ids []string
	for _, ap := range aps {
		if ap.Item == arg {
			found = append(found, ap)
			ids = append(ids, fmt.Sprintf("#%d", ap.Requests[0].Entry.ID))
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("nothing on %s is approved and not merged", arg)
	case 1:
		return found[0], nil
	}
	return nil, ambiguous{fmt.Sprintf("%s has %d approvals not merged (%s); give the id", arg, len(found), strings.Join(ids, ", "))}
}

// ambiguous is findApproval's error when an item has several approvals.
type ambiguous struct{ msg string }

func (e ambiguous) Error() string { return e.msg }

// cmdMerged records that the caller merged its approved work after a squash
// or a rebase, where the approved commit itself never reaches main (row 108).
// It writes an entry addressed to no one, so it wakes no one. It is refused
// unless the caller asked for the work, the work is approved and not merged,
// and the commit given is in main.
func (a *app) cmdMerged(args []string) (int, error) {
	fs := flag.NewFlagSet("merged", flag.ContinueOnError)
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err == nil && len(pos) != 2 {
		err = errors.New("give the item, or the id of its request, and the commit in main it was merged as")
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
	if role == status.Operator {
		return 0, errors.New("merged is for the agent that asked for the work: it records its own merge")
	}
	sha, err := gitx.ResolveCommit(a.cwd, pos[1])
	if err != nil {
		return 0, err
	}
	switch in, err := gitx.IsAncestor(a.repoRoot(), sha, a.mainRef()); {
	case err != nil:
		return 0, err
	case !in:
		return 0, fmt.Errorf("%.12s is not in %s; merge it first, then record it", sha, a.cfg.Main)
	}
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
		st := status.Compute(snap.Entries)
		a.checkMerge(st)
		ap, err := findApproval(st.ToMerge(), pos[0])
		if err != nil {
			if all, aerr := findApproval(st.Approvals(a.tracked()), pos[0]); aerr == nil && all.From == role {
				return nil, nil, fmt.Errorf("%s @ %.12s is in %s already; nothing to record", orDash(all.Item), all.Commit, a.cfg.Main)
			}
			return nil, nil, err
		}
		if ap.From != role {
			return nil, nil, fmt.Errorf("%s @ %.12s was asked for by %s; only it records the merge", orDash(ap.Item), ap.Commit, ap.From)
		}
		return &record.Entry{Kind: "merged", Item: ap.Item, From: role, Re: ap.Requests[0].Entry.ID, Commit: sha,
			Body: fmt.Sprintf("Merged as %.12s; approved at %.12s.", sha, ap.Commit)}, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	return exitOK, nil
}

// holdable is what decide --deny holds when nothing on arg awaits the
// operator: work approved and not merged (row 108).
func holdable(st *status.Status, arg string) (*status.Approval, error) {
	return findApproval(st.ToMerge(), arg)
}

// writeHold appends the operator's hold on an approval not merged, checking
// again under the lock that it is still approved and not merged. It is
// addressed to the requester, which may have had the approval already, and
// wakes it.
func (a *app) writeHold(first int64, body string) (record.Entry, bool, error) {
	if body == "" {
		body = "Held: approved, but don't merge it."
	}
	return a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		st := status.Compute(snap.Entries)
		a.checkMerge(st)
		ap, err := findApproval(st.ToMerge(), fmt.Sprint(first))
		if err != nil {
			return nil, nil, err
		}
		return &record.Entry{Kind: "decision", Item: ap.Item, From: status.Operator, To: ap.From, Re: first, Decision: "deny", Body: body}, nil, nil
	})
}

// describeHold prints what the operator is holding.
func (a *app) describeHold(ap *status.Approval) {
	fmt.Fprintf(a.stdout, "\napproved, to merge: %s for %s: %s\n", ap.Line(), ap.From, a.mergeState(ap))
}

// awaitsOn says whether anything on item arg awaits the operator.
func awaitsOn(st *status.Status, arg string) bool {
	for _, t := range st.Inbox(status.Operator).AwaitingOperator {
		if t.Entry.Item != "" && t.Entry.Item == arg {
			return true
		}
	}
	return false
}
