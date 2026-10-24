// Package wait blocks until an entry addressed to a role lands, past that
// role's cursor. The cursor lives on disk, so an entry posted while no wait
// runs, or between two waits, is delivered by the next one.
package wait

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dre4success/handoff/internal/record"
	"github.com/dre4success/handoff/internal/status"
)

const (
	ExitDelivered = 0
	ExitTimeout   = 3
)

// Clock is injected so tests can move wall-clock time, as a sleeping laptop
// does, without sleeping.
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// Real is the system clock.
var Real Clock = realClock{}

type Options struct {
	Role     string
	Timeout  time.Duration
	Emit     bool          // keep streaming until the deadline instead of returning on the first delivery
	Interval time.Duration // how often to stat the record
	Clock    Clock
	Out, Err io.Writer

	// Quiet and Notify pass on, on every tick, what the operator should hear about:
	// a move owed for longer than Quiet, or something awaiting a decision.
	// Notify sends each alert once; it may be nil.
	Quiet  time.Duration
	Notify func([]status.Alert)

	// Alive, when set, is asked at the start of every tick, before anything
	// is read or delivered; false ends the wait as a timeout. A hook's wait
	// uses it to stop when its session has gone, so that a hook resumed
	// after its session died delivers nothing nobody would read.
	Alive func() bool

	// Awake, when set, is told on every tick whether to keep the machine
	// from idle sleep: yes while an agent owes a move made less than
	// AwakeWithin ago. It is told no when the wait ends (row 76).
	Awake func(on bool)
}

// AwakeWithin bounds how long a wait keeps the machine awake for one move:
// a move owed longer than that has stalled, and the operator has been told.
const AwakeWithin = 2 * time.Hour

// A sleep shows as ticks that arrive late. A Mac asleep for an hour wakes
// for a few seconds every quarter hour or so, and in a spell of such wakes,
// every half minute (evidence/trial/implementer-wait.log). So late ticks
// less than mergeSleeps apart are one sleep, reported once, when the machine
// has been awake that long or when the wait ends. A sleep shorter than
// minSleep is not reported.
const (
	mergeSleeps = 2 * time.Minute
	minSleep    = time.Minute
)

type sleep struct {
	from, to time.Time
	wakes    int // brief wakes inside it
}

// Run delivers every entry addressed to the role past its cursor, then polls
// until one lands or the deadline passes. The deadline is wall-clock time:
// Go's monotonic clock stops while a Mac sleeps, so every tick compares
// readings with the monotonic part stripped. On each tick, waiting entries
// are delivered before an expired deadline counts as a timeout.
//
// Only verified entries are delivered. Every new warning about the record is
// printed when it appears, not only at the start, and a broken chain ends the
// wait with an error once the verified entries before it are delivered: no
// cursor moves past a line the chain does not vouch for.
func Run(store *record.Store, o Options) (int, error) {
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
	cur, err := ReadCursor(store.Dir(), o.Role)
	if err != nil {
		return 0, err
	}
	start := o.Clock.Now().Round(0)
	deadline := start.Add(o.Timeout)
	lastTick := start
	var seen [2]fileSig
	var st *status.Status
	printed := map[string]bool{}
	delivered := false
	// pass is checked on every tick, not only when the record changes: a
	// silence changes nothing on disk. After a sleep, the first tick catches up.
	pass := func(now time.Time) {
		if o.Notify != nil && st != nil {
			o.Notify(st.Alerts(now, o.Quiet))
		}
		if o.Awake != nil {
			o.Awake(st != nil && st.Owed(now, AwakeWithin))
		}
	}
	if o.Awake != nil {
		defer o.Awake(false)
	}
	var slept *sleep
	report := func() {
		if slept != nil && slept.to.Sub(slept.from) >= minSleep {
			fmt.Fprintf(o.Err, "handoff: wait: the machine may have slept from %s to %s (%s%s)\n",
				slept.from.UTC().Format(time.RFC3339), slept.to.UTC().Format(time.RFC3339),
				slept.to.Sub(slept.from).Round(time.Second), wakes(slept.wakes))
		}
		slept = nil
	}
	defer report()
	for {
		if o.Alive != nil && !o.Alive() {
			return ExitTimeout, nil
		}
		now := o.Clock.Now().Round(0)
		switch late := now.Sub(lastTick) > 10*o.Interval; {
		case late && slept != nil && lastTick.Sub(slept.to) < mergeSleeps:
			slept.to = now
			slept.wakes++
		case late:
			report()
			slept = &sleep{from: lastTick, to: now}
		case slept != nil && now.Sub(slept.to) >= mergeSleeps:
			report()
		}
		lastTick = now
		sig, err := stats(store.Dir())
		if err != nil {
			return 0, err
		}
		if sig != seen {
			seen = sig
			snap, err := store.Load()
			if err != nil {
				return 0, err
			}
			st = status.Compute(snap.Entries)
			for _, m := range snap.Warnings() {
				if !printed[m] {
					fmt.Fprintln(o.Err, "handoff: "+m)
					printed[m] = true
				}
			}
			for _, e := range snap.Entries {
				if e.To != o.Role || e.ID <= cur {
					continue
				}
				if _, err := fmt.Fprintln(o.Out, status.Summary(e)); err != nil {
					return 0, fmt.Errorf("printing #%d: %w", e.ID, err)
				}
				if f, ok := o.Out.(interface{ Sync() error }); ok {
					f.Sync()
				}
				cur = e.ID
				if err := writeCursor(store.Dir(), o.Role, cur); err != nil {
					return 0, err
				}
				delivered = true
			}
			if b := snap.Broken; b != nil {
				return 0, fmt.Errorf("%w at line %d; wait stops without delivering anything from line %d on; the operator decides", record.ErrBroken, b.Line, b.Withheld)
			}
			if delivered && !o.Emit {
				pass(now)
				return ExitDelivered, nil
			}
		}
		pass(now)
		if !now.Before(deadline) {
			return ExitTimeout, nil
		}
		o.Clock.Sleep(o.Interval)
	}
}

func wakes(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return ", waking once briefly"
	}
	return fmt.Sprintf(", waking briefly %d times", n)
}

type fileSig struct {
	size  int64
	mtime time.Time
}

// stats signs the record and head.json, so a change to either is noticed.
func stats(dir string) ([2]fileSig, error) {
	var sig [2]fileSig
	for i, name := range []string{record.FileName, record.HeadName} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) && name == record.HeadName {
			continue
		}
		if err != nil {
			return sig, err
		}
		sig[i] = fileSig{fi.Size(), fi.ModTime()}
	}
	return sig, nil
}

type cursorFile struct {
	V  int   `json:"v"`
	ID int64 `json:"id"`
}

// CursorPath is where the role's cursor is kept, inside the record directory.
func CursorPath(dir, role string) string {
	return filepath.Join(dir, "cursors", role+".json")
}

// ReadCursor returns the last entry id delivered to the role, or 0.
func ReadCursor(dir, role string) (int64, error) {
	b, err := os.ReadFile(CursorPath(dir, role))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var c cursorFile
	if err := json.Unmarshal(b, &c); err != nil {
		return 0, fmt.Errorf("cursor for %s: %w", role, err)
	}
	return c.ID, nil
}

func writeCursor(dir, role string, id int64) error {
	path := CursorPath(dir, role)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(cursorFile{V: record.Version, ID: id})
	return record.WriteFileAtomic(path, append(b, '\n'))
}
