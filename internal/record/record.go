// Package record keeps the channel record: an append-only file of JSON lines,
// each naming the hash of the line before it, plus head.json, which counts the
// lines so that a cut tail can be seen.
package record

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	FileName = "channel.jsonl"
	HeadName = "head.json"
	LockName = "lock"
	// Genesis is the prev of the first line.
	Genesis = "genesis"
	// Version is the schema version written in every entry and in head.json.
	Version = 1
)

var (
	ErrNoRecord        = errors.New("no record here")
	ErrTornTail        = errors.New("the record ends with an incomplete line")
	ErrBroken          = errors.New("the record's hash chain does not verify")
	ErrNothingToRepair = errors.New("nothing to repair")
)

// Entry is one line of the record. Prev is the last field, so a write cut
// short never leaves a complete-looking line.
type Entry struct {
	V          int      `json:"v"`
	ID         int64    `json:"id"`
	TS         string   `json:"ts"`
	Kind       string   `json:"kind"`
	Item       string   `json:"item,omitempty"`
	From       string   `json:"from"`
	To         string   `json:"to,omitempty"`
	Title      string   `json:"title,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	Base       string   `json:"base,omitempty"`
	Parents    []string `json:"parents,omitempty"`
	Re         int64    `json:"re,omitempty"`
	Supersedes int64    `json:"supersedes,omitempty"`
	Verdict    string   `json:"verdict,omitempty"`
	Decision   string   `json:"decision,omitempty"`
	Key        string   `json:"key,omitempty"`
	Body       string   `json:"body"`
	Damage     *Damage  `json:"damage,omitempty"`
	Prev       string   `json:"prev"`
}

// Damage describes bytes that a repair entry closed and kept. The repair
// entry's prev is the hash of those bytes; After is the hash of the last good
// line before them, so the chain still covers every good line.
type Damage struct {
	Line   int    `json:"line"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	After  string `json:"after"`
}

// Head is head.json. It cannot be written atomically together with the
// record, so it is advisory (proposal §5, row 51): a disagreement is reported
// and kept in Advisory until repair, and never refuses a post.
type Head struct {
	V        int    `json:"v"`
	Lines    int    `json:"lines"`
	Last     string `json:"last"`
	ID       int64  `json:"id"`
	Advisory string `json:"advisory,omitempty"`
}

// Problem is the first place the chain fails to verify.
type Problem struct {
	Line     int  // the first line whose link to the line before it fails
	Withheld int  // the first line withheld from status and delivery
	ByHead   bool // found by head.json, not by the chain: an edited last line
	Detail   string
}

// Snapshot is the record as read at one moment.
type Snapshot struct {
	Entries    []Entry // verified entries, in order; the only ones status and delivery use
	Unverified []Entry // entries that parse but are withheld by a break, for forensic reading
	Lines      int     // complete lines, damaged ones included
	LastHash   string  // hash of the last complete line, or Genesis
	LastID     int64   // highest id among all parsed entries
	Torn       []byte  // trailing bytes with no newline, or nil
	Broken     *Problem
	Head       Head
	headNote   string // how head.json disagrees with the record now, or ""
	hashes     []string
}

// HashLine is the hex SHA-256 of a line's bytes, without its newline.
func HashLine(line []byte) string {
	sum := sha256.Sum256(line)
	return hex.EncodeToString(sum[:])
}

// NextID is the id the next entry gets. It never reuses an id that head.json
// has seen, so a cut tail cannot make a delivered id point at a new entry.
func (s *Snapshot) NextID() int64 {
	return max(s.LastID, s.Head.ID) + 1
}

// Advisories lists how head.json disagrees with the record: the one kept
// since an earlier post, and any new one.
func (s *Snapshot) Advisories() []string {
	var out []string
	if s.Head.Advisory != "" {
		out = append(out, s.Head.Advisory)
	}
	if s.headNote != "" {
		out = append(out, s.headNote)
	}
	return out
}

// Warnings describes everything wrong with the record, for readers to print.
func (s *Snapshot) Warnings() []string {
	var out []string
	if s.Torn != nil {
		out = append(out, fmt.Sprintf("the record ends with %d bytes that are not a complete line; run `handoff repair`", len(s.Torn)))
	}
	if s.Broken != nil {
		out = append(out, fmt.Sprintf("the hash chain fails at line %d: %s; entries from line %d on are withheld from inbox and wait, and posting stops until the operator decides", s.Broken.Line, s.Broken.Detail, s.Broken.Withheld))
	}
	for _, a := range s.Advisories() {
		out = append(out, "advisory: "+a+"; `handoff repair` acknowledges it")
	}
	return out
}

// Find returns the verified entry with the given id.
func (s *Snapshot) Find(id int64) (*Entry, bool) {
	return find(s.Entries, id)
}

// FindUnverified returns an entry withheld by a break, for forensic reading.
func (s *Snapshot) FindUnverified(id int64) (*Entry, bool) {
	return find(s.Unverified, id)
}

func find(es []Entry, id int64) (*Entry, bool) {
	for i := range es {
		if es[i].ID == id {
			return &es[i], true
		}
	}
	return nil, false
}

func decode(line []byte) (Entry, error) {
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return e, err
	}
	if e.V != Version {
		return e, fmt.Errorf("schema version %d, want %d", e.V, Version)
	}
	if e.Kind == "" || e.ID <= 0 || e.Prev == "" {
		return e, errors.New("missing kind, id or prev")
	}
	return e, nil
}

func encode(e *Entry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// parse reads the record's bytes and verifies the chain against head, which
// is nil when head.json cannot be read. It never fails: damage is described in
// the snapshot. Entries the chain does not verify are kept apart, in
// Unverified, so that no reader mistakes them for work.
func parse(data []byte, head *Head) *Snapshot {
	snap := &Snapshot{LastHash: Genesis}
	complete := data
	if n := len(data); n > 0 && data[n-1] != '\n' {
		i := bytes.LastIndexByte(data, '\n')
		snap.Torn = append([]byte(nil), data[i+1:]...)
		complete = data[:i+1]
	}
	var lines [][]byte
	if len(complete) > 0 {
		lines = bytes.Split(complete[:len(complete)-1], []byte("\n"))
	}
	snap.Lines = len(lines)
	snap.hashes = make([]string, len(lines))
	for i, l := range lines {
		snap.hashes[i] = HashLine(l)
	}

	type parsed struct {
		line int
		e    Entry
	}
	var all []parsed
	prev := Genesis
	for i, l := range lines {
		e, err := decode(l)
		if err == nil && e.Prev == prev && e.ID > snap.LastID {
			all = append(all, parsed{i, e})
			snap.LastID = e.ID
			prev = snap.hashes[i]
			continue
		}
		if coveredByRepair(lines, snap.hashes, i, prev) {
			prev = snap.hashes[i]
			continue
		}
		if snap.Broken == nil {
			snap.Broken = &Problem{Line: i + 1, Withheld: max(i, 1), Detail: describe(e, err, prev, snap.LastID)}
		}
		if err == nil && e.ID > snap.LastID {
			all = append(all, parsed{i, e})
			snap.LastID = e.ID
		}
		prev = snap.hashes[i]
	}
	snap.LastHash = prev
	if head != nil {
		snap.witness(head)
	}

	// An entry is verified when the line after it names its hash; the last
	// line is verified by head.json. At a break the chain cannot tell which
	// side changed, an edit to the line before or to the line itself, so the
	// line before the break is withheld as well as every line from the break
	// on, unless head.json vouches for it.
	for _, p := range all {
		if snap.Broken != nil && p.line+1 >= snap.Broken.Withheld {
			snap.Unverified = append(snap.Unverified, p.e)
		} else {
			snap.Entries = append(snap.Entries, p.e)
		}
	}
	return snap
}

// witness lets head.json speak for the line it names (proposal §17, rows 52,
// 53 and 58). head.json is written only after the record is synced, so it
// can be stale but never ahead of a write. Same count with another last hash
// is an edit, to the last line or to head.json, when the last entry is one
// head.json has seen. If the last entry is newer than any head.json has seen,
// lines were cut and a later write stopped before head.json: stale, not
// edited.
func (s *Snapshot) witness(h *Head) {
	switch {
	case s.Broken != nil && h.Lines > 0 && h.Lines == s.Broken.Line-1 && h.Last == s.hashAt(h.Lines):
		// head.json names the line before the break as it is now: only the
		// break and what follows are in doubt.
		s.Broken.Withheld = s.Broken.Line
	case s.Broken == nil && s.Lines > 0 && h.Lines == s.Lines && h.Last != s.LastHash && s.LastID <= h.ID:
		s.Broken = &Problem{Line: s.Lines, Withheld: s.Lines, ByHead: true,
			Detail: fmt.Sprintf("head.json counts the same %d lines but names %.12s as the last, and the last line hashes to %.12s, so the last line or head.json was edited", h.Lines, h.Last, s.LastHash)}
	}
}

// coveredByRepair reports whether line i is damage that the next line, a
// repair entry, closed and accounted for.
func coveredByRepair(lines [][]byte, hashes []string, i int, prev string) bool {
	if i+1 >= len(lines) {
		return false
	}
	r, err := decode(lines[i+1])
	return err == nil && r.Kind == "repair" && r.Damage != nil &&
		r.Prev == hashes[i] && r.Damage.SHA256 == hashes[i] && r.Damage.After == prev
}

func describe(e Entry, err error, prev string, lastID int64) string {
	switch {
	case err != nil:
		return "does not parse as an entry: " + err.Error()
	case e.Prev != prev:
		return "prev does not match the line before it"
	default:
		return fmt.Sprintf("id %d does not follow id %d", e.ID, lastID)
	}
}

// compareHead says how head.json disagrees with the record, or "".
func (s *Snapshot) compareHead(headErr error) string {
	h := s.Head
	switch {
	case headErr != nil:
		return fmt.Sprintf("head.json is unreadable (%v); the record has %d lines", headErr, s.Lines)
	case h.Lines == s.Lines && h.Last == s.LastHash:
		return ""
	case h.Lines < s.Lines && h.Last == s.hashAt(h.Lines) && (s.Broken == nil || s.Broken.Line <= h.Lines):
		return fmt.Sprintf("head.json is behind the record: it counts %d lines, the record has %d that chain on from them (a writer stopped between the record and head.json)", h.Lines, s.Lines)
	case h.Lines == s.Lines && s.LastID > h.ID:
		return fmt.Sprintf("head.json predates the last line: it counts %d lines ending in %.12s and last saw #%d, but line %d is now #%d (lines were cut, and a later write stopped before head.json)", h.Lines, h.Last, h.ID, s.Lines, s.LastID)
	default:
		return fmt.Sprintf("head.json disagrees with the record: it counts %d lines ending in %.12s, the record has %d ending in %.12s (a cut tail, or an edited last line)", h.Lines, h.Last, s.Lines, s.LastHash)
	}
}

// hashAt is the hash of line n (1-based), or Genesis for n == 0.
func (s *Snapshot) hashAt(n int) string {
	if n <= 0 || n > len(s.hashes) {
		return Genesis
	}
	return s.hashes[n-1]
}
