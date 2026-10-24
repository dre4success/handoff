// Package awake keeps a Mac from idle sleep while a wait asks it to, so
// that the agents' work goes on while the operator is away (proposal §17 row 76). It
// runs macOS's caffeinate, tied to this process, and nothing else. Where
// there is no caffeinate it does nothing.
package awake

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// TestEnv names a file that receives a line each time a hold starts or
// ends, in place of caffeinate, so tests can see them.
const TestEnv = "HANDOFF_TEST_AWAKE"

const caffeinate = "/usr/bin/caffeinate"

// Keeper holds off idle sleep while it is on.
type Keeper struct {
	test string
	on   bool          // in a test: the last state written
	cmd  *exec.Cmd     // the running caffeinate, if any
	done chan struct{} // closed when cmd has ended
	off  bool          // caffeinate could not start, or ended on its own: no more tries
}

func New() *Keeper { return &Keeper{test: os.Getenv(TestEnv)} }

// Set starts or ends the hold. It is cheap to call on every tick.
func (k *Keeper) Set(on bool) {
	if k.test != "" {
		if on != k.on {
			k.on = on
			k.note(on)
		}
		return
	}
	switch {
	case on && k.cmd == nil && !k.off:
		k.start()
	case on && k.cmd != nil && k.ended():
		k.cmd, k.off = nil, true // someone ended it; leave sleep alone
	case !on && k.cmd != nil:
		k.cmd.Process.Kill()
		<-k.done
		k.cmd = nil
	}
}

// start runs caffeinate -i, which holds off idle sleep but not sleep when
// the lid closes, and -w, which ends it when this process ends, however
// it ends. Its stdin, stdout and stderr are /dev/null, so it never keeps a
// hook's pipes open, and Go opens every file close-on-exec, so it holds no
// lock of this process.
func (k *Keeper) start() {
	cmd := exec.Command(caffeinate, "-i", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		k.off = true
		return
	}
	k.cmd, k.done = cmd, make(chan struct{})
	go func(done chan struct{}) { cmd.Wait(); close(done) }(k.done)
}

func (k *Keeper) ended() bool {
	select {
	case <-k.done:
		return true
	default:
		return false
	}
}

func (k *Keeper) note(on bool) {
	word := "release"
	if on {
		word = "hold"
	}
	f, err := os.OpenFile(k.test, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "%s %d\n", word, os.Getpid())
	f.Close()
}
