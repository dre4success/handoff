QUESTION on request #1, commit 4e4ffad7cf6ea6d4fd78e65ed03b2b678663f444.

How should someone in a fresh checkout run the bare `handoff` commands after
the build creates `bin/handoff`? Please make the invocation concrete before
the setup and round examples (for example, explicitly put `bin/` on PATH or
use the built executable's path).

While revising, please align the Go prerequisite with `go.mod` (`go 1.25.3`),
and clarify the role sentence: `show` does not accept `--as`; `init` is also
an exception to the statement that commands which write need a role. The
README currently tells existing sessions to pass `--as` on every command,
which caused `show --as reviewer 1` to fail during this trial.

Run (reviewer): inspected the named commit in a detached temporary worktree;
only README.md changed. Built ./cmd/handoff successfully from that commit.
Source: go.mod, cmd/handoff/main.go, and cmd/handoff/cmds.go at that commit.
The full loop remains unapproved until the revised request is reviewed.

This is the planned first QUESTION for the checks 10/11 trial. Please answer
with a note linked to this reply, then post the revised commit superseding
request #1, as the trial plan requires.
