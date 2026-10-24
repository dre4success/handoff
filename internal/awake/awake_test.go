package awake

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func needCaffeinate(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(caffeinate); err != nil {
		t.Skip("no caffeinate here")
	}
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func gone(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); alive(pid); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("caffeinate %d still runs", pid)
		}
	}
}

// One caffeinate, tied to this process, while on; none once off.
func TestHoldStartsOneCaffeinateAndReleaseEndsIt(t *testing.T) {
	needCaffeinate(t)
	k := &Keeper{}
	k.Set(true)
	if k.cmd == nil {
		t.Fatal("no caffeinate started")
	}
	pid := k.cmd.Process.Pid
	if want := []string{caffeinate, "-i", "-w", strconv.Itoa(os.Getpid())}; strings.Join(k.cmd.Args, " ") != strings.Join(want, " ") {
		t.Fatalf("args %q, want %q", k.cmd.Args, want)
	}
	k.Set(true)
	if k.cmd == nil || k.cmd.Process.Pid != pid || !alive(pid) {
		t.Fatal("a second Set(true) did not keep the one caffeinate")
	}
	k.Set(false)
	if k.cmd != nil {
		t.Fatal("still holding after Set(false)")
	}
	gone(t, pid)
	k.Set(false) // already off
	k.Set(true)
	if k.cmd == nil || k.cmd.Process.Pid == pid {
		t.Fatal("no new hold after a release")
	}
	k.Set(false)
}

// A caffeinate someone else ended is not started again by this process.
func TestAHoldEndedElsewhereIsLeftEnded(t *testing.T) {
	needCaffeinate(t)
	k := &Keeper{}
	k.Set(true)
	k.cmd.Process.Kill()
	<-k.done
	k.Set(true)
	k.Set(true)
	if k.cmd != nil || !k.off {
		t.Fatal("started caffeinate again after it was ended elsewhere")
	}
}

// A process killed while it holds takes its hold with it: caffeinate -w
// watches it.
func TestAKilledHolderLeavesNoHold(t *testing.T) {
	needCaffeinate(t)
	if os.Getenv("AWAKE_HELPER") != "" {
		return
	}
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperHolds$")
	helper.Env = append(os.Environ(), "AWAKE_HELPER=1")
	out, _ := helper.StdoutPipe()
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "caffeinate ")))
	if err != nil || !alive(pid) {
		t.Fatalf("helper said %q", line)
	}
	helper.Process.Signal(syscall.SIGKILL)
	helper.Wait()
	gone(t, pid)
}

func TestHelperHolds(t *testing.T) {
	if os.Getenv("AWAKE_HELPER") == "" {
		t.Skip("run by TestAKilledHolderLeavesNoHold")
	}
	k := &Keeper{}
	k.Set(true)
	fmt.Printf("caffeinate %d\n", k.cmd.Process.Pid)
	time.Sleep(time.Minute)
}

// In a test, holds go to the file HANDOFF_TEST_AWAKE names, one line per
// change, and no caffeinate runs.
func TestTestModeWritesChangesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awake.txt")
	t.Setenv(TestEnv, path)
	k := New()
	for _, on := range []bool{false, true, true, false, false, true, false} {
		k.Set(on)
		if k.cmd != nil {
			t.Fatal("caffeinate ran in a test")
		}
	}
	b, _ := os.ReadFile(path)
	p := os.Getpid()
	if want := fmt.Sprintf("hold %d\nrelease %d\nhold %d\nrelease %d\n", p, p, p, p); string(b) != want {
		t.Fatalf("file %q, want %q", b, want)
	}
}
