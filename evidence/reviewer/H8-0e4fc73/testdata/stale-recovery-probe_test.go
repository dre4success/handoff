package notify

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/dre4success/handoff/internal/status"
)

type reviewWriter func([]byte) (int, error)
func (w reviewWriter) Write(p []byte) (int, error) { return w(p) }

// Both processes inspect the same orphaned attempt. A replaces it and is
// paused after sending, before marking done. B must not remove A's new claim.
func TestReviewConcurrentStaleRecoverySendsOnce(t *testing.T) {
    destination := sink(t)
    dir := t.TempDir()
    attempt := filepath.Join(dir, "notified", quietAlert.Key + ".sending")
    if err := os.MkdirAll(filepath.Dir(attempt), 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(attempt, nil, 0o644); err != nil { t.Fatal(err) }
    now := time.Now()
    old := now.Add(-2*time.Minute)
    if err := os.Chtimes(attempt, old, old); err != nil { t.Fatal(err) }

    bHasStat := make(chan struct{})
    aTold := make(chan struct{})
    releaseA := make(chan struct{})
    bFinished := make(chan struct{})
    a := New(dir, "A", reviewWriter(func(p []byte) (int, error) {
        if strings.Contains(string(p), "told the operator") {
            close(aTold)
            <-releaseA
        }
        return len(p), nil
    }))
    b := New(dir, "B", &bytes.Buffer{})
    aCalls, bCalls := 0, 0
    a.now = func() time.Time {
        aCalls++
        if aCalls == 2 { <-bHasStat }
        return now
    }
    b.now = func() time.Time {
        bCalls++
        if bCalls == 2 {
            close(bHasStat)
            <-aTold
        }
        return now
    }
    var wg sync.WaitGroup
    wg.Add(2)
    go func() { defer wg.Done(); a.Send([]status.Alert{quietAlert}) }()
    go func() { defer wg.Done(); b.Send([]status.Alert{quietAlert}); close(bFinished) }()
    select {
    case <-bFinished:
        close(releaseA)
    case <-time.After(5*time.Second):
        close(releaseA)
        t.Fatal("stale-recovery probe did not reach its expected interleaving")
    }
    wg.Wait()
    got := sent(t, destination)
    t.Logf("delivered: %q", got)
    if len(got) != 1 {
        t.Fatalf("two processes recovered the same stale attempt: sent %d times, want once", len(got))
    }
}
