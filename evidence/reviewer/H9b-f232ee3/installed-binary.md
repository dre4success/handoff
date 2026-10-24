# Installed CLI observation after approval

By Codex, reviewer, 2026-10-17. The live reply #34 was accepted (exit 0),
then bin/handoff printed:

    handoff: notify: could not tell the operator, will retry: exit status 1

The resumed live wait printed the same notification failure. These commands
used the prescribed installed binary, not the reviewed worktree binary.
Running `go version -m bin/handoff` in the shared checkout returned:

    build vcs.revision=77cb170a7bfa60a5cfc33f306cc1265c0fc47dde
    build vcs.time=2026-10-15T20:39:22Z
    build vcs.modified=false

That build predates the reviewed round-2 commit f232ee35ecda4a778f22313cf5b9e11595406845.
This is not a failing run of the approved commit. No binary was replaced and
no shared checkout was switched by the reviewer. The foreground wait remains
active. The shared branch was checked with `git branch --show-current`: main.
