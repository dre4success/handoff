package record

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	if err := Create(dir); err != nil {
		t.Fatal(err)
	}
	return New(dir)
}

// appendNote appends a note with the given body and fails the test on error.
func appendNote(t *testing.T, s *Store, body string) Entry {
	t.Helper()
	e, existed, err := s.Append(func(*Snapshot) (*Entry, *Entry, error) {
		return &Entry{Kind: "note", Item: "T1", From: "implementer", To: "reviewer", Body: body}, nil, nil
	})
	if err != nil {
		t.Fatalf("append %q: %v", body, err)
	}
	if existed {
		t.Fatalf("append %q: unexpectedly returned an existing entry", body)
	}
	return e
}

func load(t *testing.T, s *Store) *Snapshot {
	t.Helper()
	snap, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func readRecord(t *testing.T, s *Store) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.Dir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertIntact(t *testing.T, snap *Snapshot, wantEntries int) {
	t.Helper()
	if snap.Broken != nil {
		t.Fatalf("chain broken: %+v", *snap.Broken)
	}
	if snap.Torn != nil {
		t.Fatalf("unexpected torn tail: %q", snap.Torn)
	}
	if len(snap.Entries) != wantEntries {
		t.Fatalf("entries = %d, want %d", len(snap.Entries), wantEntries)
	}
}

func TestAppendBuildsSequentialChain(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 5; i++ {
		e := appendNote(t, s, fmt.Sprintf("body %d", i))
		if e.ID != int64(i) {
			t.Fatalf("id = %d, want %d", e.ID, i)
		}
	}
	snap := load(t, s)
	assertIntact(t, snap, 5)
	if snap.Entries[0].Prev != Genesis {
		t.Fatalf("first prev = %q, want %q", snap.Entries[0].Prev, Genesis)
	}
	lines := bytes.Split(bytes.TrimSuffix(readRecord(t, s), []byte("\n")), []byte("\n"))
	for i := 1; i < len(lines); i++ {
		if got, want := snap.Entries[i].Prev, HashLine(lines[i-1]); got != want {
			t.Fatalf("entry %d prev = %s, want %s", i+1, got, want)
		}
	}
	if adv := snap.Advisories(); len(adv) != 0 {
		t.Fatalf("advisories = %v, want none", adv)
	}
}

func TestBodyIsInsideTheLine(t *testing.T) {
	s := newStore(t)
	appendNote(t, s, "line one\nline two with \"quotes\" and <tags>")
	raw := readRecord(t, s)
	if bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("a body with newlines must stay on one record line; got %q", raw)
	}
	if !bytes.Contains(raw, []byte("<tags>")) {
		t.Fatalf("HTML characters should not be escaped in the record: %q", raw)
	}
	if got := load(t, s).Entries[0].Body; got != "line one\nline two with \"quotes\" and <tags>" {
		t.Fatalf("body round trip = %q", got)
	}
}

// Check 1, in process: concurrent writers through separate file descriptions.
// The process-level version (50 processes x 20 posts) is in cmd/handoff.
func TestConcurrentAppendsStaySequential(t *testing.T) {
	s := newStore(t)
	const writers, each = 20, 10
	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range each {
				_, _, err := New(s.Dir()).Append(func(*Snapshot) (*Entry, *Entry, error) {
					return &Entry{Kind: "note", Item: "T1", From: "implementer", To: "reviewer", Body: fmt.Sprintf("w%d-%d", w, i)}, nil, nil
				})
				if err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	snap := load(t, s)
	assertIntact(t, snap, writers*each)
	for i, e := range snap.Entries {
		if e.ID != int64(i+1) {
			t.Fatalf("entry %d has id %d", i, e.ID)
		}
	}
}

func TestBuildCanReturnExistingReceipt(t *testing.T) {
	s := newStore(t)
	first := appendNote(t, s, "once")
	got, existed, err := s.Append(func(snap *Snapshot) (*Entry, *Entry, error) {
		return nil, &snap.Entries[0], nil
	})
	if err != nil || !existed || got.ID != first.ID {
		t.Fatalf("got (%+v, %v, %v), want the existing entry #%d", got, existed, err, first.ID)
	}
	assertIntact(t, load(t, s), 1)
}

func TestBuildErrorWritesNothing(t *testing.T) {
	s := newStore(t)
	appendNote(t, s, "one")
	before := readRecord(t, s)
	refusal := errors.New("refused")
	if _, _, err := s.Append(func(*Snapshot) (*Entry, *Entry, error) { return nil, nil, refusal }); !errors.Is(err, refusal) {
		t.Fatalf("err = %v, want the build error", err)
	}
	if !bytes.Equal(before, readRecord(t, s)) {
		t.Fatal("a refused append changed the record")
	}
}

// Check 3: a torn tail refuses ordinary posts until repair, and repair keeps the bytes.
func TestTornTailRefusesUntilRepair(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 3; i++ {
		appendNote(t, s, fmt.Sprintf("good %d", i))
	}
	good := readRecord(t, s)
	torn := []byte(`{"v":1,"id":4,"ts":"2026-10-09T20:00:00Z","kind":"note","item":"T1","from":"impl`)
	f, err := os.OpenFile(filepath.Join(s.Dir(), FileName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(torn)
	f.Close()

	snap := load(t, s)
	if !bytes.Equal(snap.Torn, torn) {
		t.Fatalf("torn = %q, want %q", snap.Torn, torn)
	}
	if len(snap.Entries) != 3 {
		t.Fatalf("entries = %d, want the 3 good ones", len(snap.Entries))
	}
	_, _, err = s.Append(func(*Snapshot) (*Entry, *Entry, error) {
		return &Entry{Kind: "note", Item: "T1", From: "implementer", To: "reviewer", Body: "after"}, nil, nil
	})
	if !errors.Is(err, ErrTornTail) {
		t.Fatalf("append on a torn tail: err = %v, want ErrTornTail", err)
	}

	rep, err := s.Repair("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Kind != "repair" || rep.Damage == nil || rep.Damage.SHA256 != HashLine(torn) || rep.Damage.Bytes != len(torn) {
		t.Fatalf("repair entry = %+v", rep)
	}
	if rep.ID != 4 {
		t.Fatalf("repair id = %d, want 4", rep.ID)
	}
	raw := readRecord(t, s)
	if !bytes.HasPrefix(raw, good) {
		t.Fatal("repair changed a good line")
	}
	if !bytes.HasPrefix(raw[len(good):], append(append([]byte{}, torn...), '\n')) {
		t.Fatalf("repair did not preserve and close the torn bytes: %q", raw[len(good):])
	}
	after := appendNote(t, s, "after repair")
	if after.ID != 5 {
		t.Fatalf("next id = %d, want 5", after.ID)
	}
	snap = load(t, s)
	assertIntact(t, snap, 5) // 3 good, the repair, and the new note; the torn bytes are a covered line
	if adv := snap.Advisories(); len(adv) != 0 {
		t.Fatalf("advisories after repair = %v", adv)
	}
}

// Check 4: an edited middle line is detected by prev, and posting stops.
func TestEditedMiddleLineBreaksChain(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 5; i++ {
		appendNote(t, s, fmt.Sprintf("body %d", i))
	}
	path := filepath.Join(s.Dir(), FileName)
	raw := readRecord(t, s)
	edited := bytes.Replace(raw, []byte(`"body 2"`), []byte(`"body 2, edited"`), 1)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	snap := load(t, s)
	if snap.Broken == nil || snap.Broken.Line != 3 || snap.Broken.Withheld != 2 {
		t.Fatalf("broken = %+v, want the link at line 3 to fail and line 2 on to be withheld", snap.Broken)
	}
	// The chain cannot tell whether line 2 or line 3 changed, so only line 1 is verified.
	if len(snap.Entries) != 1 || len(snap.Unverified) != 4 {
		t.Fatalf("verified %d, unverified %d; want 1 and 4", len(snap.Entries), len(snap.Unverified))
	}
	if _, ok := snap.Find(2); ok {
		t.Fatal("the edited entry must not be found as verified")
	}
	if e, ok := snap.FindUnverified(2); !ok || e.Body != "body 2, edited" {
		t.Fatalf("forensic lookup of #2 = %+v, %v", e, ok)
	}
	if _, _, err := s.Append(func(*Snapshot) (*Entry, *Entry, error) {
		return &Entry{Kind: "note", Item: "T1", From: "implementer", To: "reviewer", Body: "x"}, nil, nil
	}); !errors.Is(err, ErrBroken) {
		t.Fatalf("append on a broken chain: err = %v, want ErrBroken", err)
	}
	if _, err := s.Repair("implementer"); !errors.Is(err, ErrBroken) {
		t.Fatalf("repair of a middle edit: err = %v, want ErrBroken", err)
	}
	if !bytes.Equal(edited, readRecord(t, s)) {
		t.Fatal("refused operations changed the record")
	}
}

// appendForged appends a well-formed line whose prev is wrong, by hand.
func appendForged(t *testing.T, s *Store) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(s.Dir(), FileName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(`{"v":1,"id":99,"ts":"2026-10-09T20:30:00Z","kind":"request","item":"W1","from":"implementer","to":"reviewer","commit":"x","body":"forged","prev":"broken"}` + "\n")
}

func partition(snap *Snapshot) (verified, withheld string) {
	var v, w []int64
	for _, e := range snap.Entries {
		v = append(v, e.ID)
	}
	for _, e := range snap.Unverified {
		w = append(w, e.ID)
	}
	return fmt.Sprint(v), fmt.Sprint(w)
}

// Gate finding 2 (Fable, case H; proposal §17 row 53): a forged append after
// good lines is withheld from the break on, because head.json names the line
// before the break as it is.
func TestForgedAppendIsWithheldFromTheBreakWhenHeadVouches(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 3; i++ {
		appendNote(t, s, fmt.Sprintf("body %d", i))
	}
	appendForged(t, s)
	snap := load(t, s)
	if snap.Broken == nil || snap.Broken.Line != 4 || snap.Broken.Withheld != 4 {
		t.Fatalf("broken = %+v, want the break at 4 and withholding from 4", snap.Broken)
	}
	if v, w := partition(snap); v != "[1 2 3]" || w != "[99]" {
		t.Fatalf("verified %s, withheld %s; want [1 2 3] and [99]", v, w)
	}
	w := snap.Warnings()
	if len(w) == 0 || !strings.Contains(w[0], "line 4 on are withheld") {
		t.Fatalf("warnings = %v", w)
	}
	for _, m := range w[1:] {
		if strings.Contains(m, "chain on") {
			t.Fatalf("the head advisory claims lines past a break chain on: %q", m)
		}
	}
}

// H5 CHANGES 1, where head.json does not vouch: the line before the break is
// withheld too, since the chain cannot tell which side changed.
func TestForgedAppendWithoutAVouchingHeadWithholdsTheLineBefore(t *testing.T) {
	s := newStore(t)
	appendNote(t, s, "body 1")
	appendNote(t, s, "body 2")
	headAfterTwo, err := os.ReadFile(filepath.Join(s.Dir(), HeadName))
	if err != nil {
		t.Fatal(err)
	}
	appendNote(t, s, "body 3")
	if err := os.WriteFile(filepath.Join(s.Dir(), HeadName), headAfterTwo, 0o644); err != nil {
		t.Fatal(err)
	}
	appendForged(t, s)
	snap := load(t, s)
	if snap.Broken == nil || snap.Broken.Line != 4 || snap.Broken.Withheld != 3 {
		t.Fatalf("broken = %+v, want the break at 4 and withholding from 3", snap.Broken)
	}
	if v, w := partition(snap); v != "[1 2]" || w != "[3 99]" {
		t.Fatalf("verified %s, withheld %s; want [1 2] and [3 99]", v, w)
	}
}

// Check 4 and row 51: a cut tail shows against head.json, as an advisory, not a refusal.
func TestCutTailIsAnAdvisoryNotARefusal(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 5; i++ {
		appendNote(t, s, fmt.Sprintf("body %d", i))
	}
	raw := readRecord(t, s)
	lines := bytes.SplitAfter(raw, []byte("\n"))
	cut := bytes.Join(lines[:3], nil)
	if err := os.WriteFile(filepath.Join(s.Dir(), FileName), cut, 0o644); err != nil {
		t.Fatal(err)
	}
	snap := load(t, s)
	assertIntact(t, snap, 3)
	adv := snap.Advisories()
	if len(adv) != 1 || !strings.Contains(adv[0], "head.json") {
		t.Fatalf("advisories = %v, want one about head.json", adv)
	}

	next := appendNote(t, s, "after the cut")
	if next.ID != 6 {
		t.Fatalf("id after a cut = %d, want 6: ids already handed out must not be reused", next.ID)
	}
	if adv := load(t, s).Advisories(); len(adv) != 1 {
		t.Fatalf("the advisory must stay until repair; got %v", adv)
	}
	rep, err := s.Repair("implementer")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Kind != "repair" || !strings.Contains(rep.Body, "head.json") {
		t.Fatalf("repair entry = %+v", rep)
	}
	snap = load(t, s)
	if adv := snap.Advisories(); len(adv) != 0 {
		t.Fatalf("advisories after repair = %v", adv)
	}
	assertIntact(t, snap, 5)
}

// Row 51: a crash after the record sync and before head.json leaves head behind.
func TestHeadBehindAfterCrashIsAnAdvisory(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 3; i++ {
		appendNote(t, s, fmt.Sprintf("body %d", i))
	}
	headBefore, err := os.ReadFile(filepath.Join(s.Dir(), HeadName))
	if err != nil {
		t.Fatal(err)
	}
	appendNote(t, s, "body 4")
	if err := os.WriteFile(filepath.Join(s.Dir(), HeadName), headBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	snap := load(t, s)
	assertIntact(t, snap, 4)
	adv := snap.Advisories()
	if len(adv) != 1 || !strings.Contains(adv[0], "behind") {
		t.Fatalf("advisories = %v, want one saying head.json is behind", adv)
	}
	if e := appendNote(t, s, "body 5"); e.ID != 5 {
		t.Fatalf("id = %d, want 5", e.ID)
	}
	if _, err := s.Repair("implementer"); err != nil {
		t.Fatal(err)
	}
	if adv := load(t, s).Advisories(); len(adv) != 0 {
		t.Fatalf("advisories after repair = %v", adv)
	}
}

// Gate finding 1 (Fable, case C; proposal §17 row 52): the last line has no
// successor, so head.json is its only witness. Same count, another hash: the
// last line is withheld and posting stops until the operator decides.
func TestEditedLastLineIsABreak(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 3; i++ {
		appendNote(t, s, fmt.Sprintf("body %d", i))
	}
	raw := readRecord(t, s)
	edited := bytes.Replace(raw, []byte(`"body 3"`), []byte(`"body 3, edited"`), 1)
	if err := os.WriteFile(filepath.Join(s.Dir(), FileName), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	snap := load(t, s)
	if b := snap.Broken; b == nil || b.Line != 3 || b.Withheld != 3 || !b.ByHead {
		t.Fatalf("broken = %+v, want a head-found break at the last line", b)
	}
	if v, w := partition(snap); v != "[1 2]" || w != "[3]" {
		t.Fatalf("verified %s, withheld %s; want [1 2] and [3]", v, w)
	}
	if adv := snap.Advisories(); len(adv) != 0 {
		t.Fatalf("the break is reported once, not again as an advisory: %v", adv)
	}
	if _, _, err := s.Append(func(*Snapshot) (*Entry, *Entry, error) {
		return &Entry{Kind: "note", Item: "T1", From: "implementer", To: "reviewer", Body: "x"}, nil, nil
	}); !errors.Is(err, ErrBroken) {
		t.Fatalf("append after an edited last line: err = %v, want ErrBroken", err)
	}
	if _, err := s.Repair("implementer"); !errors.Is(err, ErrBroken) {
		t.Fatalf("repair after an edited last line: err = %v, want ErrBroken", err)
	}
	if !bytes.Equal(edited, readRecord(t, s)) {
		t.Fatal("refused operations changed the record")
	}
}

// H6 CHANGES 1 (GPT-6; proposal §17 row 58): after a cut, a permitted
// append that stops before head.json restores the old count with a newer
// entry. That is a stale head.json, not an edit: an advisory, not a break.
func TestAppendAfterACutThatStopsBeforeHeadIsNotAnEdit(t *testing.T) {
	s := newStore(t)
	appendNote(t, s, "body 1")
	appendNote(t, s, "body 2")
	headPath := filepath.Join(s.Dir(), HeadName)
	staleHead, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(readRecord(t, s), []byte("\n"))
	if err := os.WriteFile(filepath.Join(s.Dir(), FileName), lines[0], 0o644); err != nil {
		t.Fatal(err)
	}
	if e := appendNote(t, s, "body 3"); e.ID != 3 {
		t.Fatalf("id after the cut = %d, want 3", e.ID)
	}
	if err := os.WriteFile(headPath, staleHead, 0o644); err != nil { // the append stopped before head.json
		t.Fatal(err)
	}
	snap := load(t, s)
	if snap.Broken != nil {
		t.Fatalf("a stale head.json after a cut was taken for an edit: %+v", *snap.Broken)
	}
	if v, w := partition(snap); v != "[1 3]" || w != "[]" {
		t.Fatalf("verified %s, withheld %s; want [1 3] and []", v, w)
	}
	adv := snap.Advisories()
	if len(adv) != 1 || !strings.Contains(adv[0], "#3") {
		t.Fatalf("advisories = %v, want one naming #3", adv)
	}
	if e := appendNote(t, s, "body 4"); e.ID != 4 {
		t.Fatalf("next id = %d, want 4", e.ID)
	}
	if _, err := s.Repair("implementer"); err != nil {
		t.Fatal(err)
	}
	if adv := load(t, s).Advisories(); len(adv) != 0 {
		t.Fatalf("advisories after repair = %v", adv)
	}
}

func TestRepairWithNothingToRepair(t *testing.T) {
	s := newStore(t)
	appendNote(t, s, "fine")
	if _, err := s.Repair("implementer"); !errors.Is(err, ErrNothingToRepair) {
		t.Fatalf("err = %v, want ErrNothingToRepair", err)
	}
	assertIntact(t, load(t, s), 1)
}

func TestCreateRefusesAnExistingRecord(t *testing.T) {
	s := newStore(t)
	if err := Create(s.Dir()); err == nil {
		t.Fatal("Create over an existing record succeeded")
	}
}
