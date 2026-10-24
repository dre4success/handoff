# H11 review

By Codex, reviewer, 2026-10-19. Request #54 names
a249704f01800aa4e0c28cab271093813cf31bbb, based on
ee29d8beead385456f0d653a4df51be6a772c2c4. Claim #55.
APPROVE posted as reply #56.
Detached worktree: /private/tmp/handoff-review-H11-a249704.
The shared checkout was not switched. No model session, live operator
decision, settings install, merge or deployment was performed.

## Independent Run evidence

run.py records command, reviewer, UTC date, exact commit, cwd, output and
exit. probes.py uses CLI-created disposable records; it never edits
.handoff/ by hand. All probe notifications go to /dev/null.

- ignore.txt: init without flags gives claude/codex both review permission
  and settings selects each CLI's own role. A byte-hash snapshot of all
  files outside .handoff, including Git metadata and an existing project
  .gitignore, is unchanged by init. After a note, git status is clean and
  git add -A stages no record files. Repeated init refuses without
  modifying the existing config.
- branches.txt: main beats master; master/trunk/unborn branches work;
  a detached main checkout with neither standard branch refuses before
  creating a record, and --main supplies an override. Init from a detached
  linked worktree takes the common main checkout's trunk branch and
  creates only the shared record.
- reviewers.txt: custom implementer/reviewer/reviewer-2 defaults to two
  reviewing roles; explicit --reviewers reviewer-2 wins. Custom roles
  still need an explicit settings role.
- post.txt: the brief's actual post command, with placeholders filled,
  runs through /bin/sh and posts the exact committed SHA to the other
  agent. The other agent's brief reverses the addressee correctly. The
  requesting role cannot reply to its own request; its addressee can.
- Build, vet, formatting, dependencies and whitespace checks pass.
  Dependencies remain this module and the standard library, with no net.
- The complete race suite passes (CLI package 389.359s); output is in race.txt.

## Assessment

The self-contained ignore file meets the record-only write boundary and
the plan's intent to keep the record untracked. The broad reviewer default
is documented, applies only to new records, preserves the traditional
implementer/reviewer pair, and has an explicit override. Both new default
roles have review permission; request addressee checks still govern who
can answer each request. Proposal row 72 appends the design choices.

No blocking issue found in the changed code or documentation.
