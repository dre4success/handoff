#!/bin/zsh
# Reviewer's probes for H34.8 (2759963). Usage: probe.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`]*/handoff#handoff#g; s/since [0-9]{2}:[0-9]{2}/since HH:MM/g" | cut -c1-${W:-700}; }
A() { ANTIGRAVITY_CONVERSATION_ID=A "$@"; }
agy() { local idle=""; [[ -n "$2" ]] && idle=",\"fullyIdle\":$2"; echo "\$ (Antigravity A, Stop hook, executionNum $1${2:+, fullyIdle $2}) handoff hook listen --timeout ${3:-2}s"
  echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":$1$idle}" | A $H hook listen --timeout ${3:-2}s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    d=json.loads(t); print('   ', d.get('decision','(no wake)'), '|', (d.get('reason') or '').replace('\n',' / '))
except Exception: print('   ', t[:300])" | tidy; }
bind() { python3 -c "
import json
b=json.load(open('.handoff/sessions/A.json')); print('  binding A: block:', (b.get('block') or {}).get('reason','none'), '| listened:', 'yes' if b.get('listened') else 'no')"; }

echo "## X1. What an Antigravity session is told about listening"
new x1
echo "\$ (A) handoff role reviewer   (the Listening line)"; A $H role reviewer 2>&1 | grep "^Listening" | tidy
echo "\$ (A, SessionStart hook) handoff hook brief   (the Listening line)"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"SessionStart\",\"cwd\":\"$PWD\"}" | A $H hook brief 2>&1 | python3 -c "
import sys,json
d=json.loads(sys.stdin.read()); t=d['injectSteps'][0]['systemMessage']['systemMessage']
print([l for l in t.split('\n') if l.startswith('Listening')][0])" | tidy
echo "\$ handoff brief --for reviewer   (by hand, no session: the Listening line)"; $H brief --for reviewer 2>&1 | grep "^Listening" | tidy
echo "-- a Claude Code session with hooks, for comparison:"; sleep 300 & PC=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PC $H role implementer >/dev/null; echo '{"session_id":"I","hook_event_name":"SessionStart","source":"startup"}' | CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PC $H hook brief 2>&1 | grep "^Listening" | tidy; kill $PC 2>/dev/null

echo; echo "## X2. The background wait: its exits, and the Stop hook beside it"
echo "\$ (A) handoff wait --for reviewer --timeout 2s   (nothing new)"; A $H wait --for reviewer --timeout 2s 2>&1 | tidy; echo "  [exit ${pipestatus[1]}]"; bind
$H note --as implementer --to reviewer --item P1 --title "one" --body ../body.md 2>&1 | grep "^posted"
echo "\$ (A) handoff wait --for reviewer --timeout 5s   (a note waits)"; A $H wait --for reviewer --timeout 5s 2>&1 | tidy; echo "  [exit ${pipestatus[1]}]"
echo "-- a wait runs in the background; the turn ends, and the Stop hook runs with fullyIdle false, then (were it true) beside the wait's lock:"
( A $H wait --for reviewer --timeout 30s > "$D/x2-wait.out" 2>&1; echo "[exit $?]" >> "$D/x2-wait.out" ) & WP=$!; sleep 1
agy 0 false; agy 0 true; agy 10 true; bind
echo "\$ (A) a second wait while the first runs"; A $H wait --for reviewer --timeout 2s 2>&1 | tidy; echo "  [exit ${pipestatus[1]}]"
$H note --as implementer --to reviewer --item P1 --title "two" --body ../body2.md 2>&1 | grep "^posted\|^handoff"
wait $WP; echo "-- what the background wait printed:"; sed 's/^/    /' "$D/x2-wait.out" | tidy

echo; echo "## X3. The agent forgot to wait: the hook's wake, and its cap, as before"
agy 3 true 1
agy 10 true 1; bind; echo "  alerts to the operator: $(grep -c . "$HANDOFF_TEST_NOTIFY")"; tail -1 "$HANDOFF_TEST_NOTIFY" | tidy | sed 's/^/    /'
echo "-- the agent runs the wait again: the block is over"
A $H wait --for reviewer --timeout 1s >/dev/null 2>&1; bind
