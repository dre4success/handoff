package record

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Store is the record in one directory. Every writer holds an exclusive lock
// on the lock file; readers hold a shared one, so they never see half a write.
type Store struct {
	dir string
	Now func() time.Time
}

func New(dir string) *Store { return &Store{dir: dir, Now: time.Now} }

func (s *Store) Dir() string { return s.dir }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

// Create starts an empty record and its head.json in dir.
func Create(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, FileName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return New(dir).writeHead(Head{V: Version, Last: Genesis})
}

func (s *Store) lock(how int) (unlock func(), err error) {
	f, err := os.OpenFile(s.path(LockName), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", s.path(LockName), err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

// Load reads and verifies the record under a shared lock.
func (s *Store) Load() (*Snapshot, error) {
	unlock, err := s.lock(syscall.LOCK_SH)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.read()
}

func (s *Store) read() (*Snapshot, error) {
	data, err := os.ReadFile(s.path(FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoRecord, s.path(FileName))
	}
	if err != nil {
		return nil, err
	}
	var head Head
	var headErr error
	b, err := os.ReadFile(s.path(HeadName))
	if err == nil {
		err = json.Unmarshal(b, &head)
	}
	var witness *Head
	if err != nil {
		headErr, head = err, Head{}
	} else {
		witness = &head
	}
	snap := parse(data, witness)
	snap.Head = head
	if snap.Broken == nil || !snap.Broken.ByHead {
		snap.headNote = snap.compareHead(headErr)
	}
	return snap, nil
}

// Append adds one entry under the exclusive lock. build sees the record as it
// is under the lock and returns the entry to write, or an existing entry to
// return as the receipt (a retry), or an error to refuse. Append fills in v,
// id, ts and prev. It refuses a record with a torn tail or a broken chain.
func (s *Store) Append(build func(*Snapshot) (*Entry, *Entry, error)) (Entry, bool, error) {
	unlock, err := s.lock(syscall.LOCK_EX)
	if err != nil {
		return Entry{}, false, err
	}
	defer unlock()
	snap, err := s.read()
	if err != nil {
		return Entry{}, false, err
	}
	if snap.Torn != nil {
		return Entry{}, false, fmt.Errorf("%w (%d bytes after line %d, left by an interrupted write); run `handoff repair`", ErrTornTail, len(snap.Torn), snap.Lines)
	}
	if snap.Broken != nil {
		return Entry{}, false, fmt.Errorf("%w at line %d: %s; posting stops until the operator decides", ErrBroken, snap.Broken.Line, snap.Broken.Detail)
	}
	e, existing, err := build(snap)
	if err != nil {
		return Entry{}, false, err
	}
	if existing != nil {
		return *existing, true, nil
	}
	if e == nil {
		return Entry{}, false, errors.New("nothing to append")
	}
	e.V, e.ID, e.TS, e.Prev = Version, snap.NextID(), s.timestamp(), snap.LastHash
	line, err := encode(e)
	if err != nil {
		return Entry{}, false, err
	}
	if fault("torn") {
		s.writeRecord(line[:len(line)/2])
		killSelf()
	}
	if err := s.writeRecord(append(line, '\n')); err != nil {
		return Entry{}, false, err
	}
	if fault("after-sync") {
		killSelf()
	}
	head := Head{V: Version, Lines: snap.Lines + 1, Last: HashLine(line), ID: e.ID, Advisory: s.carryAdvisory(snap)}
	if err := s.writeHead(head); err != nil {
		return *e, false, fmt.Errorf("entry #%d is in the record, but head.json was not updated: %w", e.ID, err)
	}
	return *e, false, nil
}

// Repair closes a torn tail, keeping its bytes, or rewrites a head.json that
// disagrees with a record whose chain verifies. Either way it appends a repair
// entry saying what it found. It refuses a chain broken in the middle: that is
// not something a tool should mend.
func (s *Store) Repair(from string) (Entry, error) {
	unlock, err := s.lock(syscall.LOCK_EX)
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	snap, err := s.read()
	if err != nil {
		return Entry{}, err
	}
	if snap.Broken != nil {
		return Entry{}, fmt.Errorf("%w at line %d: %s; repair only mends a torn tail or head.json, so the operator decides", ErrBroken, snap.Broken.Line, snap.Broken.Detail)
	}
	e := &Entry{V: Version, TS: s.timestamp(), Kind: "repair", From: from}
	if snap.Torn == nil {
		adv := snap.Advisories()
		if len(adv) == 0 {
			return Entry{}, ErrNothingToRepair
		}
		e.ID, e.Prev = snap.NextID(), snap.LastHash
		e.Body = "head.json rewritten to match the record, whose chain verifies to its last line. It said: " + strings.Join(adv, "; ")
		line, err := encode(e)
		if err != nil {
			return Entry{}, err
		}
		if err := s.writeRecord(append(line, '\n')); err != nil {
			return Entry{}, err
		}
		if fault("after-sync") {
			killSelf()
		}
		return *e, s.writeHead(Head{V: Version, Lines: snap.Lines + 1, Last: HashLine(line), ID: e.ID})
	}

	torn := snap.Torn
	tornHash := HashLine(torn)
	nextID := snap.NextID()
	// A complete entry that lost only its newline stays a valid entry once
	// closed, so the repair must come after its id.
	if d, err := decode(torn); err == nil && d.Prev == snap.LastHash && d.ID >= nextID {
		nextID = d.ID + 1
	}
	e.ID, e.Prev = nextID, tornHash
	e.Damage = &Damage{Line: snap.Lines + 1, Bytes: len(torn), SHA256: tornHash, After: snap.LastHash}
	e.Body = fmt.Sprintf("Closed %d bytes that an interrupted write left without a newline. They are kept unchanged as line %d (sha256 %s).", len(torn), snap.Lines+1, tornHash)
	if adv := snap.Advisories(); len(adv) > 0 {
		e.Body += " head.json rewritten as well; it said: " + strings.Join(adv, "; ")
	}
	line, err := encode(e)
	if err != nil {
		return Entry{}, err
	}
	buf := append([]byte{'\n'}, line...)
	if err := s.writeRecord(append(buf, '\n')); err != nil {
		return Entry{}, err
	}
	if fault("after-sync") {
		killSelf()
	}
	return *e, s.writeHead(Head{V: Version, Lines: snap.Lines + 2, Last: HashLine(line), ID: e.ID})
}

// carryAdvisory keeps an earlier advisory and adds a new disagreement, so
// that one post cannot erase the evidence of a cut tail.
func (s *Store) carryAdvisory(snap *Snapshot) string {
	adv := snap.Head.Advisory
	if snap.headNote == "" {
		return adv
	}
	note := s.timestamp() + ": " + snap.headNote
	if adv == "" {
		return note
	}
	return adv + "; " + note
}

func (s *Store) timestamp() string { return s.Now().UTC().Format(time.RFC3339) }

// writeRecord appends b in one write and syncs it to disk.
func (s *Store) writeRecord(b []byte) error {
	f, err := os.OpenFile(s.path(FileName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// writeHead replaces head.json atomically.
func (s *Store) writeHead(h Head) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.path(HeadName), append(b, '\n'))
}

// WriteFileAtomic writes a file beside its destination, syncs it and renames
// it into place, so a reader sees the old file or the new one, never half.
func WriteFileAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
