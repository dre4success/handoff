#!/bin/zsh
# Reviewer's probe for reviewer-3's #441: the two SessionStart hooks of a /clear, at once.
# Usage: probe-r6race.sh <a new, empty scratch dir> <handoff binary> <how many times>
D=$1; H=$2; N=${3:-40}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p "$D/p"; cd "$D/p"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null
sleep 600 & P=$!
CLAUDE_CODE_SESSION_ID=s0 CLAUDE_PID=$P $H role reviewer >/dev/null
bad=0
for i in $(seq 1 $N); do
  in="{\"session_id\":\"s$i\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}"
  ( echo "$in" | CLAUDE_CODE_SESSION_ID=s$i CLAUDE_PID=$P $H hook brief > "$D/a$i.out" 2>&1 ) & A=$!
  ( echo "$in" | CLAUDE_CODE_SESSION_ID=s$i CLAUDE_PID=$P $H hook brief > "$D/b$i.out" 2>&1 ) & B=$!
  wait $A $B
  a=$(head -1 "$D/a$i.out" | cut -c1-48); b=$(head -1 "$D/b$i.out" | cut -c1-48)
  if [[ "$a" != "handoff brief: you are reviewer"* || "$b" != "handoff brief: you are reviewer"* ]]; then bad=$((bad+1)); echo "  /clear $i: one hook said: ${a}… ; the other: ${b}…"; fi
done
echo "$N /clears, two SessionStart hooks at once each: $bad where a hook did not get the role"
kill $P 2>/dev/null
