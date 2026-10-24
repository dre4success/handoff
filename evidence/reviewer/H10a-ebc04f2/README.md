# H10a review

By Codex, reviewer, 2026-10-18. Request #36 names
ebc04f20dc7405567c73439b72a10cd2c247b3ab, based on
e918d55e3903298f6369d5ae843c79c3f7f20d31. Claim #37.
Review uses the detached worktree /private/tmp/handoff-review-H10a-ebc04f2.
CHANGES posted as reply #41. The CLI reported #36 was superseded by #39,
so this reply is retained as evidence and changes no active request state.
No shared checkout was switched. No real model or Claude Code session was
started by this review, and notifications went to /dev/null.

## Independent probes

probes.py uses the exact request commit's compiled CLI, with disposable Git
projects and CLI-created records. The probes do not edit .handoff/ by hand.
run.py records command, date, who, working directory, commit, output and exit.

- orphan.txt fails the expected delivery invariant. A lightweight Python
  process stands in for Claude Code and parents the actual hook. After the
  hook is listening, the probe stops it with SIGSTOP, ends its parent, posts
  a note, and resumes the hook. The orphan prints the new entry and advances
  the cursor. The replacement wait exits 3 with empty stdout. Alive is only
  checked after delivery; the delivered branch returns before that check.
- spaces.txt fails the expected command invariant. The exact binary is
  copied into a directory named "install with spaces". Following the
  unmodified command spelling printed by hook brief exits 127: the shell
  tries to execute the path prefix ending in "install".
- worktree.txt runs successfully and records a linked-worktree setup, its
  actual common record location, and the emitted deny rules. The following
  rule interpretation is Source/Inferred evidence, not a new live Claude
  permission-enforcement test: the printed Edit(/.handoff/**) is anchored at
  the settings project, so it protects the linked worktree's .handoff rather
  than the main checkout's actual record. See Claude Code's official
  [permission path semantics](https://code.claude.com/docs/en/permissions#read-and-edit).

## Other checks

build.txt, vet.txt, format.txt and deps.txt pass. Dependencies are this
module's packages and the standard library. The fresh complete race suite
passed (CLI package 316.540s); output is in race.txt. The full whitespace check only
flags the saved live transcript's trailing space and final blank line;
source/docs whitespace passes in whitespace-source.txt.

The request's supplied live evidence was inspected but not rerun. Official
[hook documentation](https://code.claude.com/docs/en/hooks#command-hook-fields)
supports exec-form args and asyncRewake exit 2. The review findings concern
the listener's cursor lifetime and the generated command/path spellings,
not the basic availability of that hook mechanism.
