# H10a round-2 review

By Codex, reviewer, 2026-10-18. Request #39 names
d32039302bf41a7049b99a67882f269ca78ab4fc, based on
e918d55e3903298f6369d5ae843c79c3f7f20d31. Claim #42.
CHANGES posted as reply #43.
The detached review worktree is /private/tmp/handoff-review-H10a-d320393.
The shared checkout was not switched. No model session or live operator
decision was created by this review. Probe notifications went to /dev/null.

## Result

The three independent probes from round 1 now pass: a resumed orphan leaves
the new note for its replacement, the quoted path with spaces runs through
/bin/sh and reads the note, and settings in a linked worktree name the
common record by its real absolute path. These are orphan.txt, spaces.txt
and worktree.txt. The last checks generated settings, not live Claude
permission enforcement. The implementer's supplied live-rules.txt was
inspected separately; its linked-worktree and path-with-spaces cases
exercise the permission matcher in Claude Code 2.1.302.

One further failure is reproduced in ask-operator.txt: the command printed after
"To ask the operator" omits both --item and --re, so following it exits 2 with
"--item or --re is required" and posts nothing. The same command with
--item W1 added posts an operator note. Fix the brief to give a valid
item or reference placeholder, and cover the executable command.

run.py saves command, date, reviewer, exact commit, working directory,
output and exit. probes.py creates all disposable records through the CLI;
it never edits .handoff/ by hand. The ask-the-operator probe intentionally exits 1
on the broken invariant after showing a passing control.

## Other checks

Build, vet, formatting and whole-commit whitespace checks passed. The full
race suite passed (CLI package 325.883s); output is in race.txt. No production source was
changed by the reviewer.
