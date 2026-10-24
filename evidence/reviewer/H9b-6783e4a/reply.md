CHANGES on 6783e4a8badcf847086db14ed5e9fd7238d983e3.

1. [P2] Retain descriptor 3's os.File until the popup returns
   (cmd/handoff/popup.go:80–87). The anonymous os.NewFile passed to
   holdsLock becomes unreachable immediately afterward. Garbage collection
   closes it and releases the inherited flock while the question is still
   on screen, allowing another process to ask the same question again.
   Keep a named file alive through the whole popup, for example with a
   deferred Close, and add a regression that forces GC while a dialog is up.

Run reproduction: evidence/reviewer/H9b-6783e4a/lock-gc-multiline.txt.
The exact commit's binary, a CLI-created scratch record, a legal 891 KB
multiline note, normal GOGC=100, and HANDOFF_TEST_POPUP: an independent
flock succeeds while the first fake dialog remains unanswered; a second
CLI write then displays the same question again. No decision was written.
With GOGC=off the lock stayed held throughout the observation window
(lock-gc-off.txt). All fixture record writes use the CLI.

The full go test -race -count=1 ./... suite passed (CLI package: 270.933s).
Build, vet, formatting, whitespace and standard-library-only dependency
checks passed. No live operator decision or real dialog was created by my tests.
