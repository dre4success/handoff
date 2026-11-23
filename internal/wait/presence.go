package wait

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dre4success/handoff/internal/record"
)

// Presence (proposal §8, §17 row 67). A process that waits for a role holds
// presence/<role>.lock, an exclusive flock, for as long as it waits. So two
// waits never deliver the same entries, and a second one can tell that a
// role is already listening. The kernel drops the lock when its process
// exits, killed or not, so no PID or file age is trusted. presence/<role>.json
// says who holds it, for people.

// ErrListening means another live process is waiting for the role.
var ErrListening = errors.New("already listening")

// Presence is what presence/<role>.json records.
type Presence struct {
	V     int       `json:"v"`
	PID   int       `json:"pid"`
	Since time.Time `json:"since"`
	How   string    `json:"how"` // "wait", or "hook" for a CLI's hook
	// Rounds says the listener reads a round's requests (row 106). One
	// without it is a handoff from before them, which can't see a round
	// replace anything, so post refuses a round while it listens.
	Rounds bool `json:"rounds,omitempty"`
	// Ended is when the listener let the lock go, if it did so itself: a
	// listener killed never writes it (row 110).
	Ended time.Time `json:"ended,omitempty"`
}

// PresencePath is where the role's presence is kept, inside the record
// directory. Its lock is the same path with .lock added.
func PresencePath(dir, role string) string {
	return filepath.Join(dir, "presence", role+".json")
}

// Listen takes the role's lock and records this process as listening. The
// lock lasts until the returned file is closed or the process exits. If
// another process holds it, Listen returns ErrListening, with who holds it
// when presence/<role>.json says.
func Listen(dir, role, how string) (*os.File, error) {
	path := PresencePath(dir, role)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// A reader's shared lock lasts microseconds; look again briefly before
	// deciding someone else is listening.
	for try := 0; ; try++ {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || try == 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		f.Close()
		if p, rerr := ReadPresence(dir, role); rerr == nil {
			return nil, fmt.Errorf("%w: %s for %s, pid %d, since %s", ErrListening, p.How, role, p.PID, p.Since.UTC().Format(time.RFC3339))
		}
		return nil, fmt.Errorf("%w for %s", ErrListening, role)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path+".lock", err)
	}
	b, _ := json.Marshal(Presence{V: record.Version, PID: os.Getpid(), Since: time.Now().UTC().Round(time.Second), How: how, Rounds: true})
	if err := record.WriteFileAtomic(path, append(b, '\n')); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Release notes when the listener ends, then lets the role's lock go: so
// "last listened" can say when, not only when it began (row 110).
func Release(f *os.File, dir, role string) {
	if p, err := ReadPresence(dir, role); err == nil && p.PID == os.Getpid() {
		p.Ended = time.Now().UTC().Round(time.Second)
		if b, err := json.Marshal(p); err == nil {
			record.WriteFileAtomic(PresencePath(dir, role), append(b, '\n'))
		}
	}
	f.Close()
}

// Listening says whether a process holds role's lock now: a wait, or a CLI's
// hook. It takes the lock shared, for a moment, to see.
func Listening(dir, role string) bool {
	f, err := os.Open(PresencePath(dir, role) + ".lock")
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err == nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}
	return errors.Is(err, syscall.EWOULDBLOCK)
}

// Older lists the roles, of those given, that a listener from a handoff
// older than row 106 holds now, with its pid: one whose presence doesn't say
// it reads rounds (row 106).
func Older(dir string, roles []string) []string {
	var out []string
	for _, role := range roles {
		if !Listening(dir, role) {
			continue
		}
		if p, err := ReadPresence(dir, role); err != nil {
			out = append(out, role)
		} else if !p.Rounds {
			out = append(out, fmt.Sprintf("%s (pid %d)", role, p.PID))
		}
	}
	return out
}

// ReadPresence returns what presence/<role>.json last recorded. It says who
// listened last, not that they still do: only the lock says that.
func ReadPresence(dir, role string) (Presence, error) {
	var p Presence
	b, err := os.ReadFile(PresencePath(dir, role))
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}
