#!/usr/bin/env bash
# as-operator.sh <build> <project from probes.sh's Q1> <scratch>: the other ways to name the operator, with a
# terminal on stdin and one CLI's variable set. Is an Antigravity session treated as Claude Code's and Codex's are?
BIN=$1; P=$2; T=$3; HERE=$(cd "$(dirname "$0")" && pwd -P)
export HOME=$T/home HANDOFF_TEST_NOTIFY=$T/alerts.txt HANDOFF_TEST_AWAKE=$T/awake.txt
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_ROLE
echo "A note from a probe." > "$T/op.md"; cd "$P"
for v in "CLAUDE_CODE_SESSION_ID=s-x" "CODEX_THREAD_ID=t-x" "ANTIGRAVITY_CONVERSATION_ID=c-x" "NOTHING=1"; do
  echo "-- ${v%%=*}"
  for c in "note --as operator --to implementer --item W1 --title hello-$RANDOM --body $T/op.md" "reply --as operator 1 --verdict APPROVE --commit HEAD --body $T/op.md" "inbox --as operator" "brief --for implementer"; do
    printf '   %-22s %s\n' "$(echo $c | cut -d' ' -f1-3)" "$(env $v python3 -I "$HERE/withtty.py" "$BIN" $c 2>&1 | sed -e "s|$T|<scratch>|g" -e "s|/private/tmp/claude[^ ]*|handoff|g" | cut -c1-140)"
  done
done
