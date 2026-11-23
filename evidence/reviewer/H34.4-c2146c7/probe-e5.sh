#!/bin/zsh
# Reviewer's probe E5 for H34.4 round 2 (c2146c7). Usage: probe-e5.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-330}; }
agy() { echo "\$ (Antigravity A, Stop hook, executionNum $1) handoff hook listen --timeout 1s"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":$1}" | ANTIGRAVITY_CONVERSATION_ID=A $H hook listen --timeout 1s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    d=json.loads(t); print('   ', d.get('decision','(no decision)'), '|', (d.get('reason') or '').replace('\n',' / ')[-250:])
except Exception: print('   ', t[:250])"; }

echo "## E5. fyi and away say when a session that can't take it now gets it"
new e5; sleep 300 & PC=$!; sleep 300 & PD=$!; sleep 300 & PE=$!
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
CLAUDE_CODE_SESSION_ID=R CLAUDE_PID=$PC $H role reviewer | head -1
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$PD $H role reviewer-2 | head -1
CLAUDE_CODE_SESSION_ID=R3 CLAUDE_PID=$PE $H role reviewer-3 | head -1
echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10}" | ANTIGRAVITY_CONVERSATION_ID=A $H hook listen --timeout 2s >/dev/null 2>&1
echo "-- implementer: Antigravity, at its cap. reviewer: never listened. reviewer-2: listening. reviewer-3: at a permission prompt."
( CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$PD $H wait --timeout 12s >/dev/null 2>&1 ) & WP=$!
print -r -- '{"session_id":"R3","hook_event_name":"Notification","notification_type":"permission_prompt","message":"allow Bash"}' | CLAUDE_CODE_SESSION_ID=R3 CLAUDE_PID=$PE $H hook notify >/dev/null 2>&1
sleep 1
echo "\$ (the operator, at a terminal) handoff fyi \"hold all merges\""; python3 $AT $H fyi "hold all merges" 2>&1 | grep -v '^$' | tidy | sed 's/^/    /'
wait $WP
( CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$PD $H wait --timeout 8s >/dev/null 2>&1 ) & WP=$!; sleep 1
echo "\$ (the operator, at a terminal) handoff away \"carry on\""; python3 $AT $H away "carry on" 2>&1 | grep -v '^$' | grep -v "^Under it\|told the operator" | tidy | sed 's/^/    /'
wait $WP; kill $PC $PD $PE 2>/dev/null
echo; echo "## E4. Usage"
$H nosuchcommand 2>&1 | grep -A3 "  settings" | tidy
