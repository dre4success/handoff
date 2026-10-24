# H10b round-3 review

By Codex, reviewer, 2026-10-19 Europe/London (Run timestamps are UTC).
Request #50 names 1aed04339f18c6f9b55b3d6daa5a35e00ea5c66c, based on
abe51939226d18e48058a40cd9cf7a62846e141b. Claim #51.
APPROVE posted as reply #52.
Detached worktree: /private/tmp/handoff-review-H10b-1aed043.
No shared checkout was switched, no model session was launched, and no
live operator decision was made. Probe notifications went to /dev/null.

## Run evidence

run.py records command, date, reviewer, exact commit, cwd, output and exit.
The probes initialize all disposable records through the CLI and never
hand-edit .handoff/.

- bare.txt: Python's TOML parser reads the role from this commit's printed
  config.toml. With that actual value in the environment, bare handoff at
  a PTY shows the reviewer inbox, including the waiting operator note,
  and never prompts. Without the role, the control still reaches the operator's
  prompt and receives only Skip. No operator decision is written. Codex's
  policy checker confirms the bare command remains allowed, so the new
  role setting supplies the workflow guard.
- ask-operator.txt: the generated command, with its placeholders filled, posts
  an operator note successfully, including --as reviewer and --item W1.
- rules.txt: generated policy passes Codex's checker under a path with
  spaces and apostrophes. show/claim/reply are allowed and decide is
  forbidden. The quoted SessionStart command runs and supplies the brief.
- h10-race.txt: every H10a/H10b test passes under the race detector on this
  exact commit (74.348s), including the brief's executable commands and
  the role's PTY regression.
- Build, vet, formatting and branch whitespace checks pass in their logs.

Relative to the reviewed 418a0c4, production changes print the role config
and carry the approved H10a brief correction. Listener and policy logic
are unchanged. The complete race suite passed on 418a0c4 (331.283s for the
CLI package); this revision's affected H10 suite was run instead of
repeating it. The supplied full-suite evidence was inspected too.

## Source and supplied Run evidence

The official [shell-environment policy documentation](https://learn.chatgpt.com/docs/config-file/config-advanced#shell-environment-policy)
states that set supplies values to spawned commands. The emitted TOML
uses that documented setting. Project trust is required and is now
explicit in the printed setup text and README.

The implementer's probe-env/ logs were inspected: a Codex 0.162.1 session
started without HANDOFF_ROLE and with only the project config ran env
and saw HANDOFF_ROLE=codex. That is the supplied live evidence for Codex
applying the setting; our new probe tests the resulting CLI behavior.
The supplied 660-second live-codex.txt also shows claim, temporary-worktree
review, reply, resumed listening and cleanup. It does not establish the
bare-command path itself: the model ignored that part of its prompt, as
the request candidly records. No new model run is claimed here.

Both findings in #45 are closed. Typing to Codex while its Stop hook
waits remains untested, as recorded in proposal row 68(g). No further
blocking issue found in this revision. No settings were installed and no
merge or deployment was performed by the reviewer.
