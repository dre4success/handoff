package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

// reviewer-3's #709: the alert pass reads a role's delivery before its
// cursor, as whenFyiGets does (#624). The reviewer's listener starts,
// delivers an overdue note and lets go between the two reads, so the
// cursor read after it is past the note, and no "hasn't been given" alert
// follows. Not parallel: the pause point is this process's own, set in its
// environment, which the commands it runs don't inherit.
func TestH34p5ANoteDeliveredBetweenTheReadsGetsNoAlert(t *testing.T) {
	r := newRepo(t)
	rev := claude("s-rev", cli(t))
	r.as(t, rev, "role", "reviewer")
	note := idOf(t, r.ok(t, "implementer", "note", "--to", "reviewer", "--item", "W1", "--body", r.body(t, "look")))
	a := r.app(t)
	p, pause := pauseAt(t, "delivery-reviewer")
	name, value, _ := strings.Cut(pause, "=")
	t.Setenv(name, value)
	got := make(chan map[string]status.Delivery, 1)
	go func() { got <- a.deliveries() }()
	p.reached(t)
	if res := r.run(t, "", "", rev, "wait", "--timeout", "5s"); !strings.Contains(res.out, "#"+note+" note") {
		t.Fatalf("the reviewer's wait: exit %d\n%s%s", res.code, res.out, res.err)
	}
	p.release(t)
	st := status.Compute(r.snap(t).Entries)
	st.SetDelivery(<-got, a.tracked())
	for _, al := range st.Alerts(time.Now().Add(time.Hour), time.Minute) {
		if strings.Contains(al.Text, "hasn't been given the note #"+note) {
			t.Fatalf("an alert for the note the listener delivered: %q", al.Text)
		}
	}
}
