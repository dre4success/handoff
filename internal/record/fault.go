package record

import (
	"os"
	"syscall"
	"time"
)

// FaultEnv names the variable that tests set to kill a writer at a chosen
// point, so that checks 3 and row 51 run against a real killed process
// rather than bytes a test wrote by hand. Values: "torn" (half a line written,
// or half of AppendAll's lines, then killed) and "after-sync" (a post's or a
// repair's line, or AppendAll's lines, written and synced, then killed before
// head.json). cmd/handoff reads "move-between" too: a role's move by /clear
// or /background killed between its two writes (row 107).
const FaultEnv = "HANDOFF_TEST_FAULT"

func fault(point string) bool { return os.Getenv(FaultEnv) == point }

func killSelf() {
	syscall.Kill(os.Getpid(), syscall.SIGKILL)
	time.Sleep(time.Minute) // SIGKILL cannot be caught; this only waits for it
}
