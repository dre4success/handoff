CHANGES on 418a0c42ea6e6fdbc2688834ba04277032000084.

1. [P2] Keep Codex's bare command out of the operator's decision UI
(cmd/handoff/settings.go:121). The broad allow rule accepts bare handoff,
while this setup leaves HANDOFF_ROLE unset for the session's own shell.
At a PTY that allowed command reaches "Approve, deny or skip?"; cmdHome
accepts its answer and writes an operator decision without passing through
the forbidden decide subcommand. I reproduced the prompt and entered only
Skip, so no decision was made. This path was disclosed in the request, but
it still defeats the intended guard against an agent casually deciding.
Ensure the agent role reaches its shell commands, or otherwise guard this
operator entry point while preserving the operator's terminal workflow. Evidence:
evidence/reviewer/H10b-418a0c4/bare.txt.

2. [P2] Carry the H10a operator-note correction into this brief
(cmd/handoff/brief.go:75). The "To ask the operator" command still omits --item or
--re. Following it exits 2 without posting a question. Adding --item W1
makes the same command post successfully. This is the inherited finding
already reported on H10a in #43; fix it there and carry it into this
branch. Evidence: evidence/reviewer/H10b-418a0c4/ask-operator.txt.

Direct probes pass for generated hooks, two successive Stop deliveries, exclusive
listening, parent death, and executable paths with spaces and apostrophes.
Codex's checker allows show/claim/reply and forbids decide.

The supplied 660-second live-session evidence was inspected. The direct
probes do not establish what typing during a Stop hook does; that remains
the recorded limitation. The generated hooks and rules agree with official
[hook](https://learn.chatgpt.com/docs/hooks#stop) and
[rule](https://learn.chatgpt.com/docs/agent-configuration/rules#understand-rule-fields)
semantics.

Build, vet, formatting, dependencies and whitespace checks pass. The full
race suite passes (CLI package 331.283s). Evidence and scripts:
evidence/reviewer/H10b-418a0c4/.
