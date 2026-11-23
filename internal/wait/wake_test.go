package wait

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

// H35 part 2 (row 116): Wake ends a wait with what it says, delivered as an
// entry is: under the fence, printed, then acknowledged; never in Emit mode.
func TestH35p2WakeEndsTheWaitWithItsText(t *testing.T) {
	s := newStore(t)
	asked, sent := 0, 0
	wake := func(*status.Status) (string, func(func(string) error) error) {
		if asked++; asked < 3 {
			return "", nil
		}
		return "reviewer's seat is stuck", func(print func(string) error) error {
			if err := print("reviewer's seat is stuck"); err != nil {
				return err
			}
			sent++
			return nil
		}
	}
	code, out, _ := run(t, s, Options{Role: "spare", Timeout: 10 * time.Minute, Interval: time.Minute, Clock: newClock(), Check: func(*status.Status) {}, Wake: wake})
	if code != ExitDelivered || strings.TrimSpace(out) != "reviewer's seat is stuck" || asked != 3 || sent != 1 {
		t.Fatalf("exit %d, %q, asked %d times, sent %d", code, out, asked, sent)
	}
	asked, sent = 0, 0
	code, out, _ = run(t, s, Options{Role: "spare", Timeout: 3 * time.Minute, Interval: time.Minute, Clock: newClock(), Emit: true, Check: func(*status.Status) {}, Wake: wake})
	if code != ExitTimeout || out != "" || asked != 0 {
		t.Fatalf("emit: exit %d, %q, asked %d times", code, out, asked)
	}
}

// reviewer-2's #853: a listener whose role moved delivers no wake and
// doesn't mark it sent; a wake that fails to print is an error, not sent.
func TestH35p2AWakeIsFencedAndSentOnlyOncePrinted(t *testing.T) {
	s := newStore(t)
	sent := 0
	wake := func(*status.Status) (string, func(func(string) error) error) {
		return "reviewer's seat is stuck", func(print func(string) error) error {
			if err := print("reviewer's seat is stuck"); err != nil {
				return err
			}
			sent++
			return nil
		}
	}
	fenced := func(func() error) error { return ErrFenced }
	code, out, _ := run(t, s, Options{Role: "spare", Timeout: time.Minute, Interval: time.Minute, Clock: newClock(), Check: func(*status.Status) {}, Wake: wake, Fence: fenced})
	if code != ExitTimeout || out != "" || sent != 0 {
		t.Fatalf("fenced: exit %d, %q, sent %d", code, out, sent)
	}
	var errOut strings.Builder
	_, err := Run(s, Options{Role: "spare", Timeout: time.Minute, Interval: time.Minute, Clock: newClock(), Check: func(*status.Status) {}, Wake: wake, Out: failing{}, Err: &errOut})
	if err == nil || !strings.Contains(err.Error(), "printing the wake") || sent != 0 {
		t.Fatalf("a failing writer: err %v, sent %d", err, sent)
	}
}

// reviewer-2's #862: a wake that no longer holds under the lock, as when
// the operator came back, is dropped: nothing printed, and the wait goes on
// to its end.
func TestH35p2AStaleWakeIsDropped(t *testing.T) {
	s := newStore(t)
	stale := func(*status.Status) (string, func(func(string) error) error) {
		return "reviewer's seat is stuck", func(func(string) error) error { return ErrStale }
	}
	code, out, _ := run(t, s, Options{Role: "spare", Timeout: 3 * time.Minute, Interval: time.Minute, Clock: newClock(), Check: func(*status.Status) {}, Wake: stale})
	if code != ExitTimeout || out != "" {
		t.Fatalf("a stale wake: exit %d, %q", code, out)
	}
}

type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }
