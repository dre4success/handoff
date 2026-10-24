package wait

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
)

type fakeClock struct {
	now   time.Time
	ticks int
	hook  func(c *fakeClock)
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(d time.Duration) {
	c.ticks++
	c.now = c.now.Add(d)
	if c.hook != nil {
		c.hook(c)
	}
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)}
}

func newStore(t *testing.T) *record.Store {
	t.Helper()
	dir := t.TempDir()
	if err := record.Create(dir); err != nil {
		t.Fatal(err)
	}
	return record.New(dir)
}

func post(t *testing.T, s *record.Store, to, body string) record.Entry {
	t.Helper()
	e, _, err := s.Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) {
		return &record.Entry{Kind: "note", Item: "W1", From: "implementer", To: to, Body: body}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func run(t *testing.T, s *record.Store, opts Options) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if opts.Out == nil {
		opts.Out = &out
	}
	opts.Err = &errOut
	if opts.Interval == 0 {
		opts.Interval = time.Second
	}
	code, err := Run(s, opts)
	if err != nil {
		t.Fatalf("Run: %v (stderr %q)", err, errOut.String())
	}
	return code, out.String(), errOut.String()
}

func cursor(t *testing.T, s *record.Store, role string) int64 {
	t.Helper()
	c, err := ReadCursor(s.Dir(), role)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Check 13: entries posted while no wait runs are delivered by the next wait at once.
func TestDeliversPendingEntriesAtOnce(t *testing.T) {
	s := newStore(t)
	post(t, s, "reviewer", "one")
	post(t, s, "implementer", "not for the reviewer")
	last := post(t, s, "reviewer", "two")
	clock := newClock()
	code, out, _ := run(t, s, Options{Role: "reviewer", Timeout: time.Minute, Clock: clock})
	if code != ExitDelivered || clock.ticks != 0 {
		t.Fatalf("code %d after %d ticks, want %d at once", code, clock.ticks, ExitDelivered)
	}
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 || !strings.HasPrefix(lines[0], "#1 ") || !strings.HasPrefix(lines[1], "#3 ") {
		t.Fatalf("output %q, want #1 and #3", out)
	}
	if got := cursor(t, s, "reviewer"); got != last.ID {
		t.Fatalf("cursor = %d, want %d", got, last.ID)
	}
	code, out, _ = run(t, s, Options{Role: "reviewer", Timeout: 3 * time.Second, Clock: newClock()})
	if code != ExitTimeout || out != "" {
		t.Fatalf("second wait: code %d, out %q; want a timeout with nothing delivered twice", code, out)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

func TestCursorMovesOnlyAfterPrinting(t *testing.T) {
	s := newStore(t)
	post(t, s, "reviewer", "one")
	var errOut bytes.Buffer
	_, err := Run(s, Options{Role: "reviewer", Timeout: time.Minute, Interval: time.Second, Clock: newClock(), Out: failingWriter{}, Err: &errOut})
	if err == nil {
		t.Fatal("a failed print must be an error")
	}
	if got := cursor(t, s, "reviewer"); got != 0 {
		t.Fatalf("cursor = %d after a failed print, want 0", got)
	}
}

func TestWakesWhenAnEntryLands(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		if c.ticks == 3 {
			post(t, s, "reviewer", "landed")
		}
	}
	code, out, _ := run(t, s, Options{Role: "reviewer", Timeout: time.Minute, Clock: clock})
	if code != ExitDelivered || clock.ticks != 3 || !strings.Contains(out, "landed") {
		t.Fatalf("code %d after %d ticks, out %q", code, clock.ticks, out)
	}
}

func TestTimeoutCountsAwakeTicks(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	code, _, _ := run(t, s, Options{Role: "reviewer", Timeout: 5 * time.Second, Clock: clock})
	if code != ExitTimeout || clock.ticks != 5 {
		t.Fatalf("code %d after %d ticks, want a timeout after 5", code, clock.ticks)
	}
}

// The deadline is wall-clock: a sleep that jumps the clock ends the wait at
// once, with the gap reported, instead of 25 more minutes of awake time.
func TestDeadlineIsWallClockAndGapIsReported(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		if c.ticks == 2 {
			c.now = c.now.Add(2 * time.Hour)
		}
	}
	code, _, errOut := run(t, s, Options{Role: "reviewer", Timeout: 25 * time.Minute, Clock: clock})
	if code != ExitTimeout || clock.ticks != 2 {
		t.Fatalf("code %d after %d ticks, want a timeout right after the jump", code, clock.ticks)
	}
	if want := "handoff: wait: the machine may have slept from 2026-10-09T20:00:01Z to 2026-10-09T22:00:02Z (2h0m1s)\n"; errOut != want {
		t.Fatalf("stderr %q, want %q", errOut, want)
	}
}

// A sleeping Mac wakes for moments: in the trial, every quarter hour, and
// in one spell every half minute, each wake reported as a gap of its own
// (evidence/trial/implementer-wait.log). Late ticks less than two minutes
// apart are one sleep, reported once, as soon as the machine has been awake
// two minutes, or when the wait ends.
func TestOneSleepWithBriefWakesIsReportedOnce(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	jumps := map[int]time.Duration{5: 15 * time.Minute, 16: 18 * time.Second, 27: 19 * time.Second, 38: 15 * time.Minute, 400: 5 * time.Minute}
	var marks []time.Time // the last poll before each jump, and the clock after it
	var errOut bytes.Buffer
	var midway string
	clock.hook = func(c *fakeClock) {
		if d, ok := jumps[c.ticks]; ok {
			marks = append(marks, c.now.Add(-time.Second), c.now.Add(d))
			c.now = c.now.Add(d)
		}
		if c.ticks == 38+125 {
			midway = errOut.String()
		}
	}
	code, err := Run(s, Options{Role: "reviewer", Timeout: 40 * time.Minute, Interval: time.Second, Clock: clock, Out: io.Discard, Err: &errOut})
	if err != nil || code != ExitTimeout || clock.ticks != 400 {
		t.Fatalf("code %d, err %v after %d ticks; want a timeout right after the last sleep", code, err, clock.ticks)
	}
	line := func(from, to time.Time, wakes string) string {
		return fmt.Sprintf("handoff: wait: the machine may have slept from %s to %s (%s%s)\n",
			from.Format(time.RFC3339), to.Format(time.RFC3339), to.Sub(from), wakes)
	}
	first := line(marks[0], marks[7], ", waking briefly 3 times")
	if midway != first {
		t.Fatalf("after two minutes awake, stderr %q; want %q", midway, first)
	}
	if want := first + line(marks[8], marks[9], ""); errOut.String() != want {
		t.Fatalf("stderr %q; want %q", errOut.String(), want)
	}
}

// A pause under a minute is not worth a line: a short wake from sleep, or a
// busy machine.
func TestAPauseUnderAMinuteIsNotReported(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		if c.ticks == 3 {
			c.now = c.now.Add(50 * time.Second)
		}
	}
	if _, _, errOut := run(t, s, Options{Role: "reviewer", Timeout: 5 * time.Minute, Clock: clock}); errOut != "" {
		t.Fatalf("stderr %q, want nothing", errOut)
	}
}

// The wait keeps the machine from idle sleep while an agent owes a move,
// and lets it sleep while nothing is owed, while only the operator owes one, once a
// move has been owed for AwakeWithin, and when the wait ends.
func TestAwakeWhileAnAgentOwesAMove(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	s.Now = clock.Now
	add := func(e record.Entry) int64 {
		got, _, err := s.Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) { return &e, nil, nil })
		if err != nil {
			t.Fatal(err)
		}
		return got.ID
	}
	var req int64
	clock.hook = func(c *fakeClock) {
		switch c.ticks {
		case 3:
			req = add(record.Entry{Kind: "request", Item: "W1", From: "implementer", To: "reviewer", Commit: strings.Repeat("a", 40), Title: "t"})
		case 6:
			add(record.Entry{Kind: "reply", Item: "W1", From: "reviewer", To: "implementer", Re: req, Verdict: "APPROVE", Commit: strings.Repeat("a", 40)})
		case 9:
			add(record.Entry{Kind: "request", Item: "W2", From: "implementer", To: "reviewer", Commit: strings.Repeat("b", 40), Title: "t"})
		case 12:
			c.now = c.now.Add(AwakeWithin)
		}
	}
	var got []string
	awake := func(on bool) {
		got = append(got, fmt.Sprintf("%d:%v", clock.ticks, on))
	}
	code, _, _ := run(t, s, Options{Role: "reviewer-2", Timeout: AwakeWithin + time.Minute, Awake: awake, Clock: clock})
	if code != ExitTimeout {
		t.Fatalf("code %d", code)
	}
	var changes []string
	for i, g := range got {
		if i == 0 || strings.SplitN(g, ":", 2)[1] != strings.SplitN(got[i-1], ":", 2)[1] {
			changes = append(changes, g)
		}
	}
	if want := []string{"0:false", "3:true", "6:false", "9:true", "12:false"}; !slices.Equal(changes, want) {
		t.Fatalf("awake changed at %v (tick:on), want %v", changes, want)
	}
	if last := got[len(got)-1]; !strings.HasSuffix(last, ":false") {
		t.Fatalf("the last call was %s; a wait that ends must let the machine sleep", last)
	}
}

// After a sleep, an entry that is waiting is delivered before an expired
// deadline is treated as an empty timeout.
func TestEntryIsDeliveredBeforeAnExpiredDeadline(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		if c.ticks == 2 {
			post(t, s, "reviewer", "posted just before the jump")
			c.now = c.now.Add(time.Hour)
		}
	}
	code, out, errOut := run(t, s, Options{Role: "reviewer", Timeout: 25 * time.Minute, Clock: clock})
	if code != ExitDelivered || !strings.Contains(out, "posted just before the jump") {
		t.Fatalf("code %d, out %q; want the entry delivered", code, out)
	}
	if !strings.Contains(errOut, "the machine may have slept from 2026-10-09T20:00:01Z to 2026-10-09T21:00:02Z (1h0m1s)") {
		t.Fatalf("stderr %q does not report the sleep that ended with a delivery", errOut)
	}
}

func TestEmitStreamsUntilTheDeadline(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		switch c.ticks {
		case 2:
			post(t, s, "reviewer", "first")
		case 4:
			post(t, s, "reviewer", "second")
		}
	}
	code, out, _ := run(t, s, Options{Role: "reviewer", Timeout: 10 * time.Second, Emit: true, Clock: clock})
	if code != ExitTimeout || clock.ticks != 10 {
		t.Fatalf("code %d after %d ticks, want a timeout after 10", code, clock.ticks)
	}
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 {
		t.Fatalf("emitted %q, want two lines", out)
	}
	if got := cursor(t, s, "reviewer"); got != 2 {
		t.Fatalf("cursor = %d, want 2", got)
	}
}

// Check 13, second half: an entry posted between two arms of an emitting
// watcher is delivered by the second arm.
func TestEntryBetweenTwoArmsIsDelivered(t *testing.T) {
	s := newStore(t)
	code, _, _ := run(t, s, Options{Role: "reviewer", Timeout: 2 * time.Second, Emit: true, Clock: newClock()})
	if code != ExitTimeout {
		t.Fatalf("first arm: code %d", code)
	}
	post(t, s, "reviewer", "between arms")
	_, out, _ := run(t, s, Options{Role: "reviewer", Timeout: 2 * time.Second, Emit: true, Clock: newClock()})
	if !strings.Contains(out, "between arms") {
		t.Fatalf("second arm emitted %q", out)
	}
}

func TestWarnsAboutADamagedRecord(t *testing.T) {
	s := newStore(t)
	post(t, s, "reviewer", "one")
	if err := record.WriteFileAtomic(s.Dir()+"/"+record.HeadName, []byte(`{"v":1,"lines":9,"last":"x","id":9}`)); err != nil {
		t.Fatal(err)
	}
	_, _, errOut := run(t, s, Options{Role: "reviewer", Timeout: time.Second, Clock: newClock()})
	if !strings.Contains(errOut, "head.json") {
		t.Fatalf("stderr %q does not carry the advisory", errOut)
	}
}

// appendForged writes a well-formed line whose prev is wrong, as GPT-6's H5
// probe did, to a record that a wait may already be watching.
func appendForged(t *testing.T, s *record.Store, id int) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(s.Dir(), record.FileName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, `{"v":1,"id":%d,"ts":"2026-10-09T20:30:00Z","kind":"request","item":"W1","from":"implementer","to":"reviewer","commit":"x","body":"forged","prev":"broken"}`+"\n", id)
}

// H5 CHANGES 1: damage that arrives while an emitting wait is already
// running is reported, nothing past it is delivered, and the cursor stays.
func TestDamageArrivingDuringAWaitStopsIt(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		switch c.ticks {
		case 1:
			post(t, s, "reviewer", "valid request")
		case 3:
			appendForged(t, s, 99)
		}
	}
	var out, errOut bytes.Buffer
	_, err := Run(s, Options{Role: "reviewer", Timeout: time.Minute, Emit: true, Interval: time.Second, Clock: clock, Out: &out, Err: &errOut})
	if !errors.Is(err, record.ErrBroken) {
		t.Fatalf("err = %v, want ErrBroken", err)
	}
	if !strings.Contains(out.String(), "valid request") || strings.Contains(out.String(), "#99") {
		t.Fatalf("delivered %q; want #1 only", out.String())
	}
	if !strings.Contains(errOut.String(), "fails at line 2") {
		t.Fatalf("stderr %q does not report the break", errOut.String())
	}
	if got := cursor(t, s, "reviewer"); got != 1 {
		t.Fatalf("cursor = %d, want 1", got)
	}
}

// On a record that is already broken, only verified entries are delivered.
func TestBrokenRecordDeliversOnlyVerifiedEntries(t *testing.T) {
	s := newStore(t)
	for _, b := range []string{"one", "two", "three"} {
		post(t, s, "reviewer", b)
	}
	appendForged(t, s, 99)
	var out, errOut bytes.Buffer
	_, err := Run(s, Options{Role: "reviewer", Timeout: time.Minute, Interval: time.Second, Clock: newClock(), Out: &out, Err: &errOut})
	if !errors.Is(err, record.ErrBroken) {
		t.Fatalf("err = %v, want ErrBroken", err)
	}
	// head.json still names #3 as the last line, so only the forged line is
	// withheld (gate finding 2).
	if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != 3 || !strings.HasPrefix(lines[2], "#3 ") {
		t.Fatalf("delivered %q; want #1 to #3", out.String())
	}
	if got := cursor(t, s, "reviewer"); got != 3 {
		t.Fatalf("cursor = %d, want 3", got)
	}
}

// Gate finding 1: an edited last line is not delivered, and the wait stops.
func TestEditedLastLineIsNotDelivered(t *testing.T) {
	s := newStore(t)
	post(t, s, "reviewer", "please approve W1")
	path := filepath.Join(s.Dir(), record.FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte("W1\""), []byte("W99\""), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	_, err = Run(s, Options{Role: "reviewer", Timeout: time.Minute, Interval: time.Second, Clock: newClock(), Out: &out, Err: &errOut})
	if !errors.Is(err, record.ErrBroken) || out.String() != "" {
		t.Fatalf("err %v, delivered %q; want ErrBroken and nothing delivered", err, out.String())
	}
	if got := cursor(t, s, "reviewer"); got != 0 {
		t.Fatalf("cursor = %d, want 0", got)
	}
}

// A new head.json advisory during a wait is printed when it appears, and the
// wait goes on (row 51).
func TestNewAdvisoryDuringAWaitIsPrinted(t *testing.T) {
	s := newStore(t)
	post(t, s, "implementer", "not for the reviewer")
	clock := newClock()
	clock.hook = func(c *fakeClock) {
		if c.ticks == 2 {
			record.WriteFileAtomic(filepath.Join(s.Dir(), record.HeadName), []byte(`{"v":1,"lines":9,"last":"x","id":9}`))
		}
	}
	code, _, errOut := run(t, s, Options{Role: "reviewer", Timeout: 5 * time.Second, Clock: clock})
	if code != ExitTimeout || !strings.Contains(errOut, "head.json disagrees") {
		t.Fatalf("code %d, stderr %q; want the advisory printed and the wait to go on", code, errOut)
	}
	if strings.Count(errOut, "head.json disagrees") != 1 {
		t.Fatalf("the advisory was printed more than once: %q", errOut)
	}
}

// H8: a silence that comes due while a wait runs is passed on for the operator during
// the wait, including when the machine sleeps through the moment it came due.
func TestWaitPassesOnASilenceThatComesDue(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	s.Now = clock.Now
	if _, _, err := s.Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) {
		return &record.Entry{Kind: "request", Item: "H8", From: "implementer", To: "reviewer", Commit: strings.Repeat("a", 40), Title: "t"}, nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	clock.hook = func(c *fakeClock) {
		if c.ticks == 60 {
			c.now = c.now.Add(40 * time.Minute) // asleep
		}
	}
	first := map[string]string{}
	var seenAt time.Time
	notify := func(as []status.Alert) {
		for _, a := range as {
			if _, ok := first[a.Key]; !ok {
				first[a.Key], seenAt = a.Text, clock.Now()
			}
		}
	}
	code, out, _ := run(t, s, Options{Role: "implementer", Timeout: 45 * time.Minute, Quiet: 30 * time.Minute, Notify: notify, Clock: clock})
	if code != ExitTimeout || out != "" {
		t.Fatalf("code %d, out %q; want a timeout with nothing delivered", code, out)
	}
	if len(first) != 1 {
		t.Fatalf("alerts %v; want one", first)
	}
	for _, text := range first {
		if !strings.Contains(text, "reviewer hasn't picked up #1 (41 min)") {
			t.Fatalf("alert %q", text)
		}
	}
	if want := newClock().now.Add(41 * time.Minute); !seenAt.Equal(want) {
		t.Fatalf("first passed on at %s, want %s, the first tick after waking", seenAt, want)
	}
}

func TestWaitWithoutNotifyStillWorks(t *testing.T) {
	s := newStore(t)
	post(t, s, "reviewer", "one")
	if code, _, _ := run(t, s, Options{Role: "reviewer", Timeout: time.Minute, Quiet: time.Second, Clock: newClock()}); code != ExitDelivered {
		t.Fatalf("code %d", code)
	}
}

// A wait that returns at once with an entry still passes on what is due, so
// a busy session that never waits out a tick does not hide a silence.
func TestWaitThatDeliversAtOncePassesAlertsOn(t *testing.T) {
	s := newStore(t)
	clock := newClock()
	s.Now = func() time.Time { return clock.now.Add(-time.Hour) }
	if _, _, err := s.Append(func(*record.Snapshot) (*record.Entry, *record.Entry, error) {
		return &record.Entry{Kind: "request", Item: "H8", From: "implementer", To: "reviewer", Commit: strings.Repeat("a", 40), Title: "t"}, nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	post(t, s, "implementer", "for the implementer")
	var got []status.Alert
	code, _, _ := run(t, s, Options{Role: "implementer", Timeout: time.Minute, Quiet: 30 * time.Minute, Notify: func(as []status.Alert) { got = append(got, as...) }, Clock: clock})
	if code != ExitDelivered || clock.ticks != 0 {
		t.Fatalf("code %d after %d ticks; want a delivery at once", code, clock.ticks)
	}
	if len(got) != 1 || !strings.Contains(got[0].Text, "hasn't picked up #1") {
		t.Fatalf("alerts %v; want the hour-old request passed on", got)
	}
}
