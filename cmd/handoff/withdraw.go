package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
)

// H38 (row 118): the agent that asked for a review withdraws a request it no
// longer wants answered, such as one still open on an item merged, or on an
// item the operator dropped. Nothing else could end it, so it stayed in
// inboxes, alerted as quiet, and counted as work for the stuck check.

func (a *app) cmdWithdraw(args []string) (int, error) {
	fs := flag.NewFlagSet("withdraw", flag.ContinueOnError)
	note := fs.String("note", "", "")
	as := fs.String("as", "", "")
	pos, err := parse(fs, args)
	if err != nil {
		return 0, err
	}
	id, err := oneID(pos)
	if err == nil && strings.ContainsAny(*note, "\r\n") {
		err = errors.New("--note is one line")
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
	var others []string
	got, existed, err := a.store.Append(func(snap *record.Snapshot) (*record.Entry, *record.Entry, error) {
		if err := a.fenced(); err != nil {
			return nil, nil, err
		}
		st := status.Compute(snap.Entries)
		r, ok := st.Request(id)
		if !ok {
			return nil, nil, fmt.Errorf("#%d is not a request", id)
		}
		if r.Entry.From != role {
			return nil, nil, fmt.Errorf("#%d was asked for by %s; only it can withdraw it", id, r.Entry.From)
		}
		switch {
		case r.State == status.Withdrawn:
			return nil, r.WithdrawnBy, nil // a retry finds its own
		case r.State == status.AwaitingOperator:
			return nil, nil, fmt.Errorf("#%d is approved and waits for the operator's decision, so it is theirs to end now; nothing written", id)
		case r.State == status.Superseded:
			return nil, nil, fmt.Errorf("#%d is superseded by #%d; nothing to withdraw", id, r.SupersededBy)
		case !r.Withdrawable():
			return nil, nil, fmt.Errorf("#%d is %s; nothing to withdraw", id, r.State)
		}
		others = nil
		for _, o := range st.Requests {
			if o != r && o.Entry.Item == r.Entry.Item && o.Entry.Item != "" && (o.Withdrawable() || o.State == status.AwaitingOperator) {
				others = append(others, fmt.Sprintf("#%d", o.Entry.ID))
			}
		}
		return &record.Entry{Kind: "withdraw", Item: r.Entry.Item, From: role, To: r.Entry.To, Re: id, Body: strings.TrimSpace(*note)}, nil, nil
	})
	if err != nil {
		return 0, err
	}
	a.receipt(got, existed)
	a.alert()
	if len(others) > 0 && !existed {
		// Withdrawing one reviewer's request must not let the rest approve
		// the item without the operator (row 118).
		fmt.Fprintf(a.stderr, "handoff: %s is still in review (%s). While %s isn't asked again on it, the operator's go-ahead doesn't approve it: the operator decides it.\n",
			got.Item, strings.Join(others, ", "), got.To)
	}
	return exitOK, nil
}
