CHANGES for request #61, commit 589c1cd5af490ba1f9b4d9ea8b296940d9bbb92c (base 4797d929c89c7cdc4c8335f0197d9fedd71ab5dc).

The two original findings are fixed in the ordinary sequence: independent probes now keep Codex thread bindings separate under a shared hook parent, including another thread's clear and --prefer; an already-running normal wait exits 2 after a role transfer and the replacement receives the note.

[P2] A role lost during listener startup must not disable the ownership check (cmd/handoff/session.go:162–169; cmd/handoff/hook.go:107–117; cmd/handoff/cmds.go:897–917).

Role resolution and `holding` read bindings separately. If another session takes the role between those reads, `holding` returns nil. `hookListen` then skips wrapping `alive` even though `given` is true; `cmdWait` likewise passes nil Alive to `wait.Run`. The displaced session therefore acquires the lock and listens until timeout, consuming entries and advancing the replacement's cursor. `sessionRole` can also retain its earlier `b.Role` after `update` finds that binding gone, which reaches this same path.

Independent Run on the exact commit reproduced both variants:

- `startup-race.txt`: an old hook started while the replacement's `role reviewer` ran; it was still listening 1.5 seconds after transfer completed, then exited 2 with the new note on stderr. The replacement's wait timed out.
- `startup-wait-race.txt`: the equivalent ordinary wait stayed active, consumed the new note and exited 0; the replacement timed out.

`startup-race.py` sets up 300 unrelated bindings through the CLI to widen the separate directory-read window, then overlaps the two real CLI processes. Both reproduced on the first attempt. No `.handoff/` file was edited by hand, and no model sessions were launched.

Carry the fact that a role was resolved from a session binding into listener startup. If that binding is lost, the ownership predicate must return false (or startup must refuse); it must not become the nil predicate used for an unbound explicit-role caller. Apply this to both hooks and ordinary waits, and cover transfer during startup as well as transfer after a listener registers.

Evidence: `evidence/reviewer/H12-589c1cd/`. Full `go test -race -count=1 ./...` PASS on this exact commit (CLI package 429.419s); build, vet, format, whitespace and dependencies PASS. The supplied live two-Claude and /clear probes were inspected; they were not repeated by the reviewer. No production source, installed settings or shared checkout branch was changed.
