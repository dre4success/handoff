CHANGES on ebc04f20dc7405567c73439b72a10cd2c247b3ab.

1. [P2] Check parent liveness before delivering or advancing the cursor
   (internal/wait/wait.go:140; hook.go's delivered branch).
   Alive is checked only after the record has been read and entries have
   advanced the cursor. A delivery returns before that check. I stopped a
   listening hook, ended its session parent, posted a note, and resumed the
   hook: the orphan consumed the note and printed a wake message for the
   dead session. A replacement wait then timed out with no output. Guard
   delivery with liveness checks and regress this parent-death/new-entry
   ordering. Evidence: orphan.txt.

2. [P2] Shell-quote executable paths in the brief and wake instructions
   (cmd/handoff/brief.go:68; selfCommand callers).
   When handoff is outside PATH and its full path contains spaces, the
   printed command is unquoted. Following it through /bin/sh exits 127,
   attempting to execute the prefix before the first space. Exec-form hook
   commands should remain raw paths; shell snippets need shell quoting and
   permission rules that match their command spelling. Evidence: spaces.txt.

3. [P2] Deny edits to the actual common record in linked worktrees
   (cmd/handoff/settings.go:69).
   Running settings claude from a linked worktree finds the main checkout's
   record, but prints Edit(/.handoff/**), anchored at the settings project.
   That protects a different directory and leaves the real record outside
   this deny rule. Build the protection from a.dir, using the absolute-path
   form, so all supported worktrees protect the same record. worktree.txt
   records the emitted settings and layout; the matching interpretation is
   from the official documentation, not a live model test:
   [Claude Code permission path rules](https://code.claude.com/docs/en/permissions#read-and-edit).

Build, vet, formatting, dependency and source/docs whitespace checks passed.
The full race suite passed (CLI package 316.540s). Evidence and reproduction script:
evidence/reviewer/H10a-ebc04f2/. No real model session or live operator
decision was created by this review.
