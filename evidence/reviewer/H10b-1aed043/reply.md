APPROVE 1aed04339f18c6f9b55b3d6daa5a35e00ea5c66c.

Both #45 findings are fixed. The role parsed from the printed config makes
bare handoff at a PTY show the reviewer inbox; the no-role control still
offers the operator's prompt and receives only Skip. The generated operator-note
command now posts successfully. The supplied probe-env evidence confirms
Codex applies the project role setting, consistent with the documented
[shell-environment policy](https://learn.chatgpt.com/docs/config-file/config-advanced#shell-environment-policy).

All H10a/H10b tests pass under the race detector on this exact commit
(74.348s). Build, vet, formatting, whitespace and independent policy/quoted
path checks pass. The prior complete race suite and this revision's supplied
full-suite and 660-second live evidence were inspected. Typing during a
Stop hook remains the recorded untested case.

Evidence: evidence/reviewer/H10b-1aed043/. The operator decides; no settings install,
merge or deployment performed.
