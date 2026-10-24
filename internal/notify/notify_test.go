package notify

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dre4success/handoff/internal/status"
)

func sink(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sent.txt")
	t.Setenv(TestEnv, path)
	return path
}

func sent(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

var quietAlert = status.Alert{Kind: status.Quiet, Key: "quiet-1-2", Text: "H8: codex claimed #1 and has been quiet for 31 min"}

func TestEachAlertIsSentOnce(t *testing.T) {
	path := sink(t)
	dir := t.TempDir()
	var errb bytes.Buffer
	n := New(dir, "sela", &errb)
	n.Send([]status.Alert{quietAlert})
	n.Send([]status.Alert{quietAlert})
	New(dir, "sela", &errb).Send([]status.Alert{quietAlert}) // another process, later
	got := sent(t, path)
	if len(got) != 1 || got[0] != "sela: "+quietAlert.Text {
		t.Fatalf("sent %q; want the alert once, titled with the project", got)
	}
	if !strings.Contains(errb.String(), "told the operator: "+quietAlert.Text) {
		t.Fatalf("stderr %q; want the session told what the operator was told", errb.String())
	}
}

// Two waits, the implementer's and the reviewer's, see the same silence at
// the same moment. The operator hears it once.
func TestTwoProcessesRacingSendOnce(t *testing.T) {
	path := sink(t)
	dir := t.TempDir()
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			New(dir, "p", &bytes.Buffer{}).Send([]status.Alert{quietAlert})
		}()
	}
	wg.Wait()
	if got := sent(t, path); len(got) != 1 {
		t.Fatalf("sent %d times: %q", len(got), got)
	}
}

func TestANewKeyIsANewAlert(t *testing.T) {
	path := sink(t)
	n := New(t.TempDir(), "p", &bytes.Buffer{})
	n.Send([]status.Alert{quietAlert})
	later := quietAlert
	later.Key = "quiet-1-5"
	n.Send([]status.Alert{quietAlert, later})
	if got := sent(t, path); len(got) != 2 {
		t.Fatalf("sent %q; want two alerts", got)
	}
}

// The markers live under the record directory and nowhere else.
func TestMarkersStayInsideTheRecord(t *testing.T) {
	sink(t)
	dir := t.TempDir()
	New(dir, "p", &bytes.Buffer{}).Send([]status.Alert{quietAlert})
	if _, err := os.Stat(filepath.Join(dir, "notified", quietAlert.Key)); err != nil {
		t.Fatalf("marker: %v", err)
	}
}

// A key is used as a file name; one that could leave the directory is refused.
func TestUnsafeKeyIsNotSent(t *testing.T) {
	path := sink(t)
	var errb bytes.Buffer
	bad := quietAlert
	bad.Key = "../escape"
	New(t.TempDir(), "p", &errb).Send([]status.Alert{bad})
	if got := sent(t, path); got != nil {
		t.Fatalf("sent %q for an unsafe key", got)
	}
	if !strings.Contains(errb.String(), "unsafe") {
		t.Fatalf("stderr %q; want the refusal reported", errb.String())
	}
}

// H8 CHANGES 1: an alert is used up only by a send that worked.

// The reviewer's probe: delivery fails, is restored, and the next process
// sends the alert.
func TestFailedSendIsRetriedByTheNextProcess(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "missing", "sent.txt")
	t.Setenv(TestEnv, dest)
	var errb bytes.Buffer
	New(dir, "p", &errb).Send([]status.Alert{quietAlert})
	if strings.Contains(errb.String(), "told the operator") || !strings.Contains(errb.String(), "could not tell the operator") {
		t.Fatalf("stderr %q; want the failure reported and no claim that the operator was told", errb.String())
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	New(dir, "p", &errb).Send([]status.Alert{quietAlert})
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q after delivery came back; want one", got)
	}
}

// The reviewer's second probe: the markers directory is blocked, then
// cleared, during one wait. The first retry is at once.
func TestMarkerFailureRecoversInTheSameProcess(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	blocked := filepath.Join(dir, "notified")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := New(dir, "p", &bytes.Buffer{})
	n.Send([]status.Alert{quietAlert})
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q; want one after the block cleared", got)
	}
}

// A failure that lasts is retried less and less often: at once, then after
// 1, 2, 4 minutes, up to 15. A wait passes alerts on every second, so
// without this a broken osascript would run every second.
func TestLastingFailureBacksOff(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "missing", "sent.txt")
	t.Setenv(TestEnv, dest)
	var errb bytes.Buffer
	n := New(t.TempDir(), "p", &errb)
	now := time.Date(2026, 10, 14, 22, 0, 0, 0, time.UTC)
	n.now = func() time.Time { return now }
	tries := func() int { return strings.Count(errb.String(), "could not tell the operator") }
	step := func(d time.Duration, want int) {
		t.Helper()
		now = now.Add(d)
		n.Send([]status.Alert{quietAlert})
		if got := tries(); got != want {
			t.Fatalf("after %s more: %d attempts, want %d", d, got, want)
		}
	}
	step(0, 1)
	step(0, 2) // the first retry is at once
	step(30*time.Second, 2)
	step(31*time.Second, 3) // a minute after the second
	step(119*time.Second, 3)
	step(time.Second, 4) // two minutes after the third
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	step(4*time.Minute, 4)
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q once delivery came back; want one", got)
	}
}

// H8 round-2 CHANGES: whoever holds an alert's lock owns sending it. The
// lock is the kernel's (flock), so a holder that dies releases it, and one
// suspended by sleep keeps it; no guess from a file's age is needed.

func lockKey(t *testing.T, dir, key string) *os.File {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "notified"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "notified", key+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	return f
}

// Another process holds the lock, as a sender paused by sleep would: this
// one leaves the alert alone, quietly, and sends it once the lock is free.
func TestALockedAlertIsLeftToItsHolder(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	held := lockKey(t, dir, quietAlert.Key)
	var errb bytes.Buffer
	n := New(dir, "p", &errb)
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); got != nil || errb.Len() != 0 {
		t.Fatalf("sent %q, stderr %q, while another process held the lock; want nothing", got, errb.String())
	}
	held.Close()
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q after the lock was freed; want one", got)
	}
}

// The reviewer's interleaving: A has sent and is paused before marking the
// alert sent; B tries now. B must not send a second copy.
func TestSenderPausedBeforeMarkingIsNotDuplicated(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	told, release := make(chan struct{}), make(chan struct{})
	a := New(dir, "p", writerFunc(func(p []byte) (int, error) {
		if strings.Contains(string(p), "told the operator") {
			close(told)
			<-release
		}
		return len(p), nil
	}))
	done := make(chan struct{})
	go func() { a.Send([]status.Alert{quietAlert}); close(done) }()
	<-told
	b := New(dir, "p", &bytes.Buffer{})
	b.Send([]status.Alert{quietAlert})
	close(release)
	<-done
	b.Send([]status.Alert{quietAlert})
	New(dir, "p", &bytes.Buffer{}).Send([]status.Alert{quietAlert})
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q; want one", got)
	}
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// A sender killed while holding the lock leaves no lock behind: the kernel
// drops it with the process. The next process sends the alert.
func TestKilledSendersLockIsReleased(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	cmd.Env = append(os.Environ(), "NOTIFY_HELPER_DIR="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "held\n" {
		cmd.Process.Kill()
		t.Fatalf("helper said %q, %v", line, err)
	}
	n := New(dir, "p", &bytes.Buffer{})
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); got != nil {
		t.Fatalf("sent %q while the helper held the lock", got)
	}
	cmd.Process.Kill() // SIGKILL: no cleanup runs
	cmd.Wait()
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q after the holder was killed; want one", got)
	}
}

// TestHelperHoldLock is the process TestKilledSendersLockIsReleased kills.
func TestHelperHoldLock(t *testing.T) {
	dir := os.Getenv("NOTIFY_HELPER_DIR")
	if dir == "" {
		t.Skip("a helper process for TestKilledSendersLockIsReleased")
	}
	lockKey(t, dir, quietAlert.Key)
	fmt.Println("held")
	time.Sleep(time.Minute)
}

// H9b: a process in Codex's sandbox can show nothing, so it leaves every
// alert, unmarked, to a process that can. getenv stands in for Codex's
// environment; the sink stays set, so nothing reaches the screen either way.
func TestSandboxedProcessLeavesAlertsToOthers(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	var errb bytes.Buffer
	n := New(dir, "p", &errb)
	n.getenv = func(k string) string {
		if k == "CODEX_SANDBOX" {
			return "seatbelt"
		}
		return ""
	}
	n.Send([]status.Alert{quietAlert})
	if got := sent(t, dest); got != nil || errb.Len() != 0 {
		t.Fatalf("sent %q, stderr %q, from the sandbox; want nothing", got, errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "notified", quietAlert.Key)); !os.IsNotExist(err) {
		t.Fatalf("the alert was marked sent from the sandbox: %v", err)
	}
	New(dir, "p", &errb).Send([]status.Alert{quietAlert}) // a process outside it
	if got := sent(t, dest); len(got) != 1 {
		t.Fatalf("sent %q from outside the sandbox; want one", got)
	}
}

// A decide alert goes to Ask, with its lock held, and is not marked sent:
// Ask's process does that. While it holds the lock, no one else asks.
func TestDecideAlertIsHandedToAsk(t *testing.T) {
	dest := sink(t)
	dir := t.TempDir()
	decide := status.Alert{Kind: status.Decide, Key: "decide-1-2", Text: "H9: codex approved #1. Your decision.", Target: 1}
	var held *os.File
	var errb bytes.Buffer
	n := New(dir, "p", &errb)
	n.Ask = func(a status.Alert, lock *os.File) error {
		if a.Target != 1 {
			t.Errorf("asked about #%d, want #1", a.Target)
		}
		dup, err := syscall.Dup(int(lock.Fd())) // keep the lock past try's Close, as a child would
		if err != nil {
			t.Fatal(err)
		}
		held = os.NewFile(uintptr(dup), "held")
		return nil
	}
	n.Send([]status.Alert{decide})
	if held == nil || !strings.Contains(errb.String(), "asking the operator in a pop-up") {
		t.Fatalf("Ask not called; stderr %q", errb.String())
	}
	if got := sent(t, dest); got != nil {
		t.Fatalf("a decide alert handed to Ask was also notified: %q", got)
	}
	other := New(dir, "p", &bytes.Buffer{})
	asked := 0
	other.Ask = func(status.Alert, *os.File) error { asked++; return nil }
	other.Send([]status.Alert{decide})
	if asked != 0 {
		t.Fatal("a second process asked while the first held the lock")
	}
	if err := MarkSent(dir, decide.Key); err != nil {
		t.Fatal(err)
	}
	held.Close()
	other.Send([]status.Alert{decide})
	if asked != 0 {
		t.Fatal("asked again after the pop-up marked the alert sent")
	}
}
