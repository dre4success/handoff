#!/bin/zsh
# Reviewer's probes for H34.4 (aa2ca90). Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; echo body > ../body.md; : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-330}; }
agy() { echo "\$ (Antigravity A, Stop hook, executionNum $1) handoff hook listen --timeout 1s"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":$1}" | ANTIGRAVITY_CONVERSATION_ID=A $H hook listen --timeout 1s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    d=json.loads(t); print('   ', d.get('decision','(no decision)'), '|', (d.get('reason') or '').replace('\n',' / ')[-250:])
except Exception: print('   ', t[:250])"; }

echo "## E1. Antigravity's wakes count down, on a wake for time running out and on one with work"
new e1
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
for n in 5 6 7 8 9 10; do agy $n; done
echo "-- with a note to deliver, at executionNum 8:"
$H note --as reviewer --to implementer --item P1 --title "one" --body ../body.md 2>&1 | grep "^posted"
agy 8
echo "-- a Claude Code session's wake has no count:"
sleep 300 & PC=$!; CLAUDE_CODE_SESSION_ID=R CLAUDE_PID=$PC $H role reviewer | head -1
echo '{"session_id":"R","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=R CLAUDE_PID=$PC $H hook listen --timeout 1s 2>&1 | tidy | sed 's/^/    /'

echo; echo "## E2. away warns of an Antigravity session, and not of the Claude Code one"
echo "\$ (the operator, at a terminal) handoff away \"carry on\""; python3 $AT $H away "carry on" 2>&1 | grep -v '^$' | tidy | sed 's/^/    /'
python3 -c "
import json,glob
for f in sorted(glob.glob('.handoff/sessions/*.json')):
    b=json.load(open(f)); print('  binding', b['session'], '| role', b.get('role'), '| cli', b.get('cli'))"
kill $PC 2>/dev/null

echo; echo "## E3. A role the project doesn't have"
new e3; sleep 300 & PC=$!
x() { echo "\$ (S) handoff $*"; CLAUDE_CODE_SESSION_ID=S CLAUDE_PID=$PC $H "$@" 2>&1 | head -2 | sed -E "s#$H#handoff#g" | tidy | sed 's/^/    /'; }
x role reviewer-3
x role Reviewer_3
x role operator
x roles add reviewer-3
x role reviewer-3
kill $PC 2>/dev/null

echo; echo "## E4. Usage"
$H nosuchcommand 2>&1 | grep -n "settings" | tidy
