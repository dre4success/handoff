package notify

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/dre4success/handoff/internal/status"
)

func TestReviewFailedDeliveryCanBeRetried(t *testing.T) {
    dir := t.TempDir()
    destination := filepath.Join(t.TempDir(), "temporarily-missing", "sent.txt")
    t.Setenv(TestEnv, destination)
    var errors bytes.Buffer
    New(dir, "review", &errors).Send([]status.Alert{quietAlert})
    if !strings.Contains(errors.String(), "handoff: notify:") {
        t.Fatal("first attempt did not exercise the delivery failure")
    }
    t.Logf("first attempt stderr: %s", errors.String())
    if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { t.Fatal(err) }
    New(dir, "review", &errors).Send([]status.Alert{quietAlert})
    if got := sent(t, destination); len(got) != 1 {
        t.Fatalf("delivery was unavailable, then restored; got %d notifications, want one successful retry", len(got))
    }
}

func TestReviewMarkerFailureCanRecoverInLiveWait(t *testing.T) {
    destination := sink(t)
    dir := t.TempDir()
    markerDir := filepath.Join(dir, "notified")
    if err := os.WriteFile(markerDir, []byte("blocked"), 0o644); err != nil { t.Fatal(err) }
    var errors bytes.Buffer
    n := New(dir, "review", &errors)
    n.Send([]status.Alert{quietAlert})
    t.Logf("first attempt stderr: %s", errors.String())
    if err := os.Remove(markerDir); err != nil { t.Fatal(err) }
    n.Send([]status.Alert{quietAlert})
    if got := sent(t, destination); len(got) != 1 {
        t.Fatalf("marker storage restored during same wait; got %d notifications, want one", len(got))
    }
}
