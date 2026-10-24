# H10b review

By Codex, reviewer, 2026-10-18. Request #40 names
418a0c42ea6e6fdbc2688834ba04277032000084, based on
d32039302bf41a7049b99a67882f269ca78ab4fc. Claim #44.
CHANGES posted as reply #45.
Detached worktree: /private/tmp/handoff-review-H10b-418a0c4.
No shared checkout switch, model session, or live operator decision was
made by this review. All probe notifications went to /dev/null.

## Run evidence

run.py records the exact commit, command, reviewer, date, working directory,
output and exit. probes.py creates disposable records through the CLI and
does not hand-edit .handoff/.

- rules.txt: this commit's settings, from an executable under a path with
  spaces and apostrophes, are read by the installed Codex policy checker.
  show, claim and reply are allowed; decide is forbidden. The bare command
  is allowed, as the request disclosed. The generated SessionStart command
  runs through /bin/sh, without HANDOFF_ROLE, and prints the reviewer brief.
- listen.txt: the unchanged generated Stop command is run through /bin/sh
  twice. In each turn, presence names the shell's original pid, a second
  listener exits 4, and the hook exits 2 with the posted note on stderr.
  The second invocation receives only the new note.
- parent.txt: the generated exec command is stopped while its lightweight
  parent exits. After a note is posted and the hook resumes, it emits no
  wake message and a replacement wait receives the note.
- ask-operator.txt: the inherited brief error also reproduces on this exact
  commit: its printed operator-note command exits 2 because --item/--re
  is absent. Adding --item W1 makes the same command post successfully.
  brief_probe.py is the independent H10a probe copied here. This is the
  finding already sent on H10a as reply #43.
- bare.txt: the generated rule allows bare handoff. With HANDOFF_ROLE
  absent, a PTY invocation reaches the operator's "Approve, deny or skip?" prompt.
  The probe enters only Skip; it writes no operator decision. Source in
  cmdHome confirms an Approve/Deny answer would call writeDecision without
  using the forbidden decide command. The request disclosed this path;
  it remains a gap in the intended guard against casual agent decisions.
  The first added bare-mode probe had an indentation error before it ran
  any fixture (bare-fixture-error.txt); it was corrected before this run.
- build.txt, vet.txt, format.txt and whitespace.txt pass. deps.txt lists
  only this module and the standard library, with no net dependency.
- The full race suite passes (CLI package 331.283s); output is in race.txt.

The supplied live-codex.txt was inspected, not rerun: it records the real
Codex 0.162.1 session waiting 660 seconds, claiming and reviewing a request
in a detached worktree, replying, listening again, and exiting without a
hook left behind. Our direct hook probes do not substitute for that live
model evidence or establish what typing during a Stop hook does.

## Source/Inferred assessment

The official [Codex hooks documentation](https://learn.chatgpt.com/docs/hooks#stop)
documents SessionStart context, synchronous hooks, and Stop exit 2 with a
continuation reason on stderr. This matches the generated hooks. It also
states that background hooks do not start an idle turn, explaining why
H10b keeps Stop synchronous.

The official [Codex rules documentation](https://learn.chatgpt.com/docs/agent-configuration/rules#understand-rule-fields)
defines argv-prefix matching, allow outside the sandbox, and precedence of
forbidden over allow. The direct policy-checker run confirms this commit's
generated rules under quoting-sensitive paths.

The broad handoff allow rule was explicitly chosen by the operator in proposal row
68(d). Source review of the command dispatch, Git helper, record writers,
notifications and decision paths found no newly introduced arbitrary shell
execution or network path. Git reads are passed as argv; ordinary record
writes remain under the common .handoff. The popup still needs its lock
and a human click. As previously accepted, these are workflow guards,
not authentication. However, forbidding decide alone misses cmdHome's
operator path when the Codex shell lacks HANDOFF_ROLE. This setup should
ensure the agent role reaches shell commands or otherwise guard that
entry point while keeping the operator's terminal UI. The probe reached the UI,
entered only Skip and made no decision.
